package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Merith-TK/itr-field-kit/internal/config"
	"github.com/Merith-TK/itr-field-kit/internal/modmgr"
)

// ─── Colour palette ───────────────────────────────────────────────────────────

var (
	cGreen      = lipgloss.Color("#00FF41")
	cGreenDim   = lipgloss.Color("#006600")
	cGreenFaint = lipgloss.Color("#003300")
	cGreenBg    = lipgloss.Color("#001A00")
	cBlack      = lipgloss.Color("#000000")
	cBright     = lipgloss.Color("#AAFFAA")
	cRed        = lipgloss.Color("#FF5555")
	cYellow     = lipgloss.Color("#AAFF00")
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	sTitle = lipgloss.NewStyle().
		Bold(true).Foreground(cGreen).Background(cBlack).
		Padding(0, 1)

	sTabInactive = lipgloss.NewStyle().
		Foreground(cGreenDim).Background(cBlack).
		Padding(0, 2)

	sTabActive = lipgloss.NewStyle().
		Bold(true).Foreground(cBlack).Background(cGreen).
		Padding(0, 2)

	sTabNum = lipgloss.NewStyle().
		Foreground(cGreenFaint).Background(cBlack)

	sItem = lipgloss.NewStyle().
		Foreground(cGreen).Background(cBlack)

	sItemSelected = lipgloss.NewStyle().
		Bold(true).Foreground(cBlack).Background(cGreen)

	sItemActive = lipgloss.NewStyle().
		Foreground(cYellow).Background(cBlack)

	sItemActiveSelected = lipgloss.NewStyle().
		Bold(true).Foreground(cBlack).Background(cYellow)

	sItemFaint = lipgloss.NewStyle().
		Foreground(cGreenDim).Background(cBlack)

	sAccent = lipgloss.NewStyle().
		Bold(true).Foreground(cBright)

	sSectionTitle = lipgloss.NewStyle().
		Bold(true).Foreground(cGreenDim).Background(cBlack).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(cGreenFaint).
		BorderBottom(true)

	sPaneFocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cGreen)

	sPaneBlur = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cGreenFaint)

	sStatus = lipgloss.NewStyle().
		Foreground(cGreen).Background(cGreenBg).
		Padding(0, 1)

	sStatusError = lipgloss.NewStyle().
		Bold(true).Foreground(cBlack).Background(cRed).
		Padding(0, 1)

	sHelp = lipgloss.NewStyle().
		Foreground(cGreenDim).Background(cBlack).
		Padding(0, 0)

	sHelpKey = lipgloss.NewStyle().
		Bold(true).Foreground(cGreen).Background(cBlack)

	sOverlay = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cGreen).
		Background(cBlack).
		Foreground(cGreen).
		Padding(1, 3)

	sOverlayTitle = lipgloss.NewStyle().
		Bold(true).Foreground(cBright)

	sBtnSelected = lipgloss.NewStyle().
		Bold(true).Foreground(cBlack).Background(cGreen).
		Padding(0, 2)

	sBtnUnselected = lipgloss.NewStyle().
		Foreground(cGreenDim).Background(cGreenFaint).
		Padding(0, 2)
)

// ─── Main View ────────────────────────────────────────────────────────────────

