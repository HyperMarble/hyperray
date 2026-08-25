package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/speclint"
	"github.com/HyperMarble/ray/internal/specparser"
)

type specLintResult struct {
	Pass   bool             `json:"pass"`
	Issues []speclint.Issue `json:"issues"`
}

func newSpecLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec-lint <spec.md>",
		Short: "Check a spec.md's condition tables for completeness and disjointness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSpecLint(cmd, args[0])
		},
	}
	cmd.SilenceUsage = true
	return cmd
}

func runSpecLint(cmd *cobra.Command, specPath string) error {
	content, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("spec-lint: %w", err)
	}

	tables, err := specparser.Parse(string(content))
	if err != nil {
		return fmt.Errorf("spec-lint: %w", err)
	}

	issues, err := speclint.Check(tables)
	if err != nil {
		return fmt.Errorf("spec-lint: %w", err)
	}

	if issues == nil {
		issues = []speclint.Issue{}
	}
	result := specLintResult{Pass: len(issues) == 0, Issues: issues}

	out := cmd.OutOrStdout()
	if result.Pass {
		fmt.Fprintln(out, "PASS: spec-lint")
	} else {
		fmt.Fprintf(out, "FAIL: spec-lint (%d issue(s))\n", len(issues))
		for _, iss := range issues {
			fmt.Fprintf(out, "  [%s] %s: %s\n", iss.Kind, iss.Section, iss.Message)
		}
	}

	if err := writeSpecLintLog(specPath, result); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "spec-lint: warning: could not write log: %v\n", err)
	}

	if !result.Pass {
		return fmt.Errorf("spec-lint: %d issue(s) found", len(issues))
	}
	return nil
}

func writeSpecLintLog(specPath string, result specLintResult) error {
	logDir := filepath.Join(filepath.Dir(specPath), "logs", "spec-lint")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(logDir, "result.json"), resultJSON, 0o644); err != nil {
		return err
	}

	var human string
	if result.Pass {
		human = "PASS: spec-lint\n"
	} else {
		human = fmt.Sprintf("FAIL: spec-lint (%d issue(s))\n", len(result.Issues))
		for _, iss := range result.Issues {
			human += fmt.Sprintf("[%s] %s: %s\n", iss.Kind, iss.Section, iss.Message)
		}
	}
	return os.WriteFile(filepath.Join(logDir, "output.log"), []byte(human), 0o644)
}
