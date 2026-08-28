package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// The scaffold embeds the real template and schema so a published binary
// works without the repository. tests/architecture_freeze_test.go pins the
// embedded copies byte-identical to the originals, so they cannot drift.
//
//go:embed scaffold/spec.md scaffold/schema.md
var scaffoldFiles embed.FS

// newSpecInitCmd writes a schema-correct spec.md starting point plus the
// full schema reference into a task folder, so authoring starts from the
// exact format the compiler accepts instead of from memory.
func newSpecInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "init <task-dir>",
		Aliases:      []string{"spec-init"},
		Short:        "Write a schema-correct spec.md template and the schema reference into a task folder",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskDir, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(taskDir, "bridges"), 0o755); err != nil {
				return err
			}
			for source, target := range map[string]string{
				"scaffold/spec.md":   "spec.md",
				"scaffold/schema.md": "SCHEMA.md",
			} {
				destination := filepath.Join(taskDir, target)
				if _, err := os.Stat(destination); err == nil {
					return fmt.Errorf("spec-init: %s already exists; refusing to overwrite", destination)
				}
				body, err := scaffoldFiles.ReadFile(source)
				if err != nil {
					return err
				}
				if err := os.WriteFile(destination, body, 0o644); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s/spec.md (template), %s/SCHEMA.md (format reference), %s/bridges/\nnext: author rows per SCHEMA.md, then `ray spec-lint spec.md --instruction instruction.md --reference solution.patch --task-id <id>`\n", taskDir, taskDir, taskDir)
			return nil
		},
	}
}
