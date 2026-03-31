package cmd

import (
	"fmt"
	"strings"

	"github.com/ITR-MOD/field-kit/internal/modmgr"
	"github.com/spf13/cobra"
)

var modCmd = &cobra.Command{
	Use:   "mod",
	Short: "Manage imported mods",
}

var modImportCmd = &cobra.Command{
	Use:   "import <zip>",
	Short: "Import a mod from a zip archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		meta, err := modmgr.Import(args[0])
		if err != nil {
			return err
		}
		types := make([]string, len(meta.Types))
		for i, t := range meta.Types {
			types[i] = string(t)
		}
		fmt.Printf("imported %q\n  id:    %s\n  types: %s\n  files: %d\n",
			meta.Name, meta.ID, strings.Join(types, ", "), len(meta.Files))
		return nil
	},
}

var modListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all imported mods",
	RunE: func(cmd *cobra.Command, args []string) error {
		mods, err := modmgr.ListMods()
		if err != nil {
			return err
		}
		if len(mods) == 0 {
			fmt.Println("no mods imported")
			return nil
		}
		for _, m := range mods {
			types := make([]string, len(m.Types))
			for i, t := range m.Types {
				types[i] = string(t)
			}
			fmt.Printf("  %-30s  %-12s  %s\n", m.ID, strings.Join(types, "+"), m.Name)
		}
		return nil
	},
}

var modInfoCmd = &cobra.Command{
	Use:   "info <id>",
	Short: "Show details for a mod",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := modmgr.LoadMeta(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("id:      %s\n", m.ID)
		fmt.Printf("name:    %s\n", m.Name)
		fmt.Printf("archive: %s\n", m.ArchiveName)
		fmt.Printf("imported:%s\n", m.ImportedAt.Format("2006-01-02 15:04:05"))
		types := make([]string, len(m.Types))
		for i, t := range m.Types {
			types[i] = string(t)
		}
		fmt.Printf("types:   %s\n", strings.Join(types, ", "))
		fmt.Printf("files:   %d\n", len(m.Files))

		// Show mapped instructions.
		instructions := modmgr.MapFiles(m.Files)
		if len(instructions) > 0 {
			fmt.Println("\ninstall map:")
			for _, instr := range instructions {
				custom := ""
				if instr.IsCustom {
					custom = " [custom/backup]"
				}
				fmt.Printf("  %s\n    → %s%s\n", instr.Source, instr.Dest, custom)
			}
		}
		return nil
	},
}

var modRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a mod from the local cache",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if err := modmgr.RemoveMod(id); err != nil {
			return err
		}
		fmt.Printf("removed mod %s\n", id)
		return nil
	},
}

func init() {
	modCmd.AddCommand(modImportCmd, modListCmd, modInfoCmd, modRemoveCmd)
}
