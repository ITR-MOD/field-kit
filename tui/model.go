// Package tui provides the terminal user interface for ITR Field Kit.
package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Merith-TK/itr-field-kit/internal/config"
	"github.com/Merith-TK/itr-field-kit/internal/deploy"
	"github.com/Merith-TK/itr-field-kit/internal/modmgr"
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
)

// ─── Message types ───────────────────────────────────────────────────────────

type msgStatus struct {
	text    string
	isError bool
}
type msgRefresh struct{}
type msgShowInput struct {
	prompt string
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
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink

	case msgImportDone:
		m.reload()
		if msg.err != nil {
			return m, statusCmd(msg.err.Error(), true)
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
	case "enter", " ":
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
			// Nothing detected — go straight to manual input.
			return m, func() tea.Msg {
				return msgShowInput{
					prompt: "No Steam installations detected. Enter game path manually:",
					onDone: func(path string) tea.Cmd {
						return gameAskName(trimInput(path))
					},
				}
			}
		}
		// Show the detect overlay with found paths + manual option.
		m.overlay = overlayDetect
		m.detectPaths = append(found, detectManualSentinel)
		m.detectCursor = 0
	case "delete", "d":
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
		m.openFilePicker(func(path string) tea.Cmd {
			return cmdImport(path)
		})
	case "delete", "d":
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
	case "enter", " ":
		if n > 0 {
			name := m.profiles[*cur]
			if err := config.SetActiveProfile(name); err != nil {
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
	case "delete", "d":
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
		profile, err := modmgr.LoadProfile(config.Get().ActiveProfile)
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
			// Show manual input.
			return m, func() tea.Msg {
				return msgShowInput{
					prompt: "Enter game installation path:",
					onDone: func(path string) tea.Cmd {
						return gameAskName(trimInput(path))
					},
				}
			}
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

// ─── Commands ────────────────────────────────────────────────────────────────

// detectManualSentinel is the last entry in detectPaths to represent "manual entry".
const detectManualSentinel = "__manual__"

// gameAskName shows the name-input overlay for a known path, then adds the game.
func gameAskName(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		return msgShowInput{
			prompt: "Display name (blank = \"Into the Radius 2\"):",
			onDone: func(name string) tea.Cmd {
				return cmdAddGame(path, name)
			},
		}
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
		name = "Into the Radius 2"
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
	return func() tea.Msg {
		p := &modmgr.Profile{Name: name}
		if err := p.Save(); err != nil {
			return msgStatus{text: err.Error(), isError: true}
		}
		return msgRefresh{}
	}
}

func cmdDeleteProfile(name string) func() tea.Cmd {
	return func() tea.Cmd {
		return func() tea.Msg {
			if err := os.Remove(modmgr.ProfilePath(name)); err != nil && !os.IsNotExist(err) {
				return msgStatus{text: err.Error(), isError: true}
			}
			return msgRefresh{}
		}
	}
}

// ─── Data helpers ─────────────────────────────────────────────────────────────

func (m *model) reload() {
	m.games = config.Get().Games
	mods, _ := modmgr.ListMods()
	m.mods = mods
	names, _ := modmgr.ListProfiles()
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
	p, _ := modmgr.LoadProfile(m.profiles[idx])
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
