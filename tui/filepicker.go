package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── File filters ─────────────────────────────────────────────────────────────

// fpFilter controls which files are visible in the picker.
type fpFilter struct {
	// Title shown in the picker header (e.g. " Import Mod").
	Title string
	// Match returns true if a file (not directory) should be listed.
	// Ignored when DirPick is true (no files are shown).
	Match func(name string) bool
	// DirPick switches the picker to directory-selection mode:
	// Enter on a directory calls onPick with that directory path instead of
	// navigating into it. Tab still navigates into the directory.
	// Files are hidden entirely in this mode.
	DirPick bool
}

var (
	// FpFilterArchives shows zip / tar / 7z / rar archives.
	FpFilterArchives = fpFilter{
		Title: " Import Mod",
		Match: func(name string) bool {
			ext := strings.ToLower(filepath.Ext(name))
			switch ext {
			case ".zip", ".7z", ".rar":
				return true
			}
			// tar, tar.gz, tar.bz2, tar.xz
			lower := strings.ToLower(name)
			return strings.HasSuffix(lower, ".tar") ||
				strings.HasSuffix(lower, ".tar.gz") ||
				strings.HasSuffix(lower, ".tar.bz2") ||
				strings.HasSuffix(lower, ".tar.xz") ||
				strings.HasSuffix(lower, ".tgz")
		},
	}

	// FpFilterGameExe shows IntoTheRadius*.exe executables.
	FpFilterGameExe = fpFilter{
		Title: " Select Game Executable",
		Match: func(name string) bool {
			lower := strings.ToLower(name)
			return strings.HasPrefix(lower, "intotheradius") &&
				strings.HasSuffix(lower, ".exe")
		},
	}

	// FpFilterProfile shows *.fieldkit.json profile export files.
	FpFilterProfile = fpFilter{
		Title: " Import Profile",
		Match: func(name string) bool {
			return strings.HasSuffix(strings.ToLower(name), ".fieldkit.json")
		},
	}

	// FpFilterSaveDir is a directory-picker used for choosing a save location.
	// Enter selects the highlighted directory; Tab navigates into it.
	FpFilterSaveDir = fpFilter{
		Title:   " Choose Save Directory",
		DirPick: true,
		Match:   func(string) bool { return false }, // no files shown
	}
)

// ─── Types ───────────────────────────────────────────────────────────────────

// fpEntry is one row in the file picker list.
type fpEntry struct {
	label string // display name (with trailing / for dirs)
	path  string // absolute path
	isDir bool
}

// filePickerState holds all state for the file picker overlay.
// The picker always operates in "path mode": a live os.ReadDir of the current
// directory filtered by the name portion of the query, showing dirs + files
// matching the active fpFilter.  Backspace on an empty name filter navigates
// up one directory.
type filePickerState struct {
	dir     string    // absolute path of the directory being listed
	filter  string    // name filter (chars typed after last /)
	entries []fpEntry // current visible list
	cursor  int
	scroll  int
	onPick  func(string) tea.Cmd // called with absolute path of chosen file
	fp      fpFilter             // controls which files are shown
}

// ─── State helpers ────────────────────────────────────────────────────────────

// fpDefaultDir returns the best starting directory for the picker.
func fpDefaultDir() string {
	home, _ := os.UserHomeDir()
	if dl := filepath.Join(home, "Downloads"); fpDirExists(dl) {
		return dl
	}
	return home
}

func fpDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fpBuildEntries re-reads dir and rebuilds the entry list, applying filter.
func fpBuildEntries(s *filePickerState) {
	s.entries = s.entries[:0]

	// Always offer ".." except at filesystem root.
	parent := filepath.Dir(s.dir)
	if parent != s.dir {
		s.entries = append(s.entries, fpEntry{label: "../", path: parent, isDir: true})
	}

	infos, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}

	nameFilter := strings.ToLower(s.filter)

	// Dirs first, then files — mirrors residual picker order.
	for _, pass := range []bool{true, false} {
		for _, e := range infos {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if e.IsDir() != pass {
				continue
			}
			// Files: hidden in DirPick mode, otherwise filtered by Match.
			if !e.IsDir() && (s.fp.DirPick || !s.fp.Match(name)) {
				continue
			}
			label := name
			if e.IsDir() {
				label = name + "/"
			}
			if nameFilter != "" && !strings.Contains(strings.ToLower(label), nameFilter) {
				continue
			}
			s.entries = append(s.entries, fpEntry{
				label: label,
				path:  filepath.Join(s.dir, name),
				isDir: e.IsDir(),
			})
		}
	}
}

