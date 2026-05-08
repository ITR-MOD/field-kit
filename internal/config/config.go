package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GameInstall represents a registered game installation.
type GameInstall struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"` // e.g. "itr2", "itr1-1.0" — see config.Version* constants
}

// Config is the global application config.
type Config struct {
	Games          []GameInstall     `json:"games"`
	ActiveGame     string            `json:"active_game"`
	ActiveProfiles map[string]string `json:"active_profiles,omitempty"`
}

var (
	dataDir    string
	configPath string
	cfg        *Config
)

// DataDir returns the root data directory for the application.
func DataDir() string {
	return dataDir
}

// ModsDir returns the directory where mod caches are stored.
func ModsDir() string {
	return filepath.Join(dataDir, "mods")
}

// ArchivesDir returns the directory where original zip archives are stored.
func ArchivesDir() string {
	return filepath.Join(dataDir, "mods", "archives")
}

// ProfilesDir returns the root directory where profiles are stored.
func ProfilesDir() string {
	return filepath.Join(dataDir, "profiles")
}

// GameProfilesDir returns the profile directory for a specific game install.
func GameProfilesDir(gameID string) string {
	return filepath.Join(dataDir, "profiles", sanitize(gameID))
}

// BackupsDir returns the root directory for per-game-install backups.
func BackupsDir() string {
	return filepath.Join(dataDir, "backups")
}

// GameBackupDir returns the backup dir for a specific game install (keyed by game ID).
func GameBackupDir(gameID string) string {
	return filepath.Join(dataDir, "backups", sanitize(gameID))
}

// StatePath returns the path to the deployment state file.
func StatePath() string {
	return filepath.Join(dataDir, "state.json")
}

// Init initialises the data directory and loads or creates the config.
func Init() error {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
	}
	dataDir = filepath.Join(base, "itr-field-kit")
	configPath = filepath.Join(dataDir, "config.json")

	for _, d := range []string{
		dataDir,
		ModsDir(),
		ArchivesDir(),
		ProfilesDir(),
		BackupsDir(),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	return load()
}

func load() error {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		cfg = &Config{}
		return save()
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cfg = &Config{}
	return json.Unmarshal(data, cfg)
}

func save() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// Get returns the loaded config (read-only).
func Get() *Config {
	return cfg
}

// Save persists any changes made to the config returned by Get().
func Save() error {
	return save()
}

// ActiveGameInstall returns the currently active game installation, or nil.
func ActiveGameInstall() *GameInstall {
	for i := range cfg.Games {
		if cfg.Games[i].ID == cfg.ActiveGame {
			return &cfg.Games[i]
		}
	}
	return nil
}

// GameByID returns a game install by ID.
func GameByID(id string) *GameInstall {
	for i := range cfg.Games {
		if cfg.Games[i].ID == id {
			return &cfg.Games[i]
		}
	}
	return nil
}

// AddGame adds a new game installation and saves.
// The game version is auto-detected from the install path.
func AddGame(name, path string) (GameInstall, error) {
	id := makeGameID(path)
	if GameByID(id) != nil {
		return GameInstall{}, fmt.Errorf("game already registered with id %q", id)
	}
	g := GameInstall{ID: id, Name: name, Path: path, Version: DetectGameVersion(path)}
	cfg.Games = append(cfg.Games, g)
	if cfg.ActiveGame == "" {
		cfg.ActiveGame = id
	}
	return g, save()
}

// SetActiveGame updates the active game and saves.
func SetActiveGame(id string) error {
	if GameByID(id) == nil {
		return fmt.Errorf("no game with id %q", id)
	}
	cfg.ActiveGame = id
	return save()
}

// GetActiveProfile returns the active profile name for the given game install.
// Returns "" if no profile has been set for that game.
func GetActiveProfile(gameID string) string {
	if cfg == nil || cfg.ActiveProfiles == nil {
		return ""
	}
	return cfg.ActiveProfiles[gameID]
}

// SetActiveProfile updates the active profile for the given game and saves.
func SetActiveProfile(gameID, name string) error {
	if cfg.ActiveProfiles == nil {
		cfg.ActiveProfiles = make(map[string]string)
	}
	cfg.ActiveProfiles[gameID] = name
	return save()
}

// makeGameID creates a stable short ID from a game path.
func makeGameID(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:4])
}

// sanitize makes a string safe for use as a directory name.
func sanitize(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(s)
}
