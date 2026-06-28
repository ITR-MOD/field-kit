package fomod

import (
	"fmt"
	"sort"
	"strings"
)

// FileMapping is a resolved source -> destination pair produced by Finalize.
type FileMapping struct {
	Source   string
	Dest     string
	Priority int
}

// Session drives a single FOMOD install wizard: walk visible steps in order,
// record the user's group selections via SelectStep, then call Finalize once
// Done reports true to get the resolved file list.
type Session struct {
	cfg             *Config
	flags           map[string]string
	cursor          int
	selectedPlugins []*Plugin
}

// NewSession creates a wizard session for cfg, starting at the first step.
func NewSession(cfg *Config) *Session {
	return &Session{cfg: cfg, flags: map[string]string{}}
}

// CurrentStep returns the next step whose visibility condition is satisfied
// by the flags accumulated so far, or ok=false if no steps remain.
func (s *Session) CurrentStep() (step *InstallStep, ok bool) {
	steps := s.cfg.InstallSteps.Steps
	for s.cursor < len(steps) {
		st := &steps[s.cursor]
		if evalDeps(st.Visible, s.flags) {
			return st, true
		}
		s.cursor++
	}
	return nil, false
}

// Done reports whether every visible step has been completed.
func (s *Session) Done() bool {
	_, ok := s.CurrentStep()
	return !ok
}

// SelectStep records the user's choice for each group in the current step.
// selections maps group name -> chosen plugin names. Groups with no entry
// are treated as an empty selection (only valid for SelectAny/SelectAtMostOne).
func (s *Session) SelectStep(selections map[string][]string) error {
	step, ok := s.CurrentStep()
	if !ok {
		return fmt.Errorf("fomod: no current step")
	}

	var matched []*Plugin
	for gi := range step.OptionalFileGroups.Groups {
		group := &step.OptionalFileGroups.Groups[gi]
		chosen := selections[group.Name]
		if err := validateGroupSelection(group, chosen); err != nil {
			return fmt.Errorf("fomod: group %q: %w", group.Name, err)
		}
		for _, name := range chosen {
			p := findPlugin(group, name)
			if p == nil {
				return fmt.Errorf("fomod: group %q: unknown plugin %q", group.Name, name)
			}
			matched = append(matched, p)
		}
	}

	for _, p := range matched {
		s.selectedPlugins = append(s.selectedPlugins, p)
		for _, f := range p.ConditionFlags.Flags {
			s.flags[f.Name] = f.Value
		}
	}
	s.cursor++
	return nil
}

// Finalize resolves the final file list once the wizard is complete:
// required files, then each selected plugin's files, then any
// conditionalFileInstalls patterns whose dependencies are now satisfied.
// Conflicting destinations are resolved by highest priority (later entries
// win ties), mirroring FOMOD's own priority semantics.
func (s *Session) Finalize() ([]FileMapping, error) {
	if !s.Done() {
		return nil, fmt.Errorf("fomod: wizard is not complete")
	}

	resolved := map[string]FileMapping{}
	addFileList(resolved, s.cfg.RequiredInstallFiles)
	for _, p := range s.selectedPlugins {
		addFileList(resolved, p.Files)
	}
	for i := range s.cfg.ConditionalFileInstalls.Patterns {
		pat := &s.cfg.ConditionalFileInstalls.Patterns[i]
		if evalDeps(&pat.Dependencies, s.flags) {
			addFileList(resolved, pat.Files)
		}
	}

	mappings := make([]FileMapping, 0, len(resolved))
	for _, m := range resolved {
		mappings = append(mappings, m)
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Dest < mappings[j].Dest })
	return mappings, nil
}

func validateGroupSelection(g *Group, chosen []string) error {
	n := len(chosen)
	switch g.Type {
	case GroupSelectExactlyOne:
		if n != 1 {
			return fmt.Errorf("requires exactly one selection, got %d", n)
		}
	case GroupSelectAtMostOne:
		if n > 1 {
			return fmt.Errorf("requires at most one selection, got %d", n)
		}
	case GroupSelectAtLeastOne:
		if n < 1 {
			return fmt.Errorf("requires at least one selection, got %d", n)
		}
	case GroupSelectAll:
		if n != len(g.Plugins.Plugins) {
			return fmt.Errorf("requires all %d plugin(s) selected, got %d", len(g.Plugins.Plugins), n)
		}
	case GroupSelectAny:
		// No constraint.
	}
	return nil
}

func findPlugin(g *Group, name string) *Plugin {
	for i := range g.Plugins.Plugins {
		if g.Plugins.Plugins[i].Name == name {
			return &g.Plugins.Plugins[i]
		}
	}
	return nil
}

// evalDeps evaluates a <dependencies>/<visible> condition tree against the
// current flag state. A nil tree is vacuously true.
//
// fileDependency and gameDependency checks against the real game install are
// not implemented in v1 and are treated as satisfied.
func evalDeps(deps *Dependencies, flags map[string]string) bool {
	if deps == nil {
		return true
	}

	var results []bool
	for _, fd := range deps.FlagDependencies {
		results = append(results, flags[fd.Flag] == fd.Value)
	}
	for range deps.FileDependencies {
		results = append(results, true)
	}
	for range deps.GameDependencies {
		results = append(results, true)
	}
	for i := range deps.CompositeDependencies {
		results = append(results, evalDeps(&deps.CompositeDependencies[i], flags))
	}

	if len(results) == 0 {
		return true
	}

	if strings.EqualFold(deps.Operator, "Or") {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}

	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

// addFileList merges a <files> element's entries into the resolved map,
// keyed by destination (defaulting to source when destination is empty).
// On conflict, the entry with the higher priority wins.
func addFileList(into map[string]FileMapping, list FileList) {
	add := func(item FileItem) {
		dest := item.Destination
		if dest == "" {
			dest = item.Source
		}
		if existing, ok := into[dest]; ok && existing.Priority > item.Priority {
			return
		}
		into[dest] = FileMapping{Source: item.Source, Dest: dest, Priority: item.Priority}
	}
	for _, f := range list.Files {
		add(f)
	}
	for _, f := range list.Folders {
		add(f)
	}
}
