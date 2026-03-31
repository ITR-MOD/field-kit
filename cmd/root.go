package cmd

import (
	"fmt"
	"os"

	"github.com/Merith-TK/itr-field-kit/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "itr",
	Short: "ITR Field Kit – mod manager for Into the Radius 2",
	Long: `ITR Field Kit is a symlink-based mod manager for Into the Radius 2.
Mods are stored in a local cache and symlinked into the game directory on deploy.`,
	SilenceUsage: true,
}

// Execute is the main entry-point called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(gameCmd)
	rootCmd.AddCommand(modCmd)
	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(undeployCmd)
	rootCmd.AddCommand(statusCmd)
}

func initConfig() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// requireActiveGame returns the active game install or exits with an error message.
func requireActiveGame() *config.GameInstall {
	g := config.ActiveGameInstall()
	if g == nil {
		fmt.Fprintln(os.Stderr, "no active game set – run: itr game add <path>")
		os.Exit(1)
	}
	return g
}
