package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/ray/internal/pipeline"
)

// newCheckCmd is intentionally a thin adapter. All semantics, stage ordering,
// evidence binding, and verdict derivation live in pipeline.Run, which is also
// the only path used by `ray start`.
func newCheckCmd() *cobra.Command {
	return newPipelineCommand(
		"check <task-folder>",
		"Verify one frozen finite task and its tests",
	)
}

func newPipelineCommand(use, short string) *cobra.Command {
	var configPath, certificatePath string
	command := &cobra.Command{
		Use:          use,
		Short:        short,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := pipeline.Run(cmd.Context(), pipeline.Request{
				Root:            args[0],
				ConfigPath:      configPath,
				CertificatePath: certificatePath,
			})
			out := cmd.OutOrStdout()
			for _, stage := range result.Stages {
				fmt.Fprintf(out, "%s: %s\n", stage.Name, stage.Status)
				for _, diagnostic := range stage.Diagnostic {
					fmt.Fprintf(out, "  %s\n", diagnostic)
				}
			}
			if result.CertificatePath != "" {
				fmt.Fprintf(out, "certificate: %s\n", result.CertificatePath)
			}
			fmt.Fprintln(out, result.Verdict)
			if !result.Successful() {
				return verdictError(result.Verdict)
			}
			return nil
		},
	}
	command.Flags().StringVar(&configPath, "config", "", "ray.toml path relative to the task folder")
	command.Flags().StringVar(&certificatePath, "certificate", "", "certificate output path relative to the task folder")
	return command
}

// verdictError keeps CLI errors inside the same mandatory vocabulary. Cobra's
// root prints this to stderr and main exits non-zero.
type verdictError pipeline.Verdict

func (err verdictError) Error() string { return string(err) }

// The types below keep the disconnected legacy mutation implementation
// buildable for research use. No production command constructs one of these
// values, and mutation results are never passed to pipeline.Run or a
// certificate.
type passState int

const (
	passPending passState = iota
	passRunning
	passOK
	passFailed
	passSkipped
	passAdvisory
)

type passResult struct {
	name     string
	state    passState
	summary  string
	findings []string
	dur      time.Duration
}

func shorten(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 180 {
		return value[:177] + "..."
	}
	return value
}
