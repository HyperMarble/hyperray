// Request values name every artifact and finite guard for one Isla operation.
// They must not contain program-specific translation rules.
package isla

import "strconv"

// Request contains one bounded machine-property query.
type Request struct {
	architecture  Artifact
	configuration Artifact
	memoryModel   Artifact
	program       Artifact
	pcVisitLimit  uint64
	timeLimit     uint64
}

// NewRequest creates one query from identified artifacts and finite guards.
func NewRequest(architecture Artifact, configuration Artifact, memoryModel Artifact, program Artifact, pcVisitLimit uint64, timeLimitSeconds uint64) (Request, error) {
	if pcVisitLimit == 0 || timeLimitSeconds == 0 {
		return Request{}, engineError(InvalidInput, "request", "limits must be more than zero")
	}
	request := Request{
		architecture:  architecture,
		configuration: configuration,
		memoryModel:   memoryModel,
		program:       program,
		pcVisitLimit:  pcVisitLimit,
		timeLimit:     timeLimitSeconds,
	}
	if err := request.current(); err != nil {
		return Request{}, err
	}
	return request, nil
}

func (request Request) current() error {
	artifacts := []Artifact{request.architecture, request.configuration, request.memoryModel, request.program}
	for index := range artifacts {
		artifact := artifacts[index]
		if err := artifact.current(); err != nil {
			return err
		}
	}
	return nil
}

func (request Request) arguments() []string {
	return []string{
		"--herd7", "-A", request.architecture.path,
		"-C", request.configuration.path,
		"-m", request.memoryModel.path,
		"--pc-limit", strconv.FormatUint(request.pcVisitLimit, 10),
		"--pc-limit-mode", "error",
		"-s", strconv.FormatUint(request.timeLimit, 10),
		request.program.path,
	}
}
