package plugin

import (
	"bufio"
	"encoding/binary"

	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"github.com/ssotops/gitspace/logger"
	"google.golang.org/protobuf/proto"
)

type Manager struct {
	plugins           map[string]*Plugin
	discoveredPlugins map[string]string // map of plugin name to path
	mu                sync.RWMutex
	logger            *logger.RateLimitedLogger
}

func NewManager(l *logger.RateLimitedLogger) *Manager {
	manager := &Manager{
		plugins:           make(map[string]*Plugin),
		discoveredPlugins: make(map[string]string),
		logger:            l,
	}

	err := EnsurePluginDirectoryPermissions(l)
	if err != nil {
		l.Error("Failed to ensure plugin directory permissions during manager initialization", "error", err)
	}

	return manager
}

func (m *Manager) LoadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, exists := m.discoveredPlugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not discovered", name)
	}

	m.logger.Info("Attempting to load plugin", "name", name, "path", path)

	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start plugin process: %w", err)
	}

	// Read stderr in a goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			m.logger.Info("Plugin stderr", "name", name, "message", scanner.Text())
		}
	}()

	// Read stdout in a goroutine
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			m.logger.Info("Plugin stdout", "name", name, "message", scanner.Text())
		}
	}()

	plugin := &Plugin{
		Name:   name,
		Path:   path,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		logger: m.logger,
	}

	// Send an initial message to the plugin
	m.logger.Debug("Sending initial message to plugin", "name", name)
	resp, err := plugin.sendRequest(1, &pb.PluginInfoRequest{})
	if err != nil {
		m.logger.Error("Failed to send initial message to plugin", "name", name, "error", err)
		cmd.Process.Kill()
		return fmt.Errorf("failed to send initial message to plugin: %w", err)
	}

	// Check the response
	pluginInfo, ok := resp.(*pb.PluginInfo)
	if !ok {
		m.logger.Error("Unexpected response type from plugin", "name", name, "type", fmt.Sprintf("%T", resp))
		cmd.Process.Kill()
		return fmt.Errorf("unexpected response type from plugin")
	}

	m.logger.Info("Received plugin info", "name", name, "version", pluginInfo.Version)

	m.plugins[name] = plugin

	m.logger.Info("Plugin loaded successfully", "name", name)
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
	delete(m.discoveredPlugins, name) // Changed from m.installedPlugins to m.discoveredPlugins
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

func (m *Manager) GetPluginMenu(pluginName string) (*pb.MenuResponse, error) {
	m.mu.RLock()
	plugin, exists := m.plugins[pluginName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginName)
	}

	log.Printf("Sending GetMenu request to plugin: %s", pluginName)
	req := &pb.MenuRequest{}

	resp, err := plugin.sendRequest(3, req)
	if err != nil {
		log.Printf("Error getting menu from plugin %s: %v", pluginName, err)
		return nil, err
	}

	menuResp, ok := resp.(*pb.MenuResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	log.Printf("Received menu response from plugin %s", pluginName)
	return menuResp, nil
}

func (p *Plugin) sendRequest(msgType uint32, msg proto.Message) (proto.Message, error) {
	p.logger.Debug("Sending request to plugin", "type", msgType, "name", p.Name)
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := p.stdin.Write([]byte{byte(msgType)}); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	if err := binary.Write(p.stdin, binary.LittleEndian, uint32(len(data))); err != nil {
		return nil, fmt.Errorf("failed to write message length: %w", err)
	}

	if _, err := p.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write message data: %w", err)
	}

	// Flush stdin to ensure the message is sent immediately
	if f, ok := p.stdin.(*os.File); ok {
		f.Sync()
	}

	p.logger.Debug("Waiting for response from plugin", "name", p.Name)
	respType, respData, err := readMessage(p.stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp proto.Message
	switch respType {
	case 1:
		resp = &pb.PluginInfo{}
	case 2:
		resp = &pb.CommandResponse{}
	case 3:
		resp = &pb.MenuResponse{}
	default:
		return nil, fmt.Errorf("unknown response type: %d", respType)
	}

	err = proto.Unmarshal(respData, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	p.logger.Debug("Received response from plugin", "name", p.Name)
	return resp, nil
}

func (m *Manager) GetDiscoveredPlugins() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy of the discoveredPlugins map to avoid concurrent access issues
	discoveredPlugins := make(map[string]string)
	for name, path := range m.discoveredPlugins {
		discoveredPlugins[name] = path
	}

	return discoveredPlugins
}

func (m *Manager) LoadAllPlugins() error {
	err := m.DiscoverPlugins()
	if err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	for name := range m.discoveredPlugins {
		err := m.LoadPlugin(name)
		if err != nil {
			m.logger.Warn("Failed to load plugin", "name", name, "error", err)
		}
	}

	return nil
}

// EnsurePluginDirectoryPermissions ensures that the plugins directory has the correct permissions and ownership
// Without this, we'll see logs like this (which effectively means the plugin is not loaded):
// WARN <plugin/manager.go:222> Failed to load plugin name=hello-world error="failed to start plugin process: fork/exec /Users/alechp/.ssot/gitspace/plugins/hello-world/hello-world: permission denied"
func EnsurePluginDirectoryPermissions(logger *logger.RateLimitedLogger) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	// Ensure the plugins directory exists
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Walk through the plugins directory and set permissions
	err = filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Set directory permissions to 755 (rwxr-xr-x)
		if info.IsDir() {
			if err := os.Chmod(path, 0755); err != nil {
				return fmt.Errorf("failed to set directory permissions for %s: %w", path, err)
			}
		} else {
			// Set file permissions to 755 (rwxr-xr-x) to ensure executability
			if err := os.Chmod(path, 0755); err != nil {
				return fmt.Errorf("failed to set file permissions for %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to set permissions for plugins directory: %w", err)
	}

	logger.Info("Plugin directory permissions set successfully", "path", pluginsDir)
	return nil
}

func (m *Manager) DiscoverPlugins() error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			pluginName := entry.Name()
			pluginPath := filepath.Join(pluginsDir, pluginName, pluginName)
			m.discoveredPlugins[pluginName] = pluginPath
			m.logger.Debug("Discovered plugin", "name", pluginName, "path", pluginPath)
		}
	}

	m.logger.Debug("Total discovered plugins", "count", len(m.discoveredPlugins))

	return nil
}

func readMessage(r io.Reader) (uint32, []byte, error) {
	var msgType [1]byte
	_, err := r.Read(msgType[:])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read message type: %w", err)
	}

	var msgLen uint32
	err = binary.Read(r, binary.LittleEndian, &msgLen)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read message length: %w", err)
	}

	data := make([]byte, msgLen)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read message data: %w", err)
	}

	return uint32(msgType[0]), data, nil
}

func (m *Manager) AddDiscoveredPlugin(name, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.discoveredPlugins[name] = path
}
