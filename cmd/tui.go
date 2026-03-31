package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ITR-MOD/field-kit/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive terminal UI (default when run with no subcommand)",
	RunE:  runTUI,
}

func runTUI(_ *cobra.Command, _ []string) error {
	p := tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "TUI error:", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	// Make TUI the default when the root command is invoked without a subcommand.
	rootCmd.RunE = runTUI
}
