package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ITR-MOD/field-kit/internal/fomod"
	"github.com/ITR-MOD/field-kit/internal/modmgr"
)

// fomodRow is one navigable row in the wizard overlay: either a selectable
// plugin within a group, or the trailing "Continue" action.
type fomodRow struct {
	groupIdx   int // index into the current step's groups; -1 for the continue row
	pluginIdx  int
	isContinue bool
}

// fomodState holds all state for the FOMOD install wizard overlay.
type fomodState struct {
	session *fomod.Session
	modID   string
	modName string

	// isProfile is true when this wizard run is setting a profile-specific
	// override (saved to the profile) rather than the mod's own default
	// (saved to the mod's meta.json).
	isProfile bool

	// selected tracks the in-progress selection for the step currently being
	// shown: group name -> set of selected plugin names. Reset on each step.
	selected map[string]map[string]bool

	cursor int
	scroll int

	// descScroll is the vertical scroll offset into the wrapped description
	// of the currently selected row. Reset to 0 whenever the cursor moves.
	descScroll int
}

// openFomodWizard loads a mod's ModuleConfig.xml and opens the wizard
// overlay. isProfile=false configures the mod's default selection
// (saved to its own meta.json); isProfile=true configures a profile-specific
// override (saved to the currently selected profile).
func (m *model) openFomodWizard(mod *modmgr.ModMeta, isProfile bool) tea.Cmd {
	cfg, err := modmgr.LoadFomodConfig(mod.ID)
	if err != nil {
		return statusCmd("fomod: "+err.Error(), true)
	}
	session := fomod.NewSession(cfg)
	m.fomod = fomodState{
		session:   session,
		modID:     mod.ID,
		modName:   mod.Name,
		isProfile: isProfile,
	}
	m.resetFomodStepSelection()
	m.overlay = overlayFomod
	return nil
}

// fomodEffectiveEntries returns whichever FOMOD selection currently applies
// to modID in the current scope: the profile override if one exists and
// we're profile-scoped, else the mod's own default.
func (m *model) fomodEffectiveEntries(mod *modmgr.ModMeta, isProfile bool) []modmgr.FomodFileEntry {
	if isProfile && m.selectedProfile != nil {
		if entries := m.selectedProfile.FomodEntriesFor(mod.ID); len(entries) > 0 {
			return entries
		}
	}
	return mod.FomodFiles
}

// resetFomodStepSelection clears the in-progress selection map and seeds it
// with every plugin pre-selected for SelectAll groups in the current step.
func (m *model) resetFomodStepSelection() {
	m.fomod.selected = map[string]map[string]bool{}
	m.fomod.cursor = 0
	m.fomod.scroll = 0

	step, ok := m.fomod.session.CurrentStep()
	if !ok {
		return
	}
	for _, g := range step.OptionalFileGroups.Groups {
		if g.Type != fomod.GroupSelectAll {
			continue
		}
		set := map[string]bool{}
		for _, p := range g.Plugins.Plugins {
			set[p.Name] = true
		}
		m.fomod.selected[g.Name] = set
	}
}

// fomodRows flattens the current step's groups/plugins into navigable rows,
// always ending with a "Continue" row.
func (m *model) fomodRows() ([]fomodRow, *fomod.InstallStep, bool) {
	step, ok := m.fomod.session.CurrentStep()
	if !ok {
		return nil, nil, false
	}
	var rows []fomodRow
	for gi, g := range step.OptionalFileGroups.Groups {
		for pi := range g.Plugins.Plugins {
			rows = append(rows, fomodRow{groupIdx: gi, pluginIdx: pi})
		}
	}
	rows = append(rows, fomodRow{groupIdx: -1, isContinue: true})
	return rows, step, true
}

func (m *model) fomodIsSelected(step *fomod.InstallStep, groupIdx, pluginIdx int) bool {
	g := step.OptionalFileGroups.Groups[groupIdx]
	p := g.Plugins.Plugins[pluginIdx]
	set := m.fomod.selected[g.Name]
	return set != nil && set[p.Name]
}

// toggleFomodPlugin applies group-type-aware selection rules when the user
// activates a plugin row.
func (m *model) toggleFomodPlugin(step *fomod.InstallStep, groupIdx, pluginIdx int) {
	g := &step.OptionalFileGroups.Groups[groupIdx]
	p := g.Plugins.Plugins[pluginIdx]

	if g.Type == fomod.GroupSelectAll {
		return // fixed selection, not user-editable
	}

	if m.fomod.selected[g.Name] == nil {
		m.fomod.selected[g.Name] = map[string]bool{}
	}
	set := m.fomod.selected[g.Name]

	switch g.Type {
	case fomod.GroupSelectExactlyOne:
		for k := range set {
			delete(set, k)
		}
		set[p.Name] = true
	case fomod.GroupSelectAtMostOne:
		already := set[p.Name]
		for k := range set {
			delete(set, k)
		}
		if !already {
			set[p.Name] = true
		}
	default: // SelectAtLeastOne, SelectAny
		set[p.Name] = !set[p.Name]
	}
}

