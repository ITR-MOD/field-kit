// Package tui provides the terminal user interface for ITR Field Kit.
package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ITR-MOD/field-kit/internal/config"
	"github.com/ITR-MOD/field-kit/internal/deploy"
	"github.com/ITR-MOD/field-kit/internal/modmgr"
)

// ─── Tabs ────────────────────────────────────────────────────────────────────

const (
	tabGames = iota
	tabMods
	tabProfiles
	tabDeploy
	numTabs
)

var tabNames = [numTabs]string{"GAMES", "MODS", "PROFILES", "DEPLOY"}

// ─── Overlay kinds ───────────────────────────────────────────────────────────

type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayInput
	overlayConfirm
	overlayInfo
	overlayDetect     // Steam auto-detect results list
	overlayFilePicker // zip file browser
	overlayAdjust     // editable instruction list for manual destination overrides
	overlayFomod      // FOMOD install wizard
)

// ─── Message types ───────────────────────────────────────────────────────────

type msgStatus struct {
	text    string
	isError bool
}
type msgRefresh struct{}
type msgSetAdjustOverride struct {
	src     string
	dest    string
	isReset bool // true = delete/restore-to-default
}
type msgShowInput struct {
	prompt string
	value  string // optional pre-filled text
	onDone func(string) tea.Cmd
}
type msgImportDone struct {
	meta *modmgr.ModMeta
	err  error
}
type msgDeployDone struct{ err error }
type msgUndeployDone struct{ err error }

// ─── Model ───────────────────────────────────────────────────────────────────

type model struct {
	width, height int

	activeTab int

	// Per-tab cursors (and scroll offsets for games/mods).
	cursors [numTabs]int
	offsets [numTabs]int

	// Cached data.
	games    []config.GameInstall
	mods     []*modmgr.ModMeta
	profiles []string

	// Profiles tab.
	selectedProfile    *modmgr.Profile
	profilesRightFocus bool
	profileModsCursor  int
	profileModsOffset  int

	// Deploy tab.
	deployState *deploy.State

	// Status bar.
	statusMsg   string
	statusError bool

	// Overlays.
	overlay       overlayKind
	textInput     textinput.Model
	inputPrompt   string
	inputOnDone   func(string) tea.Cmd
	confirmPrompt string
	confirmSel    int // 0=yes, 1=no
	confirmOnYes  func() tea.Cmd
	infoTitle     string
	infoLines     []string
	infoScroll    int

	// Detect overlay (Steam auto-detect).
	detectPaths  []string // found paths + sentinel "manual" as last entry
	detectCursor int

	// File picker overlay.
	filePicker filePickerState

	// Adjust overlay (manual destination override editor).
	adjustInstructions []modmgr.Instruction // instructions for the mod being adjusted
	adjustDefaults     []string             // mapper-default Dest per instruction (parallel slice)
	adjustOverrides    map[string]string    // edits in progress: source → override dest
	adjustExcluded     map[string]bool      // edits in progress: sources excluded from deployment (profile-level only)
	adjustModID        string               // mod being adjusted
	adjustCursor       int
	adjustScroll       int
	adjustIsProfile    bool // false = mod-level, true = profile-level

	// FOMOD wizard overlay.
	fomod fomodState
}

// New creates and initialises the model.
func New() model {
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = 60
	m := model{width: 80, height: 24, textInput: ti}
	m.reload()
	return m
}

