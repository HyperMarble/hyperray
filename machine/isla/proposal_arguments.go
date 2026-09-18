// Candidate labels are requested only from the release that implements them.
// Legacy tool versions keep their existing command-line contract.
package isla

import (
	"slices"
	"strings"
)

func proposalArguments(identity ToolIdentity, request Request) []string {
	arguments := request.arguments()
	if slices.Contains(strings.Split(identity.Version, "/"), "candidate-status-v1") {
		arguments = append(arguments, "--candidate-status")
	}
	return arguments
}
