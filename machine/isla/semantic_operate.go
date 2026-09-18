// Semantic execution retains one complete, resource-bounded program dump.
// Any process, artifact, diagnostic, or protocol failure discards the report.
package isla

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func (engine SemanticEngine) inspect(ctx context.Context, request VerificationRequest) (SemanticReport, error) {
	output, err := engine.runSemanticDump(ctx, request)
	if err != nil {
		return SemanticReport{}, err
	}
	summary, err := parseSemanticOutput(output.stdout)
	if err != nil {
		return SemanticReport{}, err
	}
	dispositions, err := classifyDiagnostics(output, "semantic diagnostic")
	if err != nil {
		return SemanticReport{}, err
	}
	if err := engine.current(); err != nil {
		return SemanticReport{}, err
	}
	if err := request.current(); err != nil {
		return SemanticReport{}, err
	}
	return newSemanticReport(engine, request, output, summary, dispositions), nil
}

func (engine SemanticEngine) runSemanticDump(ctx context.Context, request VerificationRequest) (commandOutput, error) {
	duration, err := boundedDuration(request.query.timeLimit)
	if err != nil {
		return commandOutput{}, err
	}
	limitedContext, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	stdout := newLimitedBuffer(request.query.maximumOutputSize)
	diagnostics := newLimitedBuffer(request.query.maximumOutputSize)
	command := exec.CommandContext(limitedContext, engine.identity.Path, request.semanticArguments()...)
	command.Stdout = stdout
	command.Stderr = diagnostics
	start := time.Now()
	err = command.Run()
	output := commandOutput{stdout: stdout.String(), diagnostics: diagnostics.String(), elapsed: time.Since(start)}
	resourceError := limitedContext.Err()
	if resourceError != nil || stdout.exceeded || diagnostics.exceeded {
		details := make([]string, 0, 3)
		if resourceError != nil {
			contextDetail := "context: canceled"
			if resourceError == context.DeadlineExceeded {
				contextDetail = "context: deadline exceeded"
			}
			details = append(details, contextDetail)
		}
		if stdout.exceeded {
			details = append(details, fmt.Sprintf("stdout: cap exceeded (retained %d bytes, cap %d bytes)", len(stdout.content), stdout.limit))
		}
		if diagnostics.exceeded {
			details = append(details, fmt.Sprintf("stderr: cap exceeded (retained %d bytes, cap %d bytes)", len(diagnostics.content), diagnostics.limit))
		}
		return commandOutput{}, engineError(ResourceLimit, request.query.program.path, strings.Join(details, "; "))
	}
	if err != nil {
		detail := strings.TrimSpace(output.diagnostics + "\n" + output.stdout)
		return commandOutput{}, engineError(ProcessFail, request.query.program.path, detail)
	}
	return output, nil
}
