package main

import "github.com/spf13/cobra"

// `hyperray start` and `hyperray check` are deliberately aliases over one production
// pipeline. Start does not run a preparatory best-effort path whose result can
// drift from check; task materialization is declared in the frozen workspace
// triple and is therefore part of pipeline.Run itself.
func newStartCmd() *cobra.Command {
	return newPipelineCommand(
		"start <task-folder>",
		"Materialize and verify one frozen finite task",
	)
}
