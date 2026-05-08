// Package gui provides the Wails application bridge, binding Go backend
// methods to the frontend JavaScript runtime.
package gui

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ITR-MOD/field-kit/internal/config"
	"github.com/ITR-MOD/field-kit/internal/deploy"
	"github.com/ITR-MOD/field-kit/internal/modmgr"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. Methods on this type are automatically
// exposed to the frontend as window.go.gui.App.<MethodName>().
type App struct {
	ctx context.Context
}

// NewApp creates the App instance used by wails.Run.
func NewApp() *App { return &App{} }

// Startup is called by Wails after the webview is ready.
func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

// ── Installs ──────────────────────────────────────────────────────────────────

// GetInstalls returns all registered game installations.
func (a *App) GetInstalls() []config.GameInstall {
	c := config.Get()
	if c == nil || len(c.Games) == 0 {
		return []config.GameInstall{}
	}
	return c.Games
}

// GetActiveGameID returns the currently active game install ID.
func (a *App) GetActiveGameID() string {
	c := config.Get()
	if c == nil {
		return ""
	}
	return c.ActiveGame
}

// SetActiveGame marks a game install as active and persists it.
func (a *App) SetActiveGame(id string) error {
	return config.SetActiveGame(id)
}

// AutoDetectInstalls scans Steam library paths for known game installations
// and returns any newly registered installs.
func (a *App) AutoDetectInstalls() ([]config.GameInstall, error) {
	paths := config.FindSteamInstalls()
	var added []config.GameInstall
	for _, p := range paths {
		name := "Into The Radius"
		if v := config.DetectGameVersion(p); v == config.VersionITR2 {
			name = "Into The Radius 2"
		}
		g, err := config.AddGame(name, p)
		if err != nil {
			// Already registered — skip silently.
			continue
		}
		added = append(added, g)
	}
	return added, nil
}

// ── Mods ──────────────────────────────────────────────────────────────────────

// ModInfo is the frontend-facing representation of a mod, combining
// ModMeta with display helpers.
type ModInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Game    string `json:"game"`    // display label: "ITR2", "ITR1 v2.7", etc.
	Types   []string `json:"types"` // mod type strings
	Files   int    `json:"files"`
	Status  string `json:"status"` // "imported" for now; deploy layer enriches this
}

// GetMods returns all imported mods from the local cache.
func (a *App) GetMods() ([]ModInfo, error) {
	metas, err := modmgr.ListMods()
	if err != nil {
		return nil, err
	}
	var out []ModInfo
	for _, m := range metas {
		out = append(out, modMetaToInfo(m))
	}
	return out, nil
}

func modMetaToInfo(m *modmgr.ModMeta) ModInfo {
	types := make([]string, len(m.Types))
	for i, t := range m.Types {
		types[i] = string(t)
	}
	return ModInfo{
		ID:      m.ID,
		Name:    m.Name,
		Version: "",  // ModMeta has no version field yet; leave blank
		Game:    "any",
		Types:   types,
		Files:   len(m.Files),
		Status:  "imported",
	}
}

// ── Profiles ──────────────────────────────────────────────────────────────────

// GetProfileNames returns the names of all saved profiles.
func (a *App) GetProfileNames() ([]string, error) {
	return modmgr.ListProfiles()
}

// GetActiveProfileName returns the currently active profile name.
func (a *App) GetActiveProfileName() string {
	c := config.Get()
	if c == nil {
		return ""
	}
	return c.ActiveProfile
}

// GetProfile loads a profile by name.
func (a *App) GetProfile(name string) (*modmgr.Profile, error) {
	return modmgr.LoadProfile(name)
}

// NewProfile creates an empty profile with the given name.
func (a *App) NewProfile(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	p := &modmgr.Profile{Name: name}
	return p.Save()
}

// SetActiveProfile marks a profile as active and persists it.
func (a *App) SetActiveProfile(name string) error {
	return config.SetActiveProfile(name)
}

// ProfileAddMod adds a mod to the named profile.
func (a *App) ProfileAddMod(profileName, modID string) error {
	p, err := modmgr.LoadProfile(profileName)
	if err != nil {
		return err
	}
	p.AddMod(modID)
	return p.Save()
}

// ProfileRemoveMod removes a mod from the named profile.
func (a *App) ProfileRemoveMod(profileName, modID string) error {
	p, err := modmgr.LoadProfile(profileName)
	if err != nil {
		return err
	}
	p.RemoveMod(modID)
	return p.Save()
}

// ── Deploy ────────────────────────────────────────────────────────────────────

// DeployResult is returned to the frontend after a deploy operation.
type DeployResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Deploy runs a full deploy of the given profile into the given game install.
func (a *App) Deploy(gameID, profileName string) DeployResult {
	g := config.GameByID(gameID)
	if g == nil {
		return DeployResult{OK: false, Message: fmt.Sprintf("no game with id %q", gameID)}
	}
	p, err := modmgr.LoadProfile(profileName)
	if err != nil {
		return DeployResult{OK: false, Message: err.Error()}
	}
	if err := deploy.Deploy(g, p); err != nil {
		return DeployResult{OK: false, Message: err.Error()}
	}
	return DeployResult{OK: true, Message: "deploy complete"}
}

// Undeploy removes all deployed mod files from the given game install.
func (a *App) Undeploy(gameID string) DeployResult {
	g := config.GameByID(gameID)
	if g == nil {
		return DeployResult{OK: false, Message: fmt.Sprintf("no game with id %q", gameID)}
	}
	if err := deploy.Undeploy(g); err != nil {
		return DeployResult{OK: false, Message: err.Error()}
	}
	return DeployResult{OK: true, Message: "undeploy complete"}
}

// ── Mod import / removal ──────────────────────────────────────────────────────

// ImportModZip opens a file-picker dialog and imports the selected .zip as a mod.
// Returns nil without error when the user cancels.
func (a *App) ImportModZip() (*ModInfo, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Import Mod (.zip)",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Zip Archives (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	m, err := modmgr.Import(path)
	if err != nil {
		return nil, err
	}
	info := modMetaToInfo(m)
	return &info, nil
}

// ImportModFolder opens a directory-picker dialog and imports the selected folder as a mod.
// Returns nil without error when the user cancels.
func (a *App) ImportModFolder() (*ModInfo, error) {
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Import Mod Folder",
	})
	if err != nil || path == "" {
		return nil, err
	}
	m, err := modmgr.ImportFolder(path)
	if err != nil {
		return nil, err
	}
	info := modMetaToInfo(m)
	return &info, nil
}

// RemoveMod deletes a mod and all its cached files.
func (a *App) RemoveMod(id string) error {
	return modmgr.RemoveMod(id)
}

// ── Profile management ────────────────────────────────────────────────────────

// DeleteProfile removes a saved profile by name.
func (a *App) DeleteProfile(name string) error {
	return modmgr.DeleteProfile(name)
}

// DuplicateProfile copies srcName into a new profile named newName.
func (a *App) DuplicateProfile(srcName, newName string) error {
	if newName == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	src, err := modmgr.LoadProfile(srcName)
	if err != nil {
		return err
	}
	dup := &modmgr.Profile{Name: newName, Mods: append([]string{}, src.Mods...)}
	return dup.Save()
}

// ── Utility ───────────────────────────────────────────────────────────────────

// OpenFolder reveals a directory in the OS file manager.
func (a *App) OpenFolder(path string) error {
	return exec.Command("explorer", path).Start()
}