// confirmFomodStep validates and commits the current step's selections,
// advancing the session. Returns (done, error).
func (m *model) confirmFomodStep(step *fomod.InstallStep) (bool, error) {
	selections := make(map[string][]string, len(step.OptionalFileGroups.Groups))
	for _, g := range step.OptionalFileGroups.Groups {
		set := m.fomod.selected[g.Name]
		var names []string
		for _, p := range g.Plugins.Plugins {
			if set[p.Name] {
				names = append(names, p.Name)
			}
		}
		selections[g.Name] = names
	}
	if err := m.fomod.session.SelectStep(selections); err != nil {
		return false, err
	}
	if m.fomod.session.Done() {
		return true, nil
	}
	m.resetFomodStepSelection()
	return false, nil
}

// finishFomodWizard finalizes the session and persists the result to the
// mod's default or the active profile's override, per m.fomod.isProfile.
func (m *model) finishFomodWizard() tea.Cmd {
	files, err := m.fomod.session.Finalize()
	if err != nil {
		return statusCmd("fomod: "+err.Error(), true)
	}

	name := m.fomod.modName
	if m.fomod.isProfile {
		if m.selectedProfile == nil {
			return statusCmd("fomod: no active profile selected", true)
		}
		entries, err := modmgr.ConvertFomodMappings(m.fomod.modID, files)
		if err != nil {
			return statusCmd("fomod: "+err.Error(), true)
		}
		m.selectedProfile.SetFomodSelection(m.fomod.modID, entries)
		if err := m.selectedProfile.Save(); err != nil {
			return statusCmd("fomod: "+err.Error(), true)
		}
		m.overlay = overlayNone
		return statusCmd("FOMOD override saved for \""+name+"\" in profile \""+m.selectedProfile.Name+"\"", false)
	}

	if err := modmgr.SaveFomodSelection(m.fomod.modID, files); err != nil {
		return statusCmd("fomod: "+err.Error(), true)
	}
	m.overlay = overlayNone
	return tea.Batch(
		func() tea.Msg { return msgRefresh{} },
		statusCmd("FOMOD default set for \""+name+"\"", false),
	)
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m model) updateFomodOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	rows, step, ok := m.fomodRows()
	if !ok {
		m.overlay = overlayNone
		return m, nil
	}

	switch key.String() {
	case "esc":
		m.overlay = overlayNone
		return m, statusCmd("FOMOD setup cancelled for \""+m.fomod.modName+"\"", true)

	case "a":
		// Abandon the in-progress wizard and jump to the plain per-source
		// adjust overlay for whichever selection is currently effective.
		mod, err := modmgr.LoadMeta(m.fomod.modID)
		if err != nil {
			return m, statusCmd("fomod: "+err.Error(), true)
		}
		if len(m.fomodEffectiveEntries(mod, m.fomod.isProfile)) == 0 {
			return m, statusCmd("nothing configured yet to adjust", true)
		}
		m.openAdjustOverlay(mod, m.fomod.isProfile)
		return m, nil

	case "d":
		if !m.fomod.isProfile || m.selectedProfile == nil {
			return m, nil
		}
		if len(m.selectedProfile.FomodEntriesFor(m.fomod.modID)) == 0 {
			return m, nil
		}
		m.selectedProfile.SetFomodSelection(m.fomod.modID, nil)
		if err := m.selectedProfile.Save(); err != nil {
			return m, statusCmd("fomod: "+err.Error(), true)
		}
		m.overlay = overlayNone
		return m, statusCmd("profile override cleared for \""+m.fomod.modName+"\" — using mod default", false)

	case "up", "k":
		if m.fomod.cursor > 0 {
			m.fomod.cursor--
			m.fomod.descScroll = 0
		}

	case "down", "j":
		if m.fomod.cursor < len(rows)-1 {
			m.fomod.cursor++
			m.fomod.descScroll = 0
		}

	case "pgup":
		if m.fomod.descScroll > 0 {
			m.fomod.descScroll--
		}

	case "pgdown":
		m.fomod.descScroll++

	case " ", "enter":
		row := rows[m.fomod.cursor]
		if row.isContinue {
			done, err := m.confirmFomodStep(step)
			if err != nil {
				return m, statusCmd("fomod: "+err.Error(), true)
			}
			if !done {
				return m, nil
			}
			return m, m.finishFomodWizard()
		}
		m.toggleFomodPlugin(step, row.groupIdx, row.pluginIdx)
	}

	return m, nil
}

// ─── Render ──────────────────────────────────────────────────────────────────

