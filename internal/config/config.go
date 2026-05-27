package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// File is the serialisable configuration structure.
type File struct {
	Port       int    `json:"port"`
	VMName     string `json:"vmName"`
	ScriptsDir string `json:"scriptsDir"`
	SSHKeyPath string `json:"sshKeyPath"`

	// Discord bot — all optional. Bot is disabled when DiscordToken is empty.
	DiscordToken     string `json:"discordToken,omitempty"`
	DiscordGuildID   string `json:"discordGuildID,omitempty"`
	DiscordChannelID string `json:"discordChannelID,omitempty"`
	DiscordRoleID    string `json:"discordRoleID,omitempty"`

	// Auto-update — set to "owner/repo" to enable GitHub release checks.
	GitHubRepo string `json:"githubRepo,omitempty"`
}

const (
	defaultPort       = 7374
	defaultVMName     = "dune-awakening"
	defaultGitHubRepo = "oldbear24/DuneManager"
)

var (
	mu       sync.RWMutex
	current  File
	filePath string
)

// Init loads the configuration from disk, falling back to defaults for any
// missing field.  Safe to call multiple times; only the first call matters.
func Init() {
	filePath = resolvePath()
	current = defaults()

	data, err := os.ReadFile(filePath)
	if err == nil {
		tmp := current
		if json.Unmarshal(data, &tmp) == nil {
			if tmp.Port == 0 {
				tmp.Port = defaultPort
			}
			if tmp.VMName == "" {
				tmp.VMName = defaultVMName
			}
			if tmp.SSHKeyPath == "" {
				tmp.SSHKeyPath = defaults().SSHKeyPath
			}
			if tmp.ScriptsDir == "" {
				tmp.ScriptsDir = defaults().ScriptsDir
			}
			if tmp.GitHubRepo == "" {
				tmp.GitHubRepo = defaultGitHubRepo
			}
			current = tmp
		}
	}
}

func resolvePath() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "dune-manager.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// New files default to exe directory.
		return p
	}
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "DuneAwakeningServer", "dune-manager.json")
}

func defaults() File {
	return File{
		Port:       defaultPort,
		VMName:     defaultVMName,
		ScriptsDir: defaultScriptsDir(),
		SSHKeyPath: defaultSSHKeyPath(),
		GitHubRepo: defaultGitHubRepo,
	}
}

func defaultSSHKeyPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "DuneAwakeningServer", "sshKey")
}

func defaultScriptsDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "bats", "battlegroup-management")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		return filepath.Join(filepath.Dir(exe), "bats", "battlegroup-management")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "bats", "battlegroup-management")
}

// Get returns a snapshot of the current configuration.
func Get() File {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Set replaces the in-memory configuration.
func Set(f File) {
	mu.Lock()
	current = f
	mu.Unlock()
}

// Save writes the current configuration to disk.
func Save() error {
	mu.RLock()
	f := current
	mu.RUnlock()
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	return os.WriteFile(filePath, data, 0644)
}

// FilePath returns the config file path being used.
func FilePath() string { return filePath }

// Convenience accessors —————————————————————————————————————————————————

func Port() int          { return Get().Port }
func VMName() string     { return Get().VMName }
func SSHKeyPath() string { return Get().SSHKeyPath }
func ScriptsDir() string { return Get().ScriptsDir }

func VMUtilitiesPS() string { return filepath.Join(ScriptsDir(), "vm-utilities.ps1") }

// ServiceAddr returns the address the background service listens on.
func ServiceAddr() string { return fmt.Sprintf("127.0.0.1:%d", Port()) }
