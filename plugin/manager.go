package plugin

import (
	"fmt"
	"io"
	"os/exec"
	"sync"

	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"google.golang.org/protobuf/proto"
)

type Manager struct {
	plugins          map[string]*Plugin
	installedPlugins map[string]string // map of plugin name to path
	mu               sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		plugins:          make(map[string]*Plugin),
		installedPlugins: make(map[string]string),
	}
}

func (m *Manager) LoadPlugin(name, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	plugin := &Plugin{
		Name:   name,
		Path:   path,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}

	// Initialize the plugin
	initReq := &pb.PluginInfoRequest{}
	resp, err := plugin.sendRequest(1, initReq)
	if err != nil {
		return err
	}

	pluginInfo := resp.(*pb.PluginInfo)
	if pluginInfo.Name == "" {
		return fmt.Errorf("failed to initialize plugin: empty name returned")
	}

	m.plugins[name] = plugin
	return nil
}

func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if err := plugin.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill plugin process: %w", err)
	}

	delete(m.plugins, name)
	delete(m.installedPlugins, name)
	return nil
}

func (m *Manager) GetLoadedPlugins() map[string]*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	loadedPlugins := make(map[string]*Plugin)
	for name, plugin := range m.plugins {
		loadedPlugins[name] = plugin
	}

	return loadedPlugins
}

func (m *Manager) ExecuteCommand(pluginName, command string, params map[string]string) (string, error) {
	m.mu.RLock()
	plugin, ok := m.plugins[pluginName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("plugin not found: %s", pluginName)
	}

	req := &pb.CommandRequest{
		Command:    command,
		Parameters: params,
	}

	resp, err := plugin.sendRequest(2, req)
	if err != nil {
		return "", err
	}

	cmdResp := resp.(*pb.CommandResponse)
	if !cmdResp.Success {
		return "", fmt.Errorf("command failed: %s", cmdResp.ErrorMessage)
	}

	return cmdResp.Result, nil
}

func (m *Manager) GetPluginMenu(pluginName string) ([]*pb.MenuItem, error) {
	m.mu.RLock()
	plugin, ok := m.plugins[pluginName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginName)
	}

	req := &pb.MenuRequest{}

	resp, err := plugin.sendRequest(3, req)
	if err != nil {
		return nil, err
	}

	menuResp := resp.(*pb.MenuResponse)
	return menuResp.Items, nil
}

func (p *Plugin) sendRequest(msgType uint32, msg proto.Message) (proto.Message, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	_, err = p.stdin.Write([]byte{byte(msgType)})
	if err != nil {
		return nil, err
	}

	_, err = p.stdin.Write(data)
	if err != nil {
		return nil, err
	}

	respData, err := io.ReadAll(p.stdout)
	if err != nil {
		return nil, err
	}

	var resp proto.Message
	switch msgType {
	case 1:
		resp = &pb.PluginInfo{}
	case 2:
		resp = &pb.CommandResponse{}
	case 3:
		resp = &pb.MenuResponse{}
	default:
		return nil, fmt.Errorf("unknown message type: %d", msgType)
	}

	err = proto.Unmarshal(respData, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (m *Manager) GetInstalledPlugins() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy of the installedPlugins map to avoid concurrent access issues
	installedPlugins := make(map[string]string)
	for name, path := range m.installedPlugins {
		installedPlugins[name] = path
	}

	return installedPlugins
}
