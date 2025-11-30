package launchconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// LaunchJSONFileName is the standard name for VS Code launch configuration file.
	LaunchJSONFileName = "launch.json"
	// VSCodeDirName is the VS Code configuration directory name.
	VSCodeDirName = ".vscode"
)

// LoadFromPath loads a launch.json file from an explicit path.
func LoadFromPath(path string) (*LaunchJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read launch.json: %w", err)
	}

	var lj LaunchJSON
	if err := json.Unmarshal(data, &lj); err != nil {
		return nil, fmt.Errorf("failed to parse launch.json: %w", err)
	}

	return &lj, nil
}

// Discover searches for a .vscode/launch.json file starting from the given path
// and walking up the directory tree until found or reaching the root.
func Discover(startPath string) (string, error) {
	if startPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		startPath = cwd
	}

	// Get absolute path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// If startPath is a file, start from its directory
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	// Walk up the directory tree
	current := absPath
	for {
		launchPath := filepath.Join(current, VSCodeDirName, LaunchJSONFileName)
		if _, err := os.Stat(launchPath); err == nil {
			return launchPath, nil
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root
			break
		}
		current = parent
	}

	return "", fmt.Errorf("no %s/%s found in %s or parent directories", VSCodeDirName, LaunchJSONFileName, startPath)
}

// LoadAndDiscover combines discovery and loading: finds a launch.json from the start path
// and loads it.
func LoadAndDiscover(startPath string) (*LaunchJSON, string, error) {
	path, err := Discover(startPath)
	if err != nil {
		return nil, "", err
	}

	lj, err := LoadFromPath(path)
	if err != nil {
		return nil, "", err
	}

	return lj, path, nil
}

// FindConfiguration finds a configuration by name in the LaunchJSON.
func FindConfiguration(lj *LaunchJSON, name string) (*DebugConfiguration, error) {
	for i := range lj.Configurations {
		if lj.Configurations[i].Name == name {
			return &lj.Configurations[i], nil
		}
	}
	return nil, fmt.Errorf("configuration %q not found", name)
}

// FindCompound finds a compound configuration by name.
func FindCompound(lj *LaunchJSON, name string) (*CompoundConfig, error) {
	for i := range lj.Compounds {
		if lj.Compounds[i].Name == name {
			return &lj.Compounds[i], nil
		}
	}
	return nil, fmt.Errorf("compound configuration %q not found", name)
}

// ListConfigurationNames returns a list of all configuration names.
func ListConfigurationNames(lj *LaunchJSON) []string {
	names := make([]string, len(lj.Configurations))
	for i, cfg := range lj.Configurations {
		names[i] = cfg.Name
	}
	return names
}

// ListCompoundNames returns a list of all compound configuration names.
func ListCompoundNames(lj *LaunchJSON) []string {
	names := make([]string, len(lj.Compounds))
	for i, compound := range lj.Compounds {
		names[i] = compound.Name
	}
	return names
}

// ConfigurationInfo provides summary information about a configuration.
type ConfigurationInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Request string `json:"request"`
}

// ListConfigurations returns summary information about all configurations.
func ListConfigurations(lj *LaunchJSON) []ConfigurationInfo {
	infos := make([]ConfigurationInfo, len(lj.Configurations))
	for i, cfg := range lj.Configurations {
		infos[i] = ConfigurationInfo{
			Name:    cfg.Name,
			Type:    cfg.Type,
			Request: cfg.Request,
		}
	}
	return infos
}

// CompoundInfo provides summary information about a compound configuration.
type CompoundInfo struct {
	Name           string   `json:"name"`
	Configurations []string `json:"configurations"`
	StopAll        bool     `json:"stopAll"`
}

// ListCompounds returns summary information about all compound configurations.
func ListCompounds(lj *LaunchJSON) []CompoundInfo {
	infos := make([]CompoundInfo, len(lj.Compounds))
	for i, compound := range lj.Compounds {
		infos[i] = CompoundInfo{
			Name:           compound.Name,
			Configurations: compound.Configurations,
			StopAll:        compound.StopAll,
		}
	}
	return infos
}

// FindInput finds an input configuration by ID.
func FindInput(lj *LaunchJSON, id string) (*InputConfig, error) {
	for i := range lj.Inputs {
		if lj.Inputs[i].ID == id {
			return &lj.Inputs[i], nil
		}
	}
	return nil, fmt.Errorf("input %q not found", id)
}

// GetWorkspaceFolder derives the workspace folder from the launch.json path.
// The workspace folder is the parent of the .vscode directory.
// Returns POSIX-style paths (forward slashes) for cross-platform consistency.
func GetWorkspaceFolder(launchJSONPath string) string {
	// launch.json is at: <workspace>/.vscode/launch.json
	// So we go up two directories
	vscodeDir := filepath.Dir(launchJSONPath)
	workspace := filepath.Dir(vscodeDir)
	// Normalize to forward slashes for cross-platform consistency
	return filepath.ToSlash(workspace)
}

// ValidateConfiguration performs basic validation on a configuration.
func ValidateConfiguration(cfg *DebugConfiguration) error {
	if cfg.Name == "" {
		return fmt.Errorf("configuration name is required")
	}
	if cfg.Type == "" {
		return fmt.Errorf("configuration type is required")
	}
	if cfg.Request == "" {
		return fmt.Errorf("configuration request is required")
	}
	if cfg.Request != "launch" && cfg.Request != "attach" {
		return fmt.Errorf("configuration request must be 'launch' or 'attach', got %q", cfg.Request)
	}
	return nil
}

// ValidateLaunchJSON performs validation on the entire launch.json.
func ValidateLaunchJSON(lj *LaunchJSON) []error {
	var errors []error

	// Validate all configurations
	for i, cfg := range lj.Configurations {
		if err := ValidateConfiguration(&cfg); err != nil {
			errors = append(errors, fmt.Errorf("configuration[%d]: %w", i, err))
		}
	}

	// Validate compounds reference existing configurations
	configNames := make(map[string]bool)
	for _, cfg := range lj.Configurations {
		configNames[cfg.Name] = true
	}

	for i, compound := range lj.Compounds {
		if compound.Name == "" {
			errors = append(errors, fmt.Errorf("compound[%d]: name is required", i))
		}
		for _, cfgName := range compound.Configurations {
			if !configNames[cfgName] {
				errors = append(errors, fmt.Errorf("compound %q references unknown configuration %q", compound.Name, cfgName))
			}
		}
	}

	return errors
}
