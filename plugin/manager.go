package plugin

import (
	"bufio"
	"context"
	"time"

	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
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

	log.Printf("Attempting to load plugin: %s from path: %s", name, path)

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
			log.Printf("Plugin %s stderr: %s", name, scanner.Text())
		}
	}()

	plugin := &Plugin{
		Name:   name,
		Path:   path,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}

	// Retrieve plugin info
	log.Printf("Retrieving plugin info for: %s", name)
	infoReq := &pb.PluginInfoRequest{}
	resp, err := plugin.sendRequest(1, infoReq)
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	pluginInfo := resp.(*pb.PluginInfo)
	plugin.Version = pluginInfo.Version

	log.Printf("Plugin %s loaded successfully with version: %s", name, plugin.Version)

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

	menuResp := resp.(*pb.MenuResponse)
	log.Printf("Received menu response from plugin %s with %d items", pluginName, len(menuResp.Items))
	return menuResp.Items, nil
}

func (p *Plugin) sendRequest(msgType uint32, msg proto.Message) (proto.Message, error) {
	log.Printf("Sending request type %d to plugin %s", msgType, p.Name)
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

	log.Printf("Waiting for response from plugin %s", p.Name)
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

	log.Printf("Received response from plugin %s", p.Name)
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

func (m *Manager) LoadAllPlugins(logger *log.Logger) error {
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

			logger.Debug("Attempting to load plugin", "name", pluginName)

			// Create a context with a timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Use a channel to communicate the result of loading the plugin
			done := make(chan error)

			go func() {
				err := m.LoadPlugin(pluginName, pluginPath)
				done <- err
			}()

			// Wait for the plugin to load or for the timeout to occur
			select {
			case err := <-done:
				if err != nil {
					logger.Warn("Failed to load plugin", "name", pluginName, "error", err)
				} else {
					logger.Info("Plugin loaded successfully", "name", pluginName)
				}
			case <-ctx.Done():
				logger.Warn("Plugin loading timed out", "name", pluginName)
			}
		}
	}

	return nil
}

// EnsurePluginDirectoryPermissions ensures that the plugins directory has the correct permissions and ownership
// Without this, we'll see logs like this (which effectively means the plugin is not loaded):
// WARN <plugin/manager.go:222> Failed to load plugin name=hello-world error="failed to start plugin process: fork/exec /Users/alechp/.ssot/gitspace/plugins/hello-world/hello-world: permission denied"
func EnsurePluginDirectoryPermissions(logger *log.Logger) error {
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
