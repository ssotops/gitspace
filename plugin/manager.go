package plugin

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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

	m.logger.Debug("Starting plugin process", "name", name, "path", path)
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start plugin process: %w", err)
	}

	// Use buffered writer for stdin
	bufferedStdin := &bufferedWriteCloser{
		Writer: bufio.NewWriter(stdin),
		closer: stdin,
	}

	// Log stderr in a goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			m.logger.Debug("Plugin stderr", "name", name, "message", scanner.Text())
		}
	}()

	pluginLogger, err := logger.NewRateLimitedLogger(name)
	if err != nil {
		return fmt.Errorf("failed to create plugin logger: %w", err)
	}

	plugin := &Plugin{
		Name:   name,
		Path:   path,
		cmd:    cmd,
		stdin:  bufferedStdin,
		stdout: stdout,
		Logger: pluginLogger,
	}

	m.logger.Debug("Sending GetPluginInfo request", "name", name)
	infoResp, err := plugin.sendRequest(1, &pb.PluginInfoRequest{})
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}
	m.logger.Debug("Received GetPluginInfo response", "name", name, "response", fmt.Sprintf("%+v", infoResp))

	// Get menu
	m.logger.Debug("Getting plugin menu", "name", name)
	menuResp, err := plugin.sendRequest(3, &pb.MenuRequest{})
	if err != nil {
		return fmt.Errorf("failed to get plugin menu: %w", err)
	}
	menu, ok := menuResp.(*pb.MenuResponse)
	if !ok {
		return fmt.Errorf("unexpected response type for plugin menu")
	}
	m.logger.Debug("Plugin menu received", "name", name, "menuDataSize", len(menu.MenuData))

	// Store the plugin
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

