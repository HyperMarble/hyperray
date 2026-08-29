// Language-aware raise breakers: a rule of the form `raise X containing
// "msg"` is wrong three ways -- wrong message, wrong type, no raise -- and
// each way has one derived source edit per language. Python edits `raise
// X(...)`, Rust edits `panic!(...)`, C++ edits `throw X(...)`. A breaker
// that cannot be derived for a site returns "" and the row stays honestly
// not-derived rather than silently skipped.
package enforce

import "strings"

// SwapRaiseType rewrites the raise site to a different error type with the
// same control flow and message.
func SwapRaiseType(language, original, exceptionType, message string) string {
	switch language {
	case "rust":
		// A panic has no meaningful sibling type to swap to; the class is
		// covered by wrong-message and no-raise.
		return ""
	case "cpp":
		return swapBeforeMessage(original, message,
			"throw "+exceptionType+"(",
			"throw std::logic_error(")
	default:
		return swapBeforeMessage(original, message,
			"raise "+exceptionType+"(",
			"raise RuntimeError(")
	}
}

// SuppressRaise removes the raise entirely: the guarded condition now
// passes silently.
func SuppressRaise(language, original, exceptionType, message string) string {
	switch language {
	case "rust":
		return replaceCallThroughParen(original, message, "panic!(", "()")
	case "cpp":
		return replaceCallThroughParen(original, message, "throw "+exceptionType+"(", ";")
	default:
		return replaceCallThroughParen(original, message, "raise "+exceptionType+"(", "pass  # ray suppressed raise")
	}
}

// swapBeforeMessage replaces the nearest `site` before the message literal
// with `replacement`, keeping everything after the opening parenthesis.
func swapBeforeMessage(original, message, site, replacement string) string {
	index := strings.Index(original, message)
	if index < 0 {
		return ""
	}
	siteIndex := strings.LastIndex(original[:index], site)
	if siteIndex < 0 {
		return ""
	}
	return original[:siteIndex] + replacement + original[siteIndex+len(site):]
}

// replaceCallThroughParen replaces the nearest `site` before the message
// literal, through its closing parenthesis, with `replacement`.
func replaceCallThroughParen(original, message, site, replacement string) string {
	index := strings.Index(original, message)
	if index < 0 {
		return ""
	}
	siteIndex := strings.LastIndex(original[:index], site)
	if siteIndex < 0 {
		return ""
	}
	relativeEnd := strings.Index(original[siteIndex:], ")")
	if relativeEnd < 0 {
		return ""
	}
	return original[:siteIndex] + replacement + original[siteIndex+relativeEnd+1:]
}
