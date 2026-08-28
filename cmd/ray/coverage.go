package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/coverage"
	"github.com/HyperMarble/ray/internal/specparser"
)

func newCoverageCmd() *cobra.Command {
	var pictPath string
	var strength int

	cmd := &cobra.Command{
		Use:   "coverage <spec.md>",
		Short: "Generate the combinatorial test matrix from spec.md's declared parameters",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("coverage: %w", err)
			}
			tables, err := specparser.Parse(string(content))
			if err != nil {
				return fmt.Errorf("coverage: %w", err)
			}

			results, err := coverage.Generate(tables, pictPath, strength)
			if err != nil {
				return fmt.Errorf("coverage: %w", err)
			}

			out := cmd.OutOrStdout()
			total := 0
			for _, r := range results {
				total += len(r.Combinations)
				fmt.Fprintf(out, "  %s (line %d): %d combination(s)\n",
					r.Section, r.Line, len(r.Combinations))
			}
			fmt.Fprintf(out, "coverage: %d table(s), %d combination(s)\n", len(results), total)

			if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&pictPath, "pict", "", "path to the pict binary (default: pict on PATH)")
	cmd.Flags().IntVar(&strength, "strength", 0, "t-way combination strength (0 = pict's default, pairwise)")
	cmd.Flags().Bool("json", false, "also emit the full matrix as JSON")
	cmd.SilenceUsage = true
	return cmd
}