// ─── Model integration ───────────────────────────────────────────────────────

// openFilePicker initialises and opens the file picker overlay with the given
// filter and callback.
func (m *model) openFilePicker(filter fpFilter, onPick func(string) tea.Cmd) {
	dir := fpDefaultDir()
	s := filePickerState{dir: dir, onPick: onPick, fp: filter}
	fpBuildEntries(&s)
	m.filePicker = s
	m.overlay = overlayFilePicker
}

// ─── Key handling ─────────────────────────────────────────────────────────────

func (m model) updateFilePickerOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	p := &m.filePicker
	listH := m.fpListHeight()

	switch key.String() {
	case "esc":
		m.overlay = overlayNone

	case "enter":
		if len(p.entries) == 0 {
			break
		}
		sel := p.entries[p.cursor]
		if sel.isDir {
			p.dir = sel.path
			p.filter = ""
			fpBuildEntries(p)
			p.cursor, p.scroll = 0, 0
		} else {
			// File selected — close picker and trigger callback.
			path := sel.path
			onPick := p.onPick
			m.overlay = overlayNone
			if onPick != nil {
				return m, onPick(path)
			}
		}

	case "e":
		if p.fp.DirPick {
			// Confirm the current directory as the save destination.
			onPick := p.onPick
			dir := p.dir
			m.overlay = overlayNone
			if onPick != nil {
				return m, onPick(dir)
			}
		}

	case "tab":
		// Tab on a directory completes (navigates in) without requiring Enter.
		if len(p.entries) > 0 && p.entries[p.cursor].isDir {
			p.dir = p.entries[p.cursor].path
			p.filter = ""
			fpBuildEntries(p)
			p.cursor, p.scroll = 0, 0
		}

	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
			if p.cursor < p.scroll {
				p.scroll = p.cursor
			}
		}

	case "down", "j":
		if p.cursor < len(p.entries)-1 {
			p.cursor++
			if p.cursor >= p.scroll+listH {
				p.scroll = p.cursor - listH + 1
			}
		}

	case "pgup":
		p.cursor -= listH
		if p.cursor < 0 {
			p.cursor = 0
		}
		if p.cursor < p.scroll {
			p.scroll = p.cursor
		}

	case "pgdown":
		p.cursor += listH
		if p.cursor >= len(p.entries) {
			p.cursor = len(p.entries) - 1
		}
		if p.cursor < 0 {
			p.cursor = 0
		}
		if p.cursor >= p.scroll+listH {
			p.scroll = p.cursor - listH + 1
		}

	case "backspace":
		if p.filter == "" {
			// Navigate up one directory.
			parent := filepath.Dir(p.dir)
			if parent != p.dir {
				p.dir = parent
				fpBuildEntries(p)
				p.cursor, p.scroll = 0, 0
			}
		} else {
			runes := []rune(p.filter)
			p.filter = string(runes[:len(runes)-1])
			fpBuildEntries(p)
			p.cursor, p.scroll = 0, 0
		}

	default:
		// Printable characters narrow the filter.
		if key.Type == tea.KeyRunes {
			p.filter += string(key.Runes)
			fpBuildEntries(p)
			p.cursor, p.scroll = 0, 0
		}
	}

	return m, nil
}

// ─── Dimensions ──────────────────────────────────────────────────────────────

func (m *model) fpBoxWidth() int {
	w := m.width - 8
	if w > 84 {
		w = 84
	}
	if w < 46 {
		w = 46
	}
	return w
}

func (m *model) fpBoxHeight() int {
	h := m.height - 6
	if h > 30 {
		h = 30
	}
	if h < 10 {
		h = 10
	}
	return h
}

