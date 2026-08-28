package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/oracle"
)

func newOracleCmd() *cobra.Command {
	var (
		lang     string
		ensures  string
		requires string
		pyPath   string
		verusBin string
		esbmcBin string
		unwind   int
	)

	cmd := &cobra.Command{
		Use:   "oracle <model-file>",
		Short: "Prove a reference model's property for every possible input",
		Long: "Proves a property of a simplified reference model using an SMT-based\n" +
			"prover: touchstone for Python, Verus for Rust, ESBMC for C/C++.\n\n" +
			"For Rust and C++ the property is written in the model file itself\n" +
			"(Verus requires/ensures clauses, or assert/__ESBMC_assume), so\n" +
			"--ensures applies to Python only.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("oracle: %w", err)
			}

			var v oracle.Verdict
			switch lang {
			case "python":
				if ensures == "" {
					return fmt.Errorf("oracle: --ensures is required for python")
				}
				v, err = oracle.Prove(pyPath, string(src), ensures, requires)
			case "rust":
				v, err = oracle.ProveRust(verusBin, string(src))
			case "cpp", "c":
				v, err = oracle.ProveCPP(esbmcBin, string(src), unwind)
			default:
				return fmt.Errorf("oracle: unsupported --lang %q (want python, rust, or cpp)", lang)
			}
			if err != nil {
				return fmt.Errorf("oracle: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s: oracle (%s)\n", v.Status, lang)
			if v.Counterexample != "" {
				fmt.Fprintf(out, "  counterexample: %s\n", v.Counterexample)
			}
			if v.Status == "UNKNOWN" && v.Reason != "" {
				fmt.Fprintf(out, "  reason: %s\n", firstLine(v.Reason))
			}

			// REFUTED is a real finding, not a tool failure -- but it must
			// still fail the command so a pipeline stops on it.
			if v.Status != "PROVED" {
				return fmt.Errorf("oracle: %s", v.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lang, "lang", "python", "model language: python, rust, or cpp")
	cmd.Flags().StringVar(&ensures, "ensures", "", "postcondition to prove (python only)")
	cmd.Flags().StringVar(&requires, "requires", "True", "precondition assumed (python only)")
	cmd.Flags().StringVar(&pyPath, "python", "", "python3 from a patched touchstone venv")
	cmd.Flags().StringVar(&verusBin, "verus", "", "path to the verus binary")
	cmd.Flags().StringVar(&esbmcBin, "esbmc", "", "path to the esbmc binary")
	cmd.Flags().IntVar(&unwind, "unwind", 10, "ESBMC loop unwind bound (cpp only)")
	cmd.SilenceUsage = true
	return cmd
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