// Init satisfies tea.Model.
func (m model) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.textInput.Width = imax(30, m.width-20)
		return m, nil

	case msgStatus:
		m.statusMsg, m.statusError = msg.text, msg.isError
		return m, nil

	case msgRefresh:
		m.reload()
		return m, nil

	case msgShowInput:
		m.overlay = overlayInput
		m.inputPrompt = msg.prompt
		m.inputOnDone = msg.onDone
		m.textInput.SetValue(msg.value)
		m.textInput.CursorEnd()
		m.textInput.Focus()
		return m, textinput.Blink

	case msgImportDone:
		m.reload()
		if msg.err != nil {
			return m, statusCmd(msg.err.Error(), true)
		}
		if msg.meta.IsFomodPending() {
			return m, statusCmd("imported \""+msg.meta.Name+"\" — FOMOD mod, press [f] in Mods tab to set its default", false)
		}
		return m, statusCmd("imported \""+msg.meta.Name+"\"", false)

	case msgDeployDone:
		if msg.err != nil {
			return m, statusCmd("deploy failed: "+msg.err.Error(), true)
		}
		m.refreshDeployState()
		return m, statusCmd("deploy complete", false)

	case msgUndeployDone:
		if msg.err != nil {
			return m, statusCmd("undeploy failed: "+msg.err.Error(), true)
		}
		m.refreshDeployState()
		return m, statusCmd("undeploy complete – originals restored", false)

	case msgSetAdjustOverride:
		if msg.isReset {
			delete(m.adjustOverrides, msg.src)
		} else {
			m.adjustOverrides[msg.src] = msg.dest
		}
		m.overlay = overlayAdjust
		return m, nil
	}

	if m.overlay != overlayNone {
		return m.updateOverlay(msg)
	}
	return m.updateNormal(msg)
}

// ─── Normal (non-overlay) key handling ───────────────────────────────────────

func (m model) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "1":
		m.activeTab = tabGames
	case "2":
		m.activeTab = tabMods
	case "3":
		m.activeTab = tabProfiles
	case "4":
		m.activeTab = tabDeploy
	case "tab":
		if m.activeTab == tabProfiles {
			m.profilesRightFocus = !m.profilesRightFocus
		} else {
			m.activeTab = (m.activeTab + 1) % numTabs
		}
	case "shift+tab":
		if m.activeTab != tabProfiles {
			m.activeTab = (m.activeTab + numTabs - 1) % numTabs
		}
	default:
		return m.updateTab(key)
	}
	return m, nil
}

func (m model) updateTab(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabGames:
		return m.updateGamesTab(key)
	case tabMods:
		return m.updateModsTab(key)
	case tabProfiles:
		return m.updateProfilesTab(key)
	case tabDeploy:
		return m.updateDeployTab(key)
	}
	return m, nil
}

// ── Games tab ─────────────────────────────────────────────────────────────

func (m model) updateGamesTab(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	cur := &m.cursors[tabGames]
	n := len(m.games)

	switch key.String() {
	case "up", "k":
		if *cur > 0 {
			*cur--
		}
	case "down", "j":
		if *cur < n-1 {
			*cur++
		}
	case "enter":
		if n > 0 {
			g := m.games[*cur]
			if err := config.SetActiveGame(g.ID); err != nil {
				return m, statusCmd(err.Error(), true)
			}
			m.reload()
			return m, statusCmd("active game: "+g.Name, false)
		}
	case "a":
		found := config.FindSteamInstalls()
		if len(found) == 0 {
			// Nothing detected — open exe browser directly.
			m.openFilePicker(FpFilterGameExe, func(path string) tea.Cmd {
				return gameAskName(filepath.Dir(path))
			})
			return m, nil
		}
		// Show the detect overlay with found paths + manual option.
		m.overlay = overlayDetect
		m.detectPaths = append(found, detectManualSentinel)
		m.detectCursor = 0
	case "f":
		// Browse for IntoTheRadius*.exe to locate a game install manually.
		m.openFilePicker(FpFilterGameExe, func(path string) tea.Cmd {
			// The exe lives at game root; pass the parent dir as game path.
			return gameAskName(filepath.Dir(path))
		})
	case "delete", "backspace":
		if n > 0 {
			g := m.games[*cur]
			m.overlay = overlayConfirm
			m.confirmPrompt = "Remove \"" + g.Name + "\" from manager?"
			m.confirmSel = 1
			m.confirmOnYes = cmdRemoveGame(g.ID)
		}
	}
	return m, nil
}

// ── Mods tab ──────────────────────────────────────────────────────────────