// fpListHeight: box interior minus title(1) + input(1) + divider(1) + hint(1) + borders(2)
func (m *model) fpListHeight() int {
	return m.fpBoxHeight() - 6
}

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	fpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cGreen).
		BorderBackground(cBlack).
		Background(cBlack)

	fpTitle = lipgloss.NewStyle().
		Bold(true).Foreground(cBright).Background(cBlack)

	fpInput = lipgloss.NewStyle().
		Foreground(cGreen).Background(cGreenBg)

	fpDiv = lipgloss.NewStyle().
		Foreground(cGreenFaint).Background(cBlack)

	fpDir = lipgloss.NewStyle().
		Foreground(cGreen).Background(cBlack)

	fpFile = lipgloss.NewStyle().
		Foreground(cGreenDim).Background(cBlack)

	fpDirSel = lipgloss.NewStyle().
			Bold(true).Foreground(cBlack).Background(cGreen)

	fpFileSel = lipgloss.NewStyle().
			Bold(true).Foreground(cBlack).Background(cYellow)

	fpEmpty = lipgloss.NewStyle().
		Foreground(cGreenFaint).Background(cBlack)

	fpCount = lipgloss.NewStyle().
		Foreground(cGreenFaint).Background(cBlack)
)

// ─── Rendering ───────────────────────────────────────────────────────────────

func (m model) renderFilePickerOverlay() string {
	p := &m.filePicker
	bw := m.fpBoxWidth()
	listH := m.fpListHeight()
	innerW := bw - 2 // subtract border chars

	var sb strings.Builder

	// ── title ──────────────────────────────────────────────────
	titleText := fpTitle.Render(p.fp.Title)
	countText := ""
	if len(p.entries) > 0 {
		countText = fpCount.Render(fmt.Sprintf(" %d items", len(p.entries)))
	}
	titleLine := titleText + countText
	titlePad := innerW - lipgloss.Width(titleLine)
	if titlePad > 0 {
		titleLine += fpEmpty.Render(strings.Repeat(" ", titlePad))
	}
	sb.WriteString(titleLine + "\n")

	// ── input line: current dir + filter ───────────────────────
	dir := p.dir
	// Show the directory relative to home if possible.
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
			dir = "~/" + rel
		}
	}
	inputContent := " " + dir + "/"
	if p.filter != "" {
		inputContent += p.filter
	}
	inputContent += "█"
	inputPad := innerW - lipgloss.Width(inputContent)
	if inputPad < 0 {
		inputPad = 0
	}
	sb.WriteString(fpInput.Render(inputContent+strings.Repeat(" ", inputPad)) + "\n")

	// ── divider ────────────────────────────────────────────────
	sb.WriteString(fpDiv.Render(strings.Repeat("─", innerW)) + "\n")

	// ── entry list ─────────────────────────────────────────────
	for row := 0; row < listH; row++ {
		idx := p.scroll + row
		if idx >= len(p.entries) {
			sb.WriteString(fpEmpty.Render(strings.Repeat(" ", innerW)) + "\n")
			continue
		}
		e := p.entries[idx]
		selected := idx == p.cursor

		prefix := "   "
		if selected {
			prefix = " ▶ "
		}

		var style lipgloss.Style
		switch {
		case selected && e.isDir:
			style = fpDirSel
		case selected:
			style = fpFileSel
		case e.isDir:
			style = fpDir
		default:
			style = fpFile
		}

		line := prefix + e.label
		lw := lipgloss.Width(line)
		if lw > innerW {
			runes := []rune(line)
			line = "…" + string(runes[len(runes)-(innerW-1):])
		} else if lw < innerW {
			line += strings.Repeat(" ", innerW-lw)
		}
		sb.WriteString(style.Render(line) + "\n")
	}

	// ── hint line ──────────────────────────────────────────────
	hintText := "↑↓ nav  Enter open  Tab dir  Bksp up  type:filter  Esc cancel"
	if p.fp.DirPick {
		hintText = "↑↓ nav  Enter browse  Tab dir  Bksp up  type:filter  e:save here  Esc cancel"
	}
	scrollSuffix := ""
	if len(p.entries) > listH && len(p.entries) > 1 {
		denom := len(p.entries) - listH + 1
		if denom < 1 {
			denom = 1
		}
		pct := p.scroll * 100 / denom
		scrollSuffix = fpCount.Render(fmt.Sprintf(" %3d%%", pct))
	}
	hint := fpDiv.Render(hintText)
	gap := innerW - lipgloss.Width(hint) - lipgloss.Width(scrollSuffix)
	if gap < 0 {
		gap = 0
	}
	sb.WriteString(hint + fpDiv.Render(strings.Repeat(" ", gap)) + scrollSuffix)

	// ── wrap in border box ─────────────────────────────────────
	box := fpBox.Width(bw).Height(m.fpBoxHeight()).Render(sb.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceBackground(cBlack))
}
