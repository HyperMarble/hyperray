// Verification requests derive semantic and solver work from one query.
// This construction prevents callers from supplying two different programs.
package isla

import "time"

// VerificationRequest contains one same-program bounded verification.
type VerificationRequest struct {
	query         Request
	threadLimit   uint64
	memoryLimitMB uint64
}

// NewVerificationRequest adds semantic resource limits to one solver query.
func NewVerificationRequest(query Request, threadLimit uint64, memoryLimitMB uint64) (VerificationRequest, error) {
	if threadLimit == 0 || memoryLimitMB == 0 {
		return VerificationRequest{}, engineError(InvalidInput, "verification limits", "limits must be more than zero")
	}
	request := VerificationRequest{
		query: query, threadLimit: threadLimit,
		memoryLimitMB: memoryLimitMB,
	}
	if err := request.current(); err != nil {
		return VerificationRequest{}, err
	}
	return request, nil
}

func (request VerificationRequest) current() error {
	return request.query.current()
}

func boundedDuration(seconds uint64) (time.Duration, error) {
	const maximumSeconds = uint64((1<<63 - 1) / int64(time.Second))
	if seconds == 0 || seconds > maximumSeconds {
		return 0, engineError(InvalidInput, "time limit", "cannot represent duration")
	}
	return time.Duration(seconds) * time.Second, nil
}
