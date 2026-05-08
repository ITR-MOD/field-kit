package cmd

import (
	"fmt"

	"github.com/ITR-MOD/field-kit/internal/config"
	"github.com/ITR-MOD/field-kit/internal/modmgr"
	"github.com/spf13/cobra"
)

// requireActiveGameID returns the active game ID or exits with an error.
func requireActiveGameID() (string, error) {
	gameID := config.Get().ActiveGame
	if gameID == "" {
		return "", fmt.Errorf("no active game â€“ run 'itr game use <id>' first")
	}
	return gameID, nil
}

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage mod profiles",
}

var profileNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new profile for the active game",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gameID, err := requireActiveGameID()
		if err != nil {
			return err
		}
		name := args[0]
		p := &modmgr.Profile{GameID: gameID, Name: name}
		if err := p.Save(); err != nil {
			return err
		}
		fmt.Printf("created profile %q for game %q\n", name, gameID)
		return nil
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles for the active game",
	RunE: func(cmd *cobra.Command, args []string) error {
		gameID, err := requireActiveGameID()
		if err != nil {
			return err
		}
		names, err := modmgr.ListProfiles(gameID)
		if err != nil {
			return err
		}
		active := config.GetActiveProfile(gameID)
		if len(names) == 0 {
			fmt.Printf("no profiles for game %q\n", gameID)
			return nil
		}
		for _, n := range names {
			marker := ""
			if n == active {
				marker = " [active]"
			}
			fmt.Printf("  %s%s\n", n, marker)
		}
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active profile for the active game",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gameID, err := requireActiveGameID()
		if err != nil {
			return err
		}
		name := args[0]
		if err := config.SetActiveProfile(gameID, name); err != nil {
			return err
		}
		fmt.Printf("active profile set to %q for game %q\n", name, gameID)
		return nil
	},
}

var profileAddCmd = &cobra.Command{
	Use:   "add <mod-id>",
	Short: "Add a mod to the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gameID, err := requireActiveGameID()
		if err != nil {
			return err
		}
		modID := args[0]
		if _, err := modmgr.LoadMeta(modID); err != nil {
			return fmt.Errorf("mod %q not found â€“ import it first", modID)
		}
		profileName := config.GetActiveProfile(gameID)
		if profileName == "" {
			return fmt.Errorf("no active profile â€“ run 'itr profile use <name>' first")
		}
		p, err := modmgr.LoadProfile(gameID, profileName)
		if err != nil {
			return err
		}
		if p.HasMod(modID) {
			fmt.Printf("mod %q already in profile %q\n", modID, profileName)
			return nil
		}
		p.AddMod(modID)
		if err := p.Save(); err != nil {
			return err
		}
		fmt.Printf("added %q to profile %q\n", modID, profileName)
		return nil
	},
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <mod-id>",
	Short: "Remove a mod from the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gameID, err := requireActiveGameID()
		if err != nil {
			return err
		}
		modID := args[0]
		profileName := config.GetActiveProfile(gameID)
		if profileName == "" {
			return fmt.Errorf("no active profile â€“ run 'itr profile use <name>' first")
		}
		p, err := modmgr.LoadProfile(gameID, profileName)
		if err != nil {
			return err
		}
		if !p.HasMod(modID) {
			fmt.Printf("mod %q not in profile %q\n", modID, profileName)
			return nil
		}
		p.RemoveMod(modID)
		if err := p.Save(); err != nil {
			return err
		}
		fmt.Printf("removed %q from profile %q\n", modID, profileName)
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show mods in a profile (defaults to active)",
	RunE: func(cmd *cobra.Command, args []string) error {
		gameID, err := requireActiveGameID()
		if err != nil {
			return err
		}
		name := config.GetActiveProfile(gameID)
		if len(args) > 0 {
			name = args[0]
		}
		if name == "" {
			return fmt.Errorf("no active profile â€“ run 'itr profile use <name>' first")
		}
		p, err := modmgr.LoadProfile(gameID, name)
		if err != nil {
			return err
		}
		if len(p.Mods) == 0 {
			fmt.Printf("profile %q has no mods\n", name)
			return nil
		}
		fmt.Printf("profile %q (game: %s):\n", name, gameID)
		for i, id := range p.Mods {
			m, err := modmgr.LoadMeta(id)
			if err != nil {
				fmt.Printf("  %d. %s  (metadata missing)\n", i+1, id)
				continue
			}
			fmt.Printf("  %d. %-30s  %s\n", i+1, id, m.Name)
		}
		return nil
	},
}

func init() {
	profileCmd.AddCommand(
		profileNewCmd,
		profileListCmd,
		profileUseCmd,
		profileAddCmd,
		profileRemoveCmd,
		profileShowCmd,
	)
}
