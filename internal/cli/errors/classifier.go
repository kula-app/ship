package errors

import (
	"regexp"
	"strings"

	"github.com/getsentry/sentry-go"
)

// IsUserError determines if a Sentry event represents a user error that should be filtered
// rather than an application error that should be reported.
//
// User errors include invalid flags, validation errors, missing credentials, and precondition
// failures (e.g., not authenticated). Application errors include crashes, API failures,
// and internal bugs.
//
// The classifier uses two strategies:
// 1. Transaction status (most reliable) - commands set status codes like invalid_argument
// 2. Error message patterns (fallback) - for cases where status isn't set
func IsUserError(event *sentry.Event) bool {
	// Always allow transaction events (performance monitoring)
	if event.Type == "transaction" {
		return false
	}

	// Check transaction status if available (most reliable method)
	if status := getTransactionStatus(event); status != sentry.SpanStatusUndefined {
		return isUserErrorStatus(status)
	}

	// Fallback: check error message patterns
	return matchesUserErrorPattern(event)
}

// getTransactionStatus extracts the transaction status from a Sentry event.
// The status is stored in the event's contexts under "trace.status".
func getTransactionStatus(event *sentry.Event) sentry.SpanStatus {
	if event.Contexts == nil {
		return sentry.SpanStatusUndefined
	}

	traceContext, ok := event.Contexts["trace"]
	if !ok {
		return sentry.SpanStatusUndefined
	}

	// Context is already map[string]interface{}
	status, ok := traceContext["status"].(string)
	if !ok {
		return sentry.SpanStatusUndefined
	}

	// Convert string status to SpanStatus by matching the string representation
	switch status {
	case "ok":
		return sentry.SpanStatusOK
	case "cancelled":
		return sentry.SpanStatusCanceled
	case "unknown", "unknown_error":
		return sentry.SpanStatusUnknown
	case "invalid_argument":
		return sentry.SpanStatusInvalidArgument
	case "deadline_exceeded":
		return sentry.SpanStatusDeadlineExceeded
	case "not_found":
		return sentry.SpanStatusNotFound
	case "already_exists":
		return sentry.SpanStatusAlreadyExists
	case "permission_denied":
		return sentry.SpanStatusPermissionDenied
	case "resource_exhausted":
		return sentry.SpanStatusResourceExhausted
	case "failed_precondition":
		return sentry.SpanStatusFailedPrecondition
	case "aborted":
		return sentry.SpanStatusAborted
	case "out_of_range":
		return sentry.SpanStatusOutOfRange
	case "unimplemented":
		return sentry.SpanStatusUnimplemented
	case "internal_error":
		return sentry.SpanStatusInternalError
	case "unavailable":
		return sentry.SpanStatusUnavailable
	case "data_loss":
		return sentry.SpanStatusDataLoss
	case "unauthenticated":
		return sentry.SpanStatusUnauthenticated
	default:
		return sentry.SpanStatusUndefined
	}
}

// isUserErrorStatus returns true if the given status represents a user error.
func isUserErrorStatus(status sentry.SpanStatus) bool {
	switch status {
	case sentry.SpanStatusInvalidArgument,
		sentry.SpanStatusUnauthenticated,
		sentry.SpanStatusFailedPrecondition:
		return true
	default:
		return false
	}
}

// matchesUserErrorPattern checks if the error message matches known user error patterns.
// This is a fallback when transaction status is not available.
func matchesUserErrorPattern(event *sentry.Event) bool {
	// Check exception messages
	for _, exception := range event.Exception {
		if exception.Value != "" && isUserErrorMessage(exception.Value) {
			return true
		}
	}

	// Check event message
	if event.Message != "" && isUserErrorMessage(event.Message) {
		return true
	}

	return false
}

// isUserErrorMessage checks if a message indicates a user error.
func isUserErrorMessage(msg string) bool {
	if isCLIUsageErrorMessage(msg) {
		return true
	}

	userErrorPatterns := []string{
		"invalid log format",
		"must be 'text' or 'json'",
		"not authenticated",
		"credentials expired",
		"SHIP_API_KEY",
		"app identifier required",
		"are mutually exclusive",
		"invalid method",
		"invalid field format",
		"invalid field key",
		"invalid header format",
		"invalid API endpoint",
		"cannot use --data",
		"file not found",
		"missing required",
		"invalid flag",
	}

	lowerMsg := strings.ToLower(msg)
	for _, pattern := range userErrorPatterns {
		if strings.Contains(lowerMsg, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

const (
	goDoubleQuotedValue = `"(?:[^"\\]|\\.)*"`
	goSingleQuotedValue = `'(?:[^'\\]|\\.)*'`
	pflagUsageError     = `(?:unknown flag: --.*|unknown shorthand flag: ` + goSingleQuotedValue + ` in -.*|flag needs an argument: (?:--.*|` + goSingleQuotedValue + ` in -.*)|invalid argument ` + goDoubleQuotedValue + ` for ` + goDoubleQuotedValue + ` flag: (?s:.*)|bad flag syntax: .*)`
)

var cliUsageErrorPatterns = []*regexp.Regexp{
	// Cobra command and positional argument validation. Go-quoted values may
	// be empty and may contain escaped quotes or spaces.
	regexp.MustCompile(`^unknown command ` + goDoubleQuotedValue + ` for ` + goDoubleQuotedValue + `(?:\n\nDid you mean this\?(?:\n\t[^\n]*)+)?$`),
	regexp.MustCompile(`^invalid argument ` + goDoubleQuotedValue + ` for ` + goDoubleQuotedValue + `(?:\n\nDid you mean this\?(?:\n\t[^\n]*)+)?$`),
	regexp.MustCompile(`^(?:requires at least [0-9]+ arg\(s\), only received [0-9]+|accepts at most [0-9]+ arg\(s\), received [0-9]+|accepts [0-9]+ arg\(s\), received [0-9]+|accepts between [0-9]+ and [0-9]+ arg\(s\), received [0-9]+)$`),

	// Cobra required flags and flag-group validation.
	regexp.MustCompile(`^required flag\(s\) ` + goDoubleQuotedValue + `(?:, ` + goDoubleQuotedValue + `)* not set$`),
	regexp.MustCompile(`^if any flags in the group \[.*\] are set they must all be set; missing \[.*\]$`),
	regexp.MustCompile(`^at least one of the flags in the group \[.*\] is required$`),
	regexp.MustCompile(`^if any flags in the group \[.*\] are set none of the others can be; \[.*\] were all set$`),

	// Cobra completion command lookup and completion-time flag parsing.
	regexp.MustCompile(`^unable to find a command for arguments: \[(?s:.*)\]$`),
	regexp.MustCompile(`^Error while parsing flags from args \[(?s:.*)\]: ` + pflagUsageError + `$`),

	// pflag parsing and value validation.
	regexp.MustCompile(`^` + pflagUsageError + `$`),
}

// isCLIUsageErrorMessage recognizes anchored errors emitted by Cobra and pflag
// while parsing or validating user input. Anchoring avoids filtering runtime
// errors that merely contain generic CLI-related phrases.
func isCLIUsageErrorMessage(msg string) bool {
	trimmedMsg := strings.TrimSpace(msg)
	for _, pattern := range cliUsageErrorPatterns {
		if pattern.MatchString(trimmedMsg) {
			return true
		}
	}

	return false
}