func (m model) updateModsTab(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	cur := &m.cursors[tabMods]
	n := len(m.mods)

	switch key.String() {
	case "up", "k":
		if *cur > 0 {
			*cur--
		}
	case "down", "j":
		if *cur < n-1 {
			*cur++
		}
	case "enter":
		if n > 0 {
			mod := m.mods[*cur]
			m.overlay = overlayInfo
			m.infoTitle = mod.Name
			m.infoLines = buildModInfoLines(mod)
			m.infoScroll = 0
		}
	case "i":
		m.openFilePicker(FpFilterArchives, func(path string) tea.Cmd {
			return cmdImport(path)
		})
	case "m":
		if n > 0 {
			m.openAdjustOverlay(m.mods[*cur], false)
		}
	case "f":
		if n > 0 {
			mod := m.mods[*cur]
			if !mod.IsFomod() {
				return m, statusCmd("\""+mod.Name+"\" is not a FOMOD mod", true)
			}
			return m, m.openFomodWizard(mod, false)
		}
	case "delete", "backspace":
		if n > 0 {
			mod := m.mods[*cur]
			m.overlay = overlayConfirm
			m.confirmPrompt = "Delete \"" + mod.Name + "\" from cache?"
			m.confirmSel = 1
			m.confirmOnYes = cmdRemoveMod(mod.ID)
		}
	}
	return m, nil
}

// ── Profiles tab ──────────────────────────────────────────────────────────

func (m model) updateProfilesTab(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.profilesRightFocus {
		return m.updateProfilesRight(key)
	}
	return m.updateProfilesLeft(key)
}

func (m model) updateProfilesLeft(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	cur := &m.cursors[tabProfiles]
	n := len(m.profiles)

	switch key.String() {
	case "up", "k":
		if *cur > 0 {
			*cur--
			m.loadSelectedProfile()
		}
	case "down", "j":
		if *cur < n-1 {
			*cur++
			m.loadSelectedProfile()
		}
	case "enter":
		if n > 0 {
			name := m.profiles[*cur]
			if err := config.SetActiveProfile(config.Get().ActiveGame, name); err != nil {
				return m, statusCmd(err.Error(), true)
			}
			m.reload()
			return m, statusCmd("active profile: "+name, false)
		}
	case "right", "l":
		if len(m.mods) > 0 {
			m.profilesRightFocus = true
		}
	case "n":
		return m, func() tea.Msg {
			return msgShowInput{
				prompt: "New profile name:",
				onDone: func(name string) tea.Cmd {
					return cmdNewProfile(name)
				},
			}
		}
	case "i":
		m.openFilePicker(FpFilterProfile, func(path string) tea.Cmd {
			return cmdImportProfile(path)
		})
	case "e":
		if n > 0 {
			name := m.profiles[*cur]
			m.openFilePicker(FpFilterSaveDir, func(destDir string) tea.Cmd {
				// Directory chosen — now ask for the filename.
				return func() tea.Msg {
					return msgShowInput{
						prompt: "Export filename:",
						value:  name + ".fieldkit.json",
						onDone: func(filename string) tea.Cmd {
							return cmdExportProfile(name, filepath.Join(destDir, filename))
						},
					}
				}
			})
		}
	case "delete", "backspace":
		if n > 0 {
			name := m.profiles[*cur]
			m.overlay = overlayConfirm
			m.confirmPrompt = "Delete profile \"" + name + "\"?"
			m.confirmSel = 1
			m.confirmOnYes = cmdDeleteProfile(name)
		}
	}
	return m, nil
}

func (m model) updateProfilesRight(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.mods)

	switch key.String() {
	case "up", "k":
		if m.profileModsCursor > 0 {
			m.profileModsCursor--
		}
	case "down", "j":
		if m.profileModsCursor < n-1 {
			m.profileModsCursor++
		}
	case "left", "h":
		m.profilesRightFocus = false
	case " ", "enter":
		if n > 0 && m.selectedProfile != nil {
			mod := m.mods[m.profileModsCursor]
			if m.selectedProfile.HasMod(mod.ID) {
				m.selectedProfile.RemoveMod(mod.ID)
			} else {
				m.selectedProfile.AddMod(mod.ID)
			}
			if err := m.selectedProfile.Save(); err != nil {
				return m, statusCmd(err.Error(), true)
			}
		}
	case "m":
		if n > 0 {
			mod := m.mods[m.profileModsCursor]
			if mod.IsFomod() {
				if mod.IsFomodPending() {
					return m, statusCmd("configure \""+mod.Name+"\"'s FOMOD default in the Mods tab first", true)
				}
				return m, m.openFomodWizard(mod, true)
			}
			m.openAdjustOverlay(mod, true)
		}
	}
	return m, nil
}

