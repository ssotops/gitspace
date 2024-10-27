package plugin

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin-sdk/gsplug"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
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

// Handle graceful plugin shutdown
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Try graceful shutdown first
	if err := plugin.stdin.Close(); err != nil {
		m.logger.Warn("Failed to close plugin stdin", "error", err)
	}

	// Give the plugin a chance to clean up
	done := make(chan error, 1)
	go func() {
		done <- plugin.cmd.Wait()
	}()

	select {
	case <-time.After(3 * time.Second):
		// Force kill if graceful shutdown takes too long
		if err := plugin.cmd.Process.Kill(); err != nil {
			m.logger.Warn("Failed to kill plugin process", "error", err)
		}
	case err := <-done:
		if err != nil {
			m.logger.Warn("Plugin exited with error", "error", err)
		}
	}

	delete(m.plugins, name)
	delete(m.discoveredPlugins, name)
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

// plugin/manager.go

func (m *Manager) ExecuteCommand(pluginName, command string, params map[string]string) (string, error) {
	m.mu.RLock()
	plugin, ok := m.plugins[pluginName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("plugin not found: %s", pluginName)
	}

	// Get the menu to validate the command and its parameters
	menuResp, err := m.GetPluginMenu(pluginName)
	if err != nil {
		return "", fmt.Errorf("failed to get plugin menu: %w", err)
	}

	var menuOptions []gsplug.MenuOption
	err = json.Unmarshal(menuResp.MenuData, &menuOptions)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal menu data: %w", err)
	}

	// Recursive function to find command in menu hierarchy
	var findCommandInMenu func([]gsplug.MenuOption, string) *gsplug.MenuOption
	findCommandInMenu = func(options []gsplug.MenuOption, cmd string) *gsplug.MenuOption {
		for _, opt := range options {
			if opt.Command == cmd {
				return &opt
			}
			if len(opt.SubMenu) > 0 {
				if subOpt := findCommandInMenu(opt.SubMenu, cmd); subOpt != nil {
					return subOpt
				}
			}
		}
		return nil
	}

	selectedOption := findCommandInMenu(menuOptions, command)
	if selectedOption == nil {
		return "", fmt.Errorf("command not found in menu: %s", command)
	}

	// Validate that all required parameters are provided
	for _, param := range selectedOption.Parameters {
		if param.Required {
			value, exists := params[param.Name]
			if !exists || value == "" {
				return "", fmt.Errorf("missing required parameter: %s", param.Name)
			}
		}
	}

	// Execute the command with provided parameters
	req := &pb.CommandRequest{
		Command:    command,
		Parameters: params,
	}

	resp, err := plugin.sendRequest(2, req)
	if err != nil {
		return "", fmt.Errorf("error sending request to plugin: %w", err)
	}

	cmdResp, ok := resp.(*pb.CommandResponse)
	if !ok {
		return "", fmt.Errorf("unexpected response type: %T", resp)
	}

	if !cmdResp.Success {
		return "", fmt.Errorf("command failed: %s", cmdResp.ErrorMessage)
	}

	// Handle progress updates if available
	if cmdResp.Progress != nil {
		m.logger.Info("Command progress",
			"phase", cmdResp.Progress.Phase,
			"step", cmdResp.Progress.Step,
			"status", cmdResp.Progress.Status,
			"message", cmdResp.Progress.Message,
			"timestamp", cmdResp.Progress.Timestamp)
	}

	// Handle navigation context if available
	if cmdResp.Navigation != nil {
		m.logger.Debug("Command navigation context",
			"current_menu", cmdResp.Navigation.CurrentMenu,
			"parent_menu", cmdResp.Navigation.ParentMenu)
	}

	return cmdResp.Result, nil
}

