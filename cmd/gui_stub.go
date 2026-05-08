//go:build !desktop

package cmd

import "github.com/spf13/cobra"

const isDesktopBuild = false

// runGUI is a stub for non-desktop builds so root.go can reference it unconditionally.
func runGUI(_ *cobra.Command, _ []string) error {
	return runTUI(nil, nil)
}