// ── Deploy tab ────────────────────────────────────────────────────────────

func (m model) updateDeployTab(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "d":
		game := config.ActiveGameInstall()
		if game == nil {
			return m, statusCmd("no active game – go to GAMES tab first", true)
		}
		profile, err := modmgr.LoadProfile(config.Get().ActiveGame, config.GetActiveProfile(config.Get().ActiveGame))
		if err != nil {
			return m, statusCmd(err.Error(), true)
		}
		if len(profile.Mods) == 0 {
			return m, statusCmd("active profile has no mods – add some in PROFILES tab", true)
		}
		m.overlay = overlayConfirm
		m.confirmPrompt = "Deploy \"" + profile.Name + "\" to " + game.Name + "?"
		m.confirmSel = 0
		m.confirmOnYes = func() tea.Cmd {
			return func() tea.Msg {
				err := deploy.Deploy(game, profile)
				return msgDeployDone{err: err}
			}
		}
	case "u":
		game := config.ActiveGameInstall()
		if game == nil {
			return m, statusCmd("no active game", true)
		}
		m.overlay = overlayConfirm
		m.confirmPrompt = "Undeploy and restore original game files?"
		m.confirmSel = 1
		m.confirmOnYes = func() tea.Cmd {
			return func() tea.Msg {
				err := deploy.Undeploy(game)
				return msgUndeployDone{err: err}
			}
		}
	case "r":
		m.refreshDeployState()
	}
	return m, nil
}

// ─── Overlay key handling ────────────────────────────────────────────────────

func (m model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayInput:
		return m.updateInputOverlay(msg)
	case overlayConfirm:
		return m.updateConfirmOverlay(msg)
	case overlayInfo:
		return m.updateInfoOverlay(msg)
	case overlayDetect:
		return m.updateDetectOverlay(msg)
	case overlayFilePicker:
		return m.updateFilePickerOverlay(msg)
	case overlayAdjust:
		return m.updateAdjustOverlay(msg)
	case overlayFomod:
		return m.updateFomodOverlay(msg)
	}
	return m, nil
}

func (m model) updateDetectOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	n := len(m.detectPaths) // last entry is always the "manual" sentinel
	switch key.String() {
	case "esc":
		m.overlay = overlayNone
	case "up", "k":
		if m.detectCursor > 0 {
			m.detectCursor--
		}
	case "down", "j":
		if m.detectCursor < n-1 {
			m.detectCursor++
		}
	case "enter", " ":
		chosen := m.detectPaths[m.detectCursor]
		m.overlay = overlayNone
		if chosen == detectManualSentinel {
			// Open exe browser for manual selection.
			m.openFilePicker(FpFilterGameExe, func(path string) tea.Cmd {
				return gameAskName(filepath.Dir(path))
			})
			return m, nil
		}
		// Auto-detected path chosen — ask for display name.
		return m, gameAskName(chosen)
	}
	return m, nil
}

func (m model) updateInputOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.overlay = overlayNone
			m.textInput.Blur()
			return m, nil
		case "enter":
			val := m.textInput.Value()
			onDone := m.inputOnDone
			m.overlay = overlayNone
			m.textInput.Blur()
			m.textInput.SetValue("")
			if onDone != nil {
				return m, onDone(val)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateConfirmOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "left", "h", "right", "l":
		m.confirmSel ^= 1
	case "y":
		onYes := m.confirmOnYes
		m.overlay = overlayNone
		if onYes != nil {
			return m, onYes()
		}
	case "n", "esc":
		m.overlay = overlayNone
	case "enter":
		if m.confirmSel == 0 {
			onYes := m.confirmOnYes
			m.overlay = overlayNone
			if onYes != nil {
				return m, onYes()
			}
		} else {
			m.overlay = overlayNone
		}
	}
	return m, nil
}