func (m *Manager) promptForParameter(param gsplug.ParameterInfo) (string, error) {
	// Implement user prompting logic here
	// You can use a library like github.com/charmbracelet/huh for interactive prompts
	// For now, we'll use a simple fmt.Scanln
	var value string
	fmt.Printf("%s (%s): ", param.Name, param.Description)
	_, err := fmt.Scanln(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// plugin_manager.go

func (m *Manager) GetPluginMenu(pluginName string) (*pb.MenuResponse, error) {
	m.mu.RLock()
	plugin, exists := m.plugins[pluginName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginName)
	}

	m.logger.Debug("Sending menu request to plugin", "name", pluginName)
	req := &pb.MenuRequest{}

	resp, err := plugin.sendRequest(3, req)
	if err != nil {
		if strings.Contains(err.Error(), "broken pipe") {
			m.mu.Lock()
			delete(m.plugins, pluginName)
			m.mu.Unlock()
			return nil, fmt.Errorf("plugin %s has terminated unexpectedly", pluginName)
		}
		return nil, fmt.Errorf("error getting menu from plugin: %w", err)
	}

	menuResp, ok := resp.(*pb.MenuResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	return menuResp, nil
}

// plugin/types.go

func (p *Plugin) sendRequest(msgType uint32, msg proto.Message) (proto.Message, error) {
	p.Logger.Debug("Starting request send process",
		"type", msgType,
		"messageType", fmt.Sprintf("%T", msg))

	// Add mutex for thread safety
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	// Validate stdin
	if p.stdin == nil {
		p.Logger.Error("Stdin pipe is nil")
		return nil, fmt.Errorf("stdin pipe is closed")
	}

	// Marshal the request with logging
	p.Logger.Debug("Marshaling request message")
	data, err := proto.Marshal(msg)
	if err != nil {
		p.Logger.Error("Failed to marshal request",
			"error", err,
			"messageType", fmt.Sprintf("%T", msg))
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	p.Logger.Debug("Request marshaled successfully",
		"dataLength", len(data),
		"data", fmt.Sprintf("%x", data))

	// Create a buffer for the complete message
	buf := new(bytes.Buffer)
	p.Logger.Debug("Created message buffer")

	// Write message type
	p.Logger.Debug("Writing message type", "type", msgType)
	if err := buf.WriteByte(byte(msgType)); err != nil {
		p.Logger.Error("Failed to write message type",
			"error", err,
			"type", msgType)
		return nil, fmt.Errorf("failed to write message type to buffer: %w", err)
	}

	// Write message length
	p.Logger.Debug("Writing message length", "length", len(data))
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(data))); err != nil {
		p.Logger.Error("Failed to write message length",
			"error", err,
			"length", len(data))
		return nil, fmt.Errorf("failed to write message length to buffer: %w", err)
	}

	// Write message data
	p.Logger.Debug("Writing message data")
	if _, err := buf.Write(data); err != nil {
		p.Logger.Error("Failed to write message data",
			"error", err,
			"dataLength", len(data))
		return nil, fmt.Errorf("failed to write message data to buffer: %w", err)
	}

	// Write the entire buffer to stdin
	p.Logger.Debug("Writing buffer to stdin",
		"bufferSize", buf.Len())
	if _, err := io.Copy(p.stdin, buf); err != nil {
		p.Logger.Error("Failed to write to stdin",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Ensure the data is written
	p.Logger.Debug("Flushing stdin buffer")
	if err := p.stdin.(*bufferedWriteCloser).Flush(); err != nil {
		p.Logger.Error("Failed to flush stdin",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
	}

	// Read response with timeout and logging
	p.Logger.Debug("Setting up response reading")
	type readResult struct {
		respType uint32
		respData []byte
		err      error
	}

	resultChan := make(chan readResult, 1)
	go func() {
		p.Logger.Debug("Starting response read")
		respType, respData, err := readMessage(p.stdout)
		p.Logger.Debug("Read completed",
			"responseType", respType,
			"dataLength", len(respData),
			"error", err)
		resultChan <- readResult{respType, respData, err}
	}()

	p.Logger.Debug("Waiting for response", "timeout", "5s")
	select {
	case result := <-resultChan:
		if result.err != nil {
			p.Logger.Error("Failed to read response",
				"error", result.err,
				"errorType", fmt.Sprintf("%T", result.err))
			return nil, fmt.Errorf("failed to read response: %w", result.err)
		}

		p.Logger.Debug("Response received",
			"type", result.respType,
			"dataLength", len(result.respData))

		// Create appropriate response type
		var resp proto.Message
		switch msgType {
		case 1:
			resp = &pb.PluginInfo{}
		case 2:
			resp = &pb.CommandResponse{}
		case 3:
			resp = &pb.MenuResponse{}
		default:
			p.Logger.Error("Unknown request type",
				"type", msgType)
			return nil, fmt.Errorf("unknown request type: %d", msgType)
		}

		// Unmarshal response
		p.Logger.Debug("Unmarshaling response")
		if err := proto.Unmarshal(result.respData, resp); err != nil {
			p.Logger.Error("Failed to unmarshal response",
				"error", err,
				"responseType", fmt.Sprintf("%T", resp))
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		p.Logger.Debug("Response processed successfully",
			"responseType", fmt.Sprintf("%T", resp))
		return resp, nil

	case <-time.After(5 * time.Second):
		p.Logger.Error("Response timeout")
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (m *Manager) LoadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path, exists := m.discoveredPlugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not discovered", name)
	}

	m.logger.Info("Starting plugin load process",
		"name", name,
		"path", path,
		"exists", exists)

	// Create command with environment variables and verbose logging
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(),
		"GODEBUG=x509roots=1",
		fmt.Sprintf("PLUGIN_NAME=%s", name),
		"PLUGIN_DEBUG=1") // Enable verbose plugin logging

	m.logger.Debug("Creating plugin pipes")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.logger.Error("Failed to create stdin pipe",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	m.logger.Debug("stdin pipe created successfully")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.logger.Error("Failed to create stdout pipe",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	m.logger.Debug("stdout pipe created successfully")

	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.logger.Error("Failed to create stderr pipe",
			"error", err,
			"errorType", fmt.Sprintf("%T", err))
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	m.logger.Debug("stderr pipe created successfully")

	// Log process attributes
	m.logger.Debug("Setting process attributes",
		"setpgid", true)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Create buffered writer with detailed logging
	m.logger.Debug("Creating buffered stdin writer",
		"bufferSize", 1024*1024)
	bufferedStdin := NewBufferedWriteCloser(stdin, m.logger)

	// Create plugin logger with debug level
	pluginLogger, err := logger.NewRateLimitedLogger(name)
	if err != nil {
		m.logger.Error("Failed to create plugin logger",
			"error", err,
			"plugin", name)
		return fmt.Errorf("failed to create plugin logger: %w", err)
	}
	pluginLogger.SetLogLevel(log.DebugLevel)

	// Create and store plugin instance
	plugin := &Plugin{
		Name:   name,
		Path:   path,
		cmd:    cmd,
		stdin:  bufferedStdin,
		stdout: stdout,
		Logger: pluginLogger,
	}

	// Start the process with error capture
	m.logger.Debug("Starting plugin process")
	if err := cmd.Start(); err != nil {
		m.logger.Error("Failed to start plugin process",
			"error", err,
			"path", path,
			"errorType", fmt.Sprintf("%T", err))
		return fmt.Errorf("failed to start plugin process: %w", err)
	}
	m.logger.Debug("Plugin process started successfully",
		"pid", cmd.Process.Pid)

	// Start stderr logging with buffer monitoring
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			text := scanner.Text()
			m.logger.Debug("Plugin stderr",
				"name", name,
				"message", text,
				"messageLength", len(text))
		}
		if err := scanner.Err(); err != nil {
			m.logger.Error("Stderr scanner error",
				"error", err,
				"name", name)
		}
		m.logger.Debug("Stderr scanner completed",
			"name", name)
	}()

	// Brief initialization delay with logging
	m.logger.Debug("Waiting for plugin initialization")
	time.Sleep(100 * time.Millisecond)

	// Store plugin in map
	m.logger.Debug("Adding plugin to managed plugins map")
	m.plugins[name] = plugin

	// Initialize plugin with enhanced timeout handling and logging
	m.logger.Debug("Starting plugin initialization sequence")
	initDone := make(chan error, 1)
	go func() {
		// Request plugin info
		m.logger.Debug("Sending GetPluginInfo request")
		infoResp, err := plugin.sendRequest(1, &pb.PluginInfoRequest{})
		if err != nil {
			m.logger.Error("Failed to get plugin info",
				"error", err,
				"errorType", fmt.Sprintf("%T", err))
			initDone <- fmt.Errorf("failed to get plugin info: %w", err)
			return
		}
		m.logger.Debug("Received plugin info response",
			"responseType", fmt.Sprintf("%T", infoResp))

		// Validate response type
		if _, ok := infoResp.(*pb.PluginInfo); !ok {
			m.logger.Error("Unexpected plugin info response type",
				"received", fmt.Sprintf("%T", infoResp))
			initDone <- fmt.Errorf("unexpected response type for plugin info")
			return
		}
		m.logger.Debug("Plugin info validation successful")

		// Get initial menu
		m.logger.Debug("Sending GetMenu request")
		menuResp, err := plugin.sendRequest(3, &pb.MenuRequest{})
		if err != nil {
			m.logger.Error("Failed to get menu",
				"error", err,
				"errorType", fmt.Sprintf("%T", err))
			initDone <- fmt.Errorf("failed to get initial menu: %w", err)
			return
		}
		m.logger.Debug("Received menu response",
			"responseType", fmt.Sprintf("%T", menuResp))

		// Validate menu response
		if _, ok := menuResp.(*pb.MenuResponse); !ok {
			m.logger.Error("Unexpected menu response type",
				"received", fmt.Sprintf("%T", menuResp))
			initDone <- fmt.Errorf("unexpected response type for menu")
			return
		}

		m.logger.Debug("Plugin initialization sequence completed successfully")
		initDone <- nil
	}()

	// Wait for initialization with timeout
	m.logger.Debug("Waiting for initialization completion",
		"timeout", "5s")
	select {
	case err := <-initDone:
		if err != nil {
			m.logger.Error("Plugin initialization failed",
				"error", err,
				"name", name)
			delete(m.plugins, name)
			cmd.Process.Kill()
			return fmt.Errorf("plugin initialization failed: %w", err)
		}
		m.logger.Info("Plugin initialization completed successfully")
	case <-time.After(5 * time.Second):
		m.logger.Error("Plugin initialization timed out",
			"name", name)
		delete(m.plugins, name)
		cmd.Process.Kill()
		return fmt.Errorf("plugin initialization timed out")
	}

	return nil
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
	var msgTypeByte [1]byte
	n, err := r.Read(msgTypeByte[:])
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read message type: %w", err)
	}
	msgType := uint32(msgTypeByte[0])
	log.Debug("Read message type", "type", msgType, "bytesRead", n, "rawByte", fmt.Sprintf("%x", msgTypeByte))

	var msgLen uint32
	err = binary.Read(r, binary.LittleEndian, &msgLen)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read message length: %w", err)
	}
	log.Debug("Read message length", "length", msgLen)

	if msgLen > 10*1024*1024 { // 10 MB limit, adjust as needed
		return 0, nil, fmt.Errorf("message too large: %d bytes", msgLen)
	}

	data := make([]byte, msgLen)
	n, err = io.ReadFull(r, data)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read message data: %w", err)
	}
	log.Debug("Read message data", "bytesRead", n, "data", fmt.Sprintf("%x", data))

	return msgType, data, nil
}

func (m *Manager) AddDiscoveredPlugin(name, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.discoveredPlugins[name] = path
}

func (m *Manager) IsPluginLoaded(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.plugins[name]
	return exists
}

func (m *Manager) GetFilteredPlugins() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filtered := make(map[string]string)
	for name, path := range m.discoveredPlugins {
		// Filter out internal directories and non-plugin entries
		if name != "data" {
			filtered[name] = path
		}
	}

	return filtered
}

func (m *Manager) IsPluginRunning(pluginName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	plugin, exists := m.plugins[pluginName]
	if !exists {
		return false
	}
	return plugin.cmd.ProcessState == nil || !plugin.cmd.ProcessState.Exited()
}
