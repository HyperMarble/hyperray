// This file defines observable ARM64 Mach-O header rejections.
// It must not add loader, runtime, or model semantics.
package arm64

// RejectionCode names one unsupported ARM64 Mach-O header condition.
type RejectionCode string

const (
	UnsupportedMagic     RejectionCode = "unsupported_magic"
	UnsupportedByteOrder RejectionCode = "unsupported_byte_order"
	UnsupportedCPU       RejectionCode = "unsupported_cpu"
	UnsupportedSubCPU    RejectionCode = "unsupported_subcpu"
	UnsupportedType      RejectionCode = "unsupported_type"
)

// Rejection reports the first header policy rule that failed.
type Rejection struct {
	Code   RejectionCode
	Detail string
}

func (rejection *Rejection) Error() string {
	return string(rejection.Code) + ": " + rejection.Detail
}
