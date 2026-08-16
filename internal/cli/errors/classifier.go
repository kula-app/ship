package errors

import (
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
		"required flag",
		"missing required",
		"invalid argument",
		"invalid flag",
		"accepts 1 arg(s)",
	}

	lowerMsg := strings.ToLower(msg)
	for _, pattern := range userErrorPatterns {
		if strings.Contains(lowerMsg, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