func (p *ScmteaPlugin) ExecuteCommand(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	p.logger.Info("Executing command",
		"command", req.Command,
		"parameters", req.Parameters)

	switch req.Command {
	case "set_compose_file":
		return &pb.CommandResponse{
			Success: true,
			Result: `Please choose one of the following options:
1. Use Default Docker Compose File
2. Enter Custom Docker Compose Path
3. Go Back`,
			Navigation: &pb.NavigationContext{
				CurrentMenu: "set_compose_file",
				ParentMenu:  "installation_menu",
				AvailableCommands: []*pb.MenuItem{
					{
						Label:   "Use Default Docker Compose File",
						Command: "set_compose_file_default",
					},
					{
						Label:   "Enter Custom Docker Compose Path",
						Command: "set_compose_file_custom",
						Parameters: []*pb.ParameterInfo{
							{
								Name:        "custom_path",
								Description: "Path to custom Docker Compose file",
								Required:    true,
							},
						},
					},
					{
						Label:   "Go Back",
						Command: "go_back",
					},
				},
			},
		}, nil

	case "set_compose_file_default":
		return setComposeFile("Use default", "")

	case "set_compose_file_custom":
		customPath, ok := req.Parameters["custom_path"]
		if !ok || customPath == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Custom path is required for set_compose_file_custom command",
			}, nil
		}
		return setComposeFile("Enter custom path", customPath)

	case "setup":
		// Check for Docker Compose file before proceeding
		if _, err := getComposePath(); err != nil {
			return &pb.CommandResponse{
				Success: false,
				Result: `Before proceeding with setup, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu to continue.`,
				ErrorMessage: "Docker Compose file required. Please select 'Set Docker Compose File' from the Installation menu.",
				Navigation: &pb.NavigationContext{
					CurrentMenu: "installation_menu",
					ParentMenu:  "main",
					AvailableCommands: []*pb.MenuItem{
						{
							Label:     "Set Docker Compose File",
							Command:   "set_compose_file",
							SubmenuId: "compose_file_menu",
						},
						{
							Label:   "Go Back",
							Command: "go_back",
						},
					},
				},
			}, nil
		}

		// Validate parameters
		for _, param := range []string{"username", "password", "email"} {
			if _, ok := req.Parameters[param]; !ok {
				return &pb.CommandResponse{
					Success:      false,
					ErrorMessage: fmt.Sprintf("Missing required parameter: %s", param),
				}, nil
			}
		}

		return p.setupGitea(req)

	case "generate_ssh_key":
		// Validate parameters
		for _, param := range []string{"username", "password", "email"} {
			if _, ok := req.Parameters[param]; !ok {
				return &pb.CommandResponse{
					Success:      false,
					ErrorMessage: fmt.Sprintf("Missing required parameter: %s", param),
				}, nil
			}
		}
		return generateAndUploadSSHKey(req)

	case "start":
		return runDockerCompose("up", "-d")

	case "stop":
		return runDockerCompose("down")

	case "restart":
		return runDockerCompose("restart")

	case "print_summary":
		summary, err := printGiteaSummary(p.logger)
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to print Gitea summary: %v", err),
			}, nil
		}
		return &pb.CommandResponse{
			Success: true,
			Result:  summary,
		}, nil

	case "git_config_summary":
		return gitConfigSummary()

	case "delete_containers_images":
		return deleteContainersAndImages()

	case "delete_volumes":
		return deleteVolumes()

	case "go_back":
		return &pb.CommandResponse{
			Success: true,
			Result:  "Returned to previous menu",
			Navigation: &pb.NavigationContext{
				ParentMenu: "main",
			},
		}, nil

	// Backup Management Commands
	case "configure_backup":
		if err := validateBackupConfig(req.Parameters); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Invalid backup configuration: %v", err),
			}, nil
		}
		return p.handleBackupCommands(req)

	case "create_backup":
		// Check if Docker Compose file exists before proceeding
		if _, err := getComposePath(); err != nil {
			return &pb.CommandResponse{
				Success: false,
				Result: `Before creating a backup, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu.`,
				ErrorMessage: "Docker Compose file required for backup operations.",
				Navigation: &pb.NavigationContext{
					CurrentMenu: "installation_menu",
					ParentMenu:  "main",
					AvailableCommands: []*pb.MenuItem{
						{
							Label:     "Set Docker Compose File",
							Command:   "set_compose_file",
							SubmenuId: "compose_file_menu",
						},
					},
				},
			}, nil
		}
		return p.handleBackupCommands(req)

	case "set_backup_schedule":
		schedule, ok := req.Parameters["schedule"]
		if !ok || schedule == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Backup schedule (cron expression) is required",
			}, nil
		}

		// Check if Docker Compose file exists before proceeding
		if _, err := getComposePath(); err != nil {
			return &pb.CommandResponse{
				Success: false,
				Result: `Before setting a backup schedule, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu.`,
				ErrorMessage: "Docker Compose file required for backup operations.",
				Navigation: &pb.NavigationContext{
					CurrentMenu: "installation_menu",
					ParentMenu:  "main",
					AvailableCommands: []*pb.MenuItem{
						{
							Label:     "Set Docker Compose File",
							Command:   "set_compose_file",
							SubmenuId: "compose_file_menu",
						},
					},
				},
			}, nil
		}
		return p.handleBackupCommands(req)

	case "restore_backup":
		backupFile, ok := req.Parameters["backup_file"]
		if !ok || backupFile == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Backup file path is required",
			}, nil
		}

		// Check if Docker Compose file exists before proceeding
		if _, err := getComposePath(); err != nil {
			return &pb.CommandResponse{
				Success: false,
				Result: `Before restoring a backup, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu.`,
				ErrorMessage: "Docker Compose file required for backup operations.",
				Navigation: &pb.NavigationContext{
					CurrentMenu: "installation_menu",
					ParentMenu:  "main",
					AvailableCommands: []*pb.MenuItem{
						{
							Label:     "Set Docker Compose File",
							Command:   "set_compose_file",
							SubmenuId: "compose_file_menu",
						},
					},
				},
			}, nil
		}
		return p.handleBackupCommands(req)

	case "view_backup_summary":
		if _, err := getComposePath(); err != nil {
			return &pb.CommandResponse{
				Success: false,
				Result: `Before viewing backup summary, you need to configure a Docker Compose file.
Please select 'Set Docker Compose File' from the Installation menu.`,
				ErrorMessage: "Docker Compose file required for backup operations.",
				Navigation: &pb.NavigationContext{
					CurrentMenu: "installation_menu",
					ParentMenu:  "main",
					AvailableCommands: []*pb.MenuItem{
						{
							Label:     "Set Docker Compose File",
							Command:   "set_compose_file",
							SubmenuId: "compose_file_menu",
						},
					},
				},
			}, nil
		}
		return p.handleBackupCommands(req)

	default:
		p.logger.Error("Unknown command received", "command", req.Command)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Unknown command: %s", req.Command),
		}, nil
	}
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
		if strings.Contains(err.Error(), "broken pipe") {
			m.mu.Lock()
			delete(m.plugins, pluginName)
			m.mu.Unlock()
			return nil, fmt.Errorf("plugin %s has terminated unexpectedly", pluginName)
		}
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
	p.Logger.Debug("Preparing to send request", "type", msgType, "name", p.Name)

	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	p.Logger.Debug("Marshaled request", "data", fmt.Sprintf("%x", data))

	p.Logger.Debug("Writing message type", "type", msgType)
	if _, err := p.stdin.Write([]byte{byte(msgType)}); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	p.Logger.Debug("Writing message length", "length", len(data))
	if err := binary.Write(p.stdin, binary.LittleEndian, uint32(len(data))); err != nil {
		return nil, fmt.Errorf("failed to write message length: %w", err)
	}

	p.Logger.Debug("Writing message data", "data", fmt.Sprintf("%x", data))
	if _, err := p.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write message data: %w", err)
	}

	if err := p.stdin.(*bufferedWriteCloser).Flush(); err != nil {
		p.Logger.Warn("Failed to flush stdin", "error", err)
	}

	p.Logger.Debug("Waiting for response", "name", p.Name)
	respType, respData, err := readMessage(p.stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	p.Logger.Debug("Received response", "type", respType, "dataLength", len(respData), "rawData", fmt.Sprintf("%x", respData))

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

	p.Logger.Debug("Unmarshalled response", "content", fmt.Sprintf("%+v", resp))
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

func (bwc *bufferedWriteCloser) Close() error {
	if err := bwc.Flush(); err != nil {
		return err
	}
	return bwc.closer.Close()
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