func (m model) updateInfoOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "q", "enter":
		m.overlay = overlayNone
	case "up", "k":
		if m.infoScroll > 0 {
			m.infoScroll--
		}
	case "down", "j":
		if m.infoScroll < len(m.infoLines)-1 {
			m.infoScroll++
		}
	}
	return m, nil
}

// ─── Adjust overlay ───────────────────────────────────────────────────────────

// openAdjustOverlay opens the manual destination editor for a mod.
// isProfile=true scopes the edits to the currently selected profile;
// isProfile=false edits the mod's own overrides.fk.json.
func (m *model) openAdjustOverlay(mod *modmgr.ModMeta, isProfile bool) {
	game := config.ActiveGameInstall()
	gameID := ""
	if game != nil {
		gameID = config.GameInternalID(game.Version)
	} else {
		gameID = config.GameInternalID(config.VersionITR2)
	}

	var instructions []modmgr.Instruction
	if mod.IsFomod() {
		instructions = modmgr.MapFomodEntries(m.fomodEffectiveEntries(mod, isProfile))
	} else {
		instructions = modmgr.MapFilesForGame(mod.Files, gameID)
	}

	// Snapshot mapper-default Dest for each instruction.
	defaults := make([]string, len(instructions))
	for i, instr := range instructions {
		defaults[i] = instr.Dest
	}

	// Load existing overrides into the editing map.
	overrides := make(map[string]string)
	excluded := make(map[string]bool)
	if isProfile && m.selectedProfile != nil {
		if m.selectedProfile.Overrides != nil {
			if modOvr, ok := m.selectedProfile.Overrides[mod.ID]; ok {
				for k, v := range modOvr.Sources {
					overrides[k] = v
				}
				for _, src := range modOvr.Excluded {
					excluded[src] = true
				}
			}
		}
	} else {
		modOvr, _ := modmgr.LoadModOverrides(mod.ID)
		if modOvr != nil {
			for k, v := range modOvr.Sources {
				overrides[k] = v
			}
		}
	}

	m.adjustInstructions = instructions
	m.adjustDefaults = defaults
	m.adjustOverrides = overrides
	m.adjustExcluded = excluded
	m.adjustModID = mod.ID
	m.adjustCursor = 0
	m.adjustScroll = 0
	m.adjustIsProfile = isProfile
	m.overlay = overlayAdjust
}

// saveAdjustOverlay persists current edits and closes the overlay.
func (m *model) saveAdjustOverlay() tea.Cmd {
	modID := m.adjustModID

	if m.adjustIsProfile && m.selectedProfile != nil {
		excludedList := make([]string, 0, len(m.adjustExcluded))
		for src := range m.adjustExcluded {
			excludedList = append(excludedList, src)
		}
		if m.selectedProfile.Overrides == nil {
			m.selectedProfile.Overrides = make(map[string]modmgr.ModOverrides)
		}
		if len(m.adjustOverrides) == 0 && len(excludedList) == 0 {
			delete(m.selectedProfile.Overrides, modID)
		} else {
			m.selectedProfile.Overrides[modID] = modmgr.ModOverrides{Sources: m.adjustOverrides, Excluded: excludedList}
		}
		if err := m.selectedProfile.Save(); err != nil {
			return statusCmd(err.Error(), true)
		}
	} else {
		overrides := &modmgr.ModOverrides{Sources: m.adjustOverrides}
		if err := modmgr.SaveModOverrides(modID, overrides); err != nil {
			return statusCmd(err.Error(), true)
		}
	}
	m.overlay = overlayNone
	return statusCmd("overrides saved", false)
}