func (m model) renderFomodOverlay() string {
	rows, step, ok := m.fomodRows()
	if !ok {
		return ""
	}

	w := imin(imax(96, m.width*9/10), m.width-4)
	visH := imin(imax(12, m.height*3/4), imax(4, m.height-6))
	innerW := w - 6

	dividerW := 3
	listW := imax(10, (innerW-dividerW)*2/5)
	descW := imax(10, innerW-dividerW-listW)

	scopeLabel := " [mod default]"
	if m.fomod.isProfile {
		scopeLabel = " [profile override]"
	}

	var sb strings.Builder
	sb.WriteString(sOverlayTitle.Render(" FOMOD Setup: "+m.fomod.modName) + sItemFaint.Render(scopeLabel) + "\n")
	sb.WriteString(sItemFaint.Render(" "+step.Name) + "\n")
	sb.WriteString(sItemFaint.Render(strings.Repeat("─", innerW)) + "\n")

	// The trailing "Continue" row is rendered full-width below the two-column
	// block, not split into list/description cells.
	optRows := rows
	continueIdx := -1
	if len(rows) > 0 && rows[len(rows)-1].isContinue {
		continueIdx = len(rows) - 1
		optRows = rows[:continueIdx]
	}

	scroll := m.fomod.scroll
	if m.fomod.cursor < scroll {
		scroll = m.fomod.cursor
	}
	if m.fomod.cursor >= scroll+visH {
		scroll = m.fomod.cursor - visH + 1
	}
	if maxScroll := imax(0, len(optRows)-visH); scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	var leftLines []string
	lastGroup := -1
	for i, row := range optRows {
		if i < scroll || i >= scroll+visH {
			lastGroup = row.groupIdx
			continue
		}
		selected := i == m.fomod.cursor

		if row.groupIdx != lastGroup {
			lastGroup = row.groupIdx
			g := step.OptionalFileGroups.Groups[row.groupIdx]
			header := truncate(fmt.Sprintf(" %s (%s)", g.Name, g.Type), listW)
			leftLines = append(leftLines, sAccent.Width(listW).Render(header))
		}

		g := step.OptionalFileGroups.Groups[row.groupIdx]
		p := g.Plugins.Plugins[row.pluginIdx]
		mark := "[ ]"
		if m.fomodIsSelected(step, row.groupIdx, row.pluginIdx) {
			mark = "[x]"
		}
		line := fmt.Sprintf("  %s %s", mark, truncate(p.Name, listW-6))

		style := sItem
		if selected {
			style = sItemSelected
		}
		leftLines = append(leftLines, style.Width(listW).Render(line))
	}
	for len(leftLines) < visH {
		leftLines = append(leftLines, sItem.Width(listW).Render(""))
	}
	leftLines = leftLines[:visH]

	// Description pane shows the currently selected row's full description,
	// independent of which list row that corresponds to vertically.
	var descText string
	if m.fomod.cursor < len(optRows) {
		row := optRows[m.fomod.cursor]
		g := step.OptionalFileGroups.Groups[row.groupIdx]
		p := g.Plugins.Plugins[row.pluginIdx]
		descText = strings.TrimSpace(p.Description)
	} else {
		descText = "Press Enter to continue with the selections above."
	}
	if descText == "" {
		descText = "(no description)"
	}
	wrapped := lipgloss.NewStyle().Width(descW).Render(descText)
	descLines := strings.Split(wrapped, "\n")

	descScroll := m.fomod.descScroll
	if maxDescScroll := imax(0, len(descLines)-visH); descScroll > maxDescScroll {
		descScroll = maxDescScroll
	}
	overflow := len(descLines) > visH

	var rightLines []string
	for i := 0; i < visH; i++ {
		idx := descScroll + i
		if idx < len(descLines) {
			line := descLines[idx]
			if overflow && i == visH-1 && idx < len(descLines)-1 {
				line = truncate(line, descW-1) + "…"
			}
			rightLines = append(rightLines, sItemFaint.Width(descW).Render(line))
		} else {
			rightLines = append(rightLines, sItemFaint.Width(descW).Render(""))
		}
	}

	divider := sItemFaint.Render(" │ ")
	for i := 0; i < visH; i++ {
		sb.WriteString(leftLines[i] + divider + rightLines[i] + "\n")
	}

	if continueIdx >= 0 {
		label := "  [ Continue ]"
		style := sItem
		if m.fomod.cursor == continueIdx {
			style = sItemSelected
		}
		sb.WriteString(style.Width(innerW).Render(label) + "\n")
	}

	hint := "↑↓ nav   Space/Enter toggle   Continue to advance   a adjust   Esc cancel"
	if m.fomod.isProfile {
		hint = "↑↓ nav   Space/Enter toggle   Continue to advance   a adjust   d use default   Esc cancel"
	}
	if overflow {
		hint += "   PgUp/PgDn desc"
	}
	sb.WriteString("\n" + sItemFaint.Render(hint))
	return sOverlay.Width(w).Render(sb.String())
}
