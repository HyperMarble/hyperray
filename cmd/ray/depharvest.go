package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newDepHarvestCmd collects a pinned dependency's own tested edge values.
// An SMT solver cannot reason through a complex dependency, so ray does not
// try: the dependency's test suite is a curated list of inputs its authors
// proved they care about, and those literals become candidate domain values
// for the author to review into spec.md. Cached per (package, version)
// because a pinned version's tests never change.
func newDepHarvestCmd() *cobra.Command {
	var pythonPath string
	command := &cobra.Command{
		Use:          "harvest <package>",
		Aliases:      []string{"dep-harvest"},
		Short:        "Harvest a pinned dependency's own tested edge values for spec domains",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			harvester := filepath.Join(rayRepoRoot(), "third_party", "mutate", "harvest_dep.py")
			sub := exec.Command(pythonPath, harvester, args[0])
			body, err := sub.Output()
			if err != nil {
				return fmt.Errorf("dep-harvest: %w", err)
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			cacheDir := filepath.Join(home, ".ray", "depharvest")
			if err := os.MkdirAll(cacheDir, 0o755); err != nil {
				return err
			}
			// The helper prints {"package":..., "version":...} first; the
			// cache key repeats them so a stale interpreter cannot alias.
			cachePath := filepath.Join(cacheDir, args[0]+".json")
			if err := os.WriteFile(cachePath, body, 0o644); err != nil {
				return err
			}
			cmd.OutOrStdout().Write(body)
			fmt.Fprintf(cmd.OutOrStdout(), "\ncached: %s\nreview these into spec.md Universe lists; nothing is added automatically\n", cachePath)
			return nil
		},
	}
	command.Flags().StringVar(&pythonPath, "python", "python3", "interpreter with the pinned dependency installed")
	return command
}