func (m model) updateAdjustOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	instrs := m.adjustInstructions
	// Build a navigable index skipping WriteContent rows (manager-written, non-editable).
	var editableIdx []int
	for i, instr := range instrs {
		if instr.WriteContent == "" {
			editableIdx = append(editableIdx, i)
		}
	}
	ne := len(editableIdx)

	switch key.String() {
	case "esc", "s":
		return m, m.saveAdjustOverlay()

	case "up", "k":
		if m.adjustCursor > 0 {
			m.adjustCursor--
		}

	case "down", "j":
		if m.adjustCursor < ne-1 {
			m.adjustCursor++
		}

	case "enter":
		if ne == 0 {
			break
		}
		instrIdx := editableIdx[m.adjustCursor]
		instr := instrs[instrIdx]
		// Current effective dest: override if set, else mapper default.
		current := m.adjustDefaults[instrIdx]
		if ov, ok := m.adjustOverrides[instr.Source]; ok {
			current = ov
		}
		src := instr.Source
		defaultDest := m.adjustDefaults[instrIdx]
		return m, func() tea.Msg {
			return msgShowInput{
				prompt: "Destination for " + src + ":",
				value:  current,
				onDone: func(newDest string) tea.Cmd {
					newDest = strings.TrimSpace(newDest)
					isReset := newDest == "" || newDest == defaultDest
					return func() tea.Msg {
						return msgSetAdjustOverride{src: src, dest: newDest, isReset: isReset}
					}
				},
			}
		}

	case "r":
		if ne == 0 {
			break
		}
		src := instrs[editableIdx[m.adjustCursor]].Source
		delete(m.adjustOverrides, src)

	case "x":
		if !m.adjustIsProfile || ne == 0 {
			break
		}
		src := instrs[editableIdx[m.adjustCursor]].Source
		if m.adjustExcluded[src] {
			delete(m.adjustExcluded, src)
		} else {
			m.adjustExcluded[src] = true
		}
	}

	return m, nil
}

// ─── Commands ────────────────────────────────────────────────────────────────

// detectManualSentinel is the last entry in detectPaths to represent "manual entry".
const detectManualSentinel = "__manual__"

// gameAskName shows the name-input overlay for a known path, then adds the game.
func gameAskName(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	defaultName := defaultGameName(path)
	return func() tea.Msg {
		return msgShowInput{
			prompt: "Display name (blank = \"" + defaultName + "\"):",
			onDone: func(name string) tea.Cmd {
				return cmdAddGame(path, name)
			},
		}
	}
}

// defaultGameName returns a version-aware default display name for a game path.
func defaultGameName(path string) string {
	switch config.DetectGameVersion(path) {
	case config.VersionITR1_07a:
		return "Into the Radius v0.7a"
	case config.VersionITR1_10:
		return "Into the Radius v1.0"
	case config.VersionITR1_27:
		return "Into the Radius v2.7"
	default:
		return "Into the Radius 2"
	}
}

func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return msgStatus{text: text, isError: isErr} }
}

func cmdImport(path string) tea.Cmd {
	path = trimInput(path)
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		meta, err := modmgr.Import(path)
		return msgImportDone{meta: meta, err: err}
	}
}

func cmdAddGame(path, name string) tea.Cmd {
	if name == "" {
		name = defaultGameName(path)
	}
	return func() tea.Msg {
		if _, err := config.AddGame(name, path); err != nil {
			return msgStatus{text: err.Error(), isError: true}
		}
		return msgRefresh{}
	}
}

func cmdRemoveGame(id string) func() tea.Cmd {
	return func() tea.Cmd {
		return func() tea.Msg {
			cfg := config.Get()
			games := cfg.Games[:0]
			for _, g := range cfg.Games {
				if g.ID != id {
					games = append(games, g)
				}
			}
			cfg.Games = games
			if cfg.ActiveGame == id {
				if len(games) > 0 {
					cfg.ActiveGame = games[0].ID
				} else {
					cfg.ActiveGame = ""
				}
			}
			if err := config.Save(); err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgRefresh{}
		}
	}
}