func (m model) View() string {
	// Layout: header(2 rows) + content(variable) + status(1) + help(1)
	const headerRows = 2
	const footRows = 2
	contentH := imax(1, m.height-headerRows-footRows)

	header := m.renderHeader()
	content := m.renderContent(contentH)
	foot := m.renderFoot()

	// Ensure content fills exactly contentH rows.
	contentLines := strings.Split(content, "\n")
	for len(contentLines) < contentH {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > contentH {
		contentLines = contentLines[:contentH]
	}
	content = strings.Join(contentLines, "\n")

	page := header + "\n" + content + "\n" + foot

	if m.overlay != overlayNone {
		return m.renderOverlayOn(page)
	}
	return page
}

// ─── Header ───────────────────────────────────────────────────────────────────

func (m model) renderHeader() string {
	title := sTitle.Render("ITR FIELD KIT")

	var tabs []string
	for i, name := range tabNames {
		num := sTabNum.Render(fmt.Sprintf("[%d]", i+1))
		if i == m.activeTab {
			tabs = append(tabs, num+sTabActive.Render(name))
		} else {
			tabs = append(tabs, num+sTabInactive.Render(name))
		}
	}
	row := title + "  " + strings.Join(tabs, "")
	// Fill remaining width with black background.
	w := lipgloss.Width(row)
	if w < m.width {
		row += lipgloss.NewStyle().Background(cBlack).Render(strings.Repeat(" ", m.width-w))
	}

	// Divider line.
	divider := lipgloss.NewStyle().Foreground(cGreenFaint).Render(strings.Repeat("─", m.width))
	return row + "\n" + divider
}

// ─── Content ─────────────────────────────────────────────────────────────────

func (m model) renderContent(h int) string {
	switch m.activeTab {
	case tabGames:
		return m.renderGamesTab(h)
	case tabMods:
		return m.renderModsTab(h)
	case tabProfiles:
		return m.renderProfilesTab(h)
	case tabDeploy:
		return m.renderDeployTab(h)
	}
	return ""
}

// ─── Games tab ───────────────────────────────────────────────────────────────

func (m model) renderGamesTab(h int) string {
	var sb strings.Builder
	sb.WriteString(sSectionTitle.Width(m.width-2).Render(" Registered Game Installations") + "\n")

	if len(m.games) == 0 {
		sb.WriteString("\n")
		sb.WriteString(sItemFaint.Render("  No games registered.") + "\n")
		sb.WriteString(sItemFaint.Render("  Press [a] to add a game installation path."))
		return sb.String()
	}

	activeCfgID := config.Get().ActiveGame
	cur := m.cursors[tabGames]
	offset := scrollOffset(cur, m.offsets[tabGames], h-2)
	m.offsets[tabGames] = offset // note: doesn't persist because model is value type, but good enough

	for i, g := range m.games {
		if i < offset || i >= offset+h-2 {
			continue
		}
		isActive := g.ID == activeCfgID
		isCursor := i == cur

		marker := "  "
		if isActive {
			marker = "▶ "
		}
		name := truncate(g.Name, 22)
		id := truncate(g.ID, 10)
		path := truncate(g.Path, m.width-42)
		line := fmt.Sprintf("%s%-22s  %-10s  %s", marker, name, id, path)

		switch {
		case isCursor && isActive:
			sb.WriteString(sItemActiveSelected.Width(m.width).Render(line))
		case isActive:
			sb.WriteString(sItemActive.Width(m.width).Render(line))
		case isCursor:
			sb.WriteString(sItemSelected.Width(m.width).Render(line))
		default:
			sb.WriteString(sItem.Width(m.width).Render(line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ─── Mods tab ────────────────────────────────────────────────────────────────

func (m model) renderModsTab(h int) string {
	var sb strings.Builder
	sb.WriteString(sSectionTitle.Width(m.width-2).Render(" Imported Mods") + "\n")

	if len(m.mods) == 0 {
		sb.WriteString("\n")
		sb.WriteString(sItemFaint.Render("  No mods imported.") + "\n")
		sb.WriteString(sItemFaint.Render("  Press [i] to import a mod .zip file."))
		return sb.String()
	}

	cur := m.cursors[tabMods]
	offset := scrollOffset(cur, m.offsets[tabMods], h-2)
	m.offsets[tabMods] = offset

	for i, mod := range m.mods {
		if i < offset || i >= offset+h-2 {
			continue
		}
		isCursor := i == cur

		typeStr := modTypeSummary(mod)
		name := truncate(mod.Name, m.width-22)
		line := fmt.Sprintf("  %-18s  %s", typeStr, name)

		if isCursor {
			sb.WriteString(sItemSelected.Width(m.width).Render(line))
		} else {
			sb.WriteString(sItem.Width(m.width).Render(line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ─── Profiles tab ────────────────────────────────────────────────────────────

func (m model) renderProfilesTab(h int) string {
	// Split horizontally: left = profile list, right = mod toggle list.
	leftW := 26
	rightW := m.width - leftW - 5 // 5 = borders + gap

	leftContent := m.renderProfilesList(h-2, leftW)
	rightContent := m.renderProfileMods(h-2, rightW)

	var leftStyle, rightStyle lipgloss.Style
	if !m.profilesRightFocus {
		leftStyle = sPaneFocused
		rightStyle = sPaneBlur
	} else {
		leftStyle = sPaneBlur
		rightStyle = sPaneFocused
	}

	left := leftStyle.Width(leftW).Height(h - 2).Render(leftContent)
	right := rightStyle.Width(rightW).Height(h - 2).Render(rightContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m model) renderProfilesList(h, w int) string {
	if len(m.profiles) == 0 {
		return sItemFaint.Render("No profiles.\n[n] to create one.")
	}
	activeProfile := config.Get().ActiveProfile
	cur := m.cursors[tabProfiles]
	offset := scrollOffset(cur, m.offsets[tabProfiles], h-1)
	m.offsets[tabProfiles] = offset

	var sb strings.Builder
	for i, name := range m.profiles {
		if i < offset || i >= offset+h-1 {
			continue
		}
		isActive := name == activeProfile
		isCursor := i == cur

		marker := "  "
		if isActive {
			marker = "▶ "
		}
		line := marker + truncate(name, w-4)

		switch {
		case isCursor && isActive:
			sb.WriteString(sItemActiveSelected.Width(w).Render(line))
		case isActive:
			sb.WriteString(sItemActive.Width(w).Render(line))
		case isCursor:
			sb.WriteString(sItemSelected.Width(w).Render(line))
		default:
			sb.WriteString(sItem.Width(w).Render(line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m model) renderProfileMods(h, w int) string {
	if m.selectedProfile == nil {
		return sItemFaint.Render("Select a profile on the left.")
	}
	var sb strings.Builder
	sb.WriteString(sAccent.Render(truncate(m.selectedProfile.Name, w-2)) + "\n")

	if len(m.mods) == 0 {
		sb.WriteString(sItemFaint.Render("\nNo mods imported yet.\nGo to MODS tab to import."))
		return sb.String()
	}

	offset := scrollOffset(m.profileModsCursor, m.profileModsOffset, h-2)
	m.profileModsOffset = offset

	for i, mod := range m.mods {
		if i < offset || i >= offset+h-2 {
			continue
		}
		inProfile := m.selectedProfile.HasMod(mod.ID)
		isCursor := m.profilesRightFocus && i == m.profileModsCursor

		check := "[ ]"
		if inProfile {
			check = "[✓]"
		}
		line := fmt.Sprintf(" %s %s", check, truncate(mod.Name, w-7))

		switch {
		case isCursor && inProfile:
			sb.WriteString(sItemActiveSelected.Width(w).Render(line))
		case isCursor:
			sb.WriteString(sItemSelected.Width(w).Render(line))
		case inProfile:
			sb.WriteString(sItem.Width(w).Render(line))
		default:
			sb.WriteString(sItemFaint.Width(w).Render(line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ─── Deploy tab ───────────────────────────────────────────────────────────────

func (m model) renderDeployTab(h int) string {
	var sb strings.Builder
	sb.WriteString(sSectionTitle.Width(m.width-2).Render(" Deployment") + "\n\n")

	cfg := config.Get()

	// Active game.
	game := config.ActiveGameInstall()
	if game == nil {
		sb.WriteString(sItemFaint.Render("  No active game.") + "\n")
		sb.WriteString(sItemFaint.Render("  Go to GAMES tab [1] and register your game folder.\n\n"))
	} else {
		sb.WriteString(sAccent.Render("  Game:    ") + sItem.Render(game.Name) + "\n")
		sb.WriteString(sAccent.Render("  Path:    ") + sItemFaint.Render(truncate(game.Path, m.width-14)) + "\n")
	}

	// Active profile.
	sb.WriteString(sAccent.Render("  Profile: ") + sItem.Render(cfg.ActiveProfile) + "\n\n")

	// Deployment status.
	state := m.deployState
	if state == nil || len(state.Deployed) == 0 {
		sb.WriteString(sItemFaint.Render("  ● Not deployed") + "\n")
	} else {
		sb.WriteString(sItem.Render(fmt.Sprintf("  ● Active — %d symlinks", len(state.Deployed))) + "\n")
		maxShow := imax(0, h-12)
		for i, df := range state.Deployed {
			if i >= maxShow {
				sb.WriteString(sItemFaint.Render(fmt.Sprintf("    … +%d more", len(state.Deployed)-i)) + "\n")
				break
			}
			tag := ""
			if df.IsCustom {
				tag = " [custom]"
			}
			sb.WriteString(sItemFaint.Render(fmt.Sprintf("    [%-12s] %s%s",
				truncate(df.ModID, 12),
				truncate(df.GamePath, m.width-34),
				tag)) + "\n")
		}
	}
	return sb.String()
}

// ─── Footer ───────────────────────────────────────────────────────────────────

func (m model) renderFoot() string {
	// Status line.
	var statusLine string
	if m.statusMsg != "" {
		if m.statusError {
			statusLine = sStatusError.Width(m.width).Render("✗ " + m.statusMsg)
		} else {
			statusLine = sStatus.Width(m.width).Render("✓ " + m.statusMsg)
		}
	} else {
		statusLine = sStatus.Width(m.width).Render("Ready")
	}

	helpLine := m.renderHelp()
	return statusLine + "\n" + helpLine
}

func (m model) renderHelp() string {
	key := func(k, desc string) string {
		return sHelpKey.Render(k) + sHelp.Render(":"+desc)
	}

	parts := []string{key("1-4", "tabs"), key("Tab", "next tab"), key("↑↓", "nav")}

	switch m.activeTab {
	case tabGames:
		parts = append(parts, key("a", "add"), key("↵", "set active"), key("d", "remove"))
	case tabMods:
		parts = append(parts, key("i", "import"), key("↵", "info"), key("d", "remove"))
	case tabProfiles:
		if m.profilesRightFocus {
			parts = append(parts, key("Space", "toggle mod"), key("←", "profiles pane"))
		} else {
			parts = append(parts, key("n", "new"), key("↵", "activate"), key("→", "mods"), key("d", "delete"))
		}
	case tabDeploy:
		parts = append(parts, key("d", "deploy"), key("u", "undeploy"), key("r", "refresh"))
	}
	parts = append(parts, key("q", "quit"))

	line := "  " + strings.Join(parts, "  ")
	w := lipgloss.Width(line)
	if w < m.width {
		line += sHelp.Render(strings.Repeat(" ", m.width-w))
	}
	return line
}

// ─── Overlays ────────────────────────────────────────────────────────────────

func (m model) renderOverlayOn(base string) string {
	var content string
	switch m.overlay {
	case overlayInput:
		content = m.renderInputOverlay()
	case overlayConfirm:
		content = m.renderConfirmOverlay()
	case overlayInfo:
		content = m.renderInfoOverlay()
	}
	if content == "" {
		return base
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceForeground(cGreenFaint),
		lipgloss.WithWhitespaceBackground(cBlack))
}

func (m model) renderInputOverlay() string {
	w := imax(52, m.width/2)
	content := sOverlayTitle.Render(m.inputPrompt) + "\n\n" +
		m.textInput.View() + "\n\n" +
		sItemFaint.Render("Enter  confirm   Esc  cancel")
	return sOverlay.Width(w).Render(content)
}

func (m model) renderConfirmOverlay() string {
	yesStyle, noStyle := sBtnUnselected, sBtnUnselected
	if m.confirmSel == 0 {
		yesStyle = sBtnSelected
	} else {
		noStyle = sBtnSelected
	}
	content := sOverlayTitle.Render(m.confirmPrompt) + "\n\n" +
		yesStyle.Render(" YES ") + "    " + noStyle.Render("  NO  ") + "\n\n" +
		sItemFaint.Render("←/→ choose   y/n shortcut   Enter confirm")
	return sOverlay.Width(50).Render(content)
}

func (m model) renderInfoOverlay() string {
	w := imax(62, m.width*3/4)
	visH := imax(5, m.height/2)

	end := m.infoScroll + visH
	if end > len(m.infoLines) {
		end = len(m.infoLines)
	}
	start := m.infoScroll
	if start > len(m.infoLines) {
		start = len(m.infoLines)
	}

	body := strings.Join(m.infoLines[start:end], "\n")
	scrollInfo := ""
	if len(m.infoLines) > visH {
		scrollInfo = "\n" + sItemFaint.Render(fmt.Sprintf("(%d/%d)  ↑↓ scroll", m.infoScroll+1, len(m.infoLines)))
	}

	content := sOverlayTitle.Render(m.infoTitle) + "\n" +
		sItemFaint.Render(strings.Repeat("─", w-6)) + "\n" +
		body + scrollInfo + "\n\n" +
		sItemFaint.Render("↑↓ scroll   Enter/Esc close")

	return sOverlay.Width(w).Render(content)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func scrollOffset(cursor, prevOffset, height int) int {
	if height <= 0 {
		return 0
	}
	if cursor < prevOffset {
		return cursor
	}
	if cursor >= prevOffset+height {
		return cursor - height + 1
	}
	return prevOffset
}

func modTypeSummary(mod *modmgr.ModMeta) string {
	parts := make([]string, len(mod.Types))
	for i, t := range mod.Types {
		parts[i] = string(t)
	}
	return strings.Join(parts, "+")
}
