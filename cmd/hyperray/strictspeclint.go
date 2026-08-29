package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HyperMarble/hyperray/internal/semanticir"
	"github.com/HyperMarble/hyperray/internal/speccompiler"
)

// newStrictSpecLintCmd is the authoring-time view of the same strict compiler
// used by pipeline.Run. It deliberately performs no proof and therefore never
// emits a verification verdict or certificate.
func newStrictSpecLintCmd() *cobra.Command {
	var instructionPath, referencePath, taskID string
	command := &cobra.Command{
		Use:          "lint <spec.md>",
		Aliases:      []string{"spec-lint"},
		Short:        "Strictly compile a finite spec against frozen instruction bytes",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}

			specSource, err := os.ReadFile(specPath)
			if err != nil {
				return fmt.Errorf("read spec: %w", err)
			}
			// Optional: a spec may anchor every row into the reference.
			var instruction semanticir.ArtifactRef
			var instructionSource []byte
			if instructionPath != "" {
				path, err := filepath.Abs(instructionPath)
				if err != nil {
					return err
				}
				if instructionSource, err = os.ReadFile(path); err != nil {
					return fmt.Errorf("read instruction: %w", err)
				}
				instruction = semanticir.ArtifactRef{
					ID: "instruction", Kind: semanticir.ArtifactInstruction, Path: path,
					Digest: semanticir.DigestBytes(instructionSource),
				}
			}
			// The reference is optional: a spec whose rows all anchor into
			// the instruction compiles without it, exactly as before.
			var reference semanticir.ArtifactRef
			var referenceSource []byte
			if referencePath != "" {
				path, err := filepath.Abs(referencePath)
				if err != nil {
					return err
				}
				if referenceSource, err = os.ReadFile(path); err != nil {
					return fmt.Errorf("read reference: %w", err)
				}
				reference = semanticir.ArtifactRef{
					ID: "reference", Kind: semanticir.ArtifactCode, Path: path,
					Digest: semanticir.DigestBytes(referenceSource),
				}
			}
			task, diagnostics := speccompiler.Compile(cmd.Context(), speccompiler.Request{
				TaskID: taskID,
				Artifact: semanticir.ArtifactRef{
					ID: "spec", Kind: semanticir.ArtifactSpec, Path: specPath,
					Digest: semanticir.DigestBytes(specSource),
				},
				Source:            specSource,
				Instruction:       instruction,
				InstructionSource: instructionSource,
				Reference:         reference,
				ReferenceSource:   referenceSource,
			})
			for _, diagnostic := range diagnostics {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", diagnostic.Code, diagnostic.Message)
			}
			// A bridge declared in the spec must exist on disk: "declared on
			// paper, absent on disk" once let a spec read as complete while
			// its rules were untestable.
			if task != nil {
				missing := 0
				for _, commands := range []map[string]string{task.Classifiers} {
					for _, command := range commands {
						missing += reportMissingBridgeFile(cmd, specPath, command)
					}
				}
				for _, byLabel := range task.Observers {
					for _, command := range byLabel {
						missing += reportMissingBridgeFile(cmd, specPath, command)
					}
				}
				if missing > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "bridge-file-missing: %d declared bridge command file(s) absent; their rules stay untestable until the files exist\n", missing)
				}
			}
			// Warnings print above but only errors block: a warning is the
			// author's call to make, with the line named.
			if semanticir.HasErrors(diagnostics) || task == nil {
				return fmt.Errorf("strict spec compilation blocked")
			}
			digest, err := semanticir.Digest(task)
			if err != nil {
				return err
			}
			frozenDigest, err := semanticir.FrozenSpecSemanticsDigest(task)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spec: complete\nir: %s\nfrozen-semantics: %s\n", digest, frozenDigest)
			return nil
		},
	}
	command.Flags().StringVar(&instructionPath, "instruction", "", "frozen instruction path")
	_ = command.MarkFlagRequired("instruction")
	command.Flags().StringVar(&referencePath, "reference", "", "frozen reference solution path, for rows anchored with reference:<span>")
	command.Flags().StringVar(&taskID, "task-id", "", "stable task identifier")
	_ = command.MarkFlagRequired("task-id")
	return command
}

// reportMissingBridgeFile checks a declared bridge command's file exists
// beside the spec; returns 1 when it does not.
func reportMissingBridgeFile(cmd *cobra.Command, specPath, command string) int {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return 0
	}
	target := fields[len(fields)-1]
	if !strings.Contains(target, "/") {
		return 0
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(specPath), filepath.FromSlash(target))); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "bridge-file-missing: %s\n", target)
		return 1
	}
	return 0
}