func cmdRemoveMod(id string) func() tea.Cmd {
	return func() tea.Cmd {
		return func() tea.Msg {
			if err := modmgr.RemoveMod(id); err != nil {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgRefresh{}
		}
	}
}

func cmdNewProfile(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	gameID := config.Get().ActiveGame
	return func() tea.Msg {
		p := &modmgr.Profile{GameID: gameID, Name: name}
		if err := p.Save(); err != nil {
			return msgStatus{text: err.Error(), isError: true}
		}
		return msgRefresh{}
	}
}

func cmdDeleteProfile(name string) func() tea.Cmd {
	gameID := config.Get().ActiveGame
	return func() tea.Cmd {
		return func() tea.Msg {
			if err := modmgr.DeleteProfile(gameID, name); err != nil && !os.IsNotExist(err) {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgRefresh{}
		}
	}
}

// cmdImportProfile reads a .fieldkit.json file and saves it as a local profile.
func cmdImportProfile(path string) tea.Cmd {
	gameID := config.Get().ActiveGame
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return msgStatus{text: "import profile: " + err.Error(), isError: true}
		}
		var p modmgr.Profile
		if err := json.Unmarshal(data, &p); err != nil {
			return msgStatus{text: "import profile: invalid file: " + err.Error(), isError: true}
		}
		if p.Name == "" {
			base := filepath.Base(path)
			p.Name = strings.TrimSuffix(base, ".fieldkit.json")
		}
		p.GameID = gameID
		if err := p.Save(); err != nil {
			return msgStatus{text: "import profile: " + err.Error(), isError: true}
		}
		return msgRefresh{}
	}
}

// cmdExportProfile writes a profile as a .fieldkit.json to the given full path.
// outPath is the complete destination (dir + filename) chosen via the save dialog.
func cmdExportProfile(name, outPath string) tea.Cmd {
	gameID := config.Get().ActiveGame
	return func() tea.Msg {
		p, err := modmgr.LoadProfile(gameID, name)
		if err != nil {
			return msgStatus{text: "export profile: " + err.Error(), isError: true}
		}
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return msgStatus{text: "export profile: " + err.Error(), isError: true}
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return msgStatus{text: "export profile: " + err.Error(), isError: true}
		}
		return msgStatus{text: "exported: " + outPath, isError: false}
	}
}

// ─── Data helpers ─────────────────────────────────────────────────────────────

func (m *model) reload() {
	m.games = config.Get().Games
	mods, _ := modmgr.ListMods()
	m.mods = mods
	names, _ := modmgr.ListProfiles(config.Get().ActiveGame)
	m.profiles = names

	clamp(&m.cursors[tabGames], len(m.games))
	clamp(&m.cursors[tabMods], len(m.mods))
	clamp(&m.cursors[tabProfiles], len(m.profiles))
	clamp(&m.profileModsCursor, len(m.mods))

	m.loadSelectedProfile()
	m.refreshDeployState()
}

func (m *model) loadSelectedProfile() {
	if len(m.profiles) == 0 {
		m.selectedProfile = nil
		return
	}
	idx := m.cursors[tabProfiles]
	if idx >= len(m.profiles) {
		idx = len(m.profiles) - 1
	}
	p, _ := modmgr.LoadProfile(config.Get().ActiveGame, m.profiles[idx])
	m.selectedProfile = p
}

func (m *model) refreshDeployState() {
	s, _ := deploy.Status()
	m.deployState = s
}

func buildModInfoLines(m *modmgr.ModMeta) []string {
	lines := []string{
		"ID:       " + m.ID,
		"Archive:  " + m.ArchiveName,
		"Imported: " + m.ImportedAt.Format("2006-01-02 15:04"),
		"",
		"── Install map ──",
	}
	for _, instr := range modmgr.MapFiles(m.Files) {
		custom := ""
		if instr.IsCustom {
			custom = "  [backup]"
		}
		lines = append(lines, "  "+instr.Source)
		lines = append(lines, "    → "+instr.Dest+custom)
	}
	return lines
}

// ─── Utilities ────────────────────────────────────────────────────────────────

func clamp(cur *int, n int) {
	if n == 0 {
		*cur = 0
	} else if *cur >= n {
		*cur = n - 1
	}
}

func trimInput(s string) string {
	s = strings.TrimSpace(s)
	// Strip surrounding quotes (drag-and-drop on some terminals adds them).
	if len(s) >= 2 && ((s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"')) {
		s = s[1 : len(s)-1]
	}
	return s
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
