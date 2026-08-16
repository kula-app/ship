package errors

import (
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestIsUserError_TransactionEvents(t *testing.T) {
	t.Run("transaction events are never filtered", func(t *testing.T) {
		event := &sentry.Event{
			Type: "transaction",
		}

		if IsUserError(event) {
			t.Error("transaction events should not be filtered")
		}
	})

	t.Run("transaction events with user error status are not filtered", func(t *testing.T) {
		event := &sentry.Event{
			Type: "transaction",
			Contexts: map[string]sentry.Context{
				"trace": map[string]interface{}{
					"status": sentry.SpanStatusInvalidArgument.String(),
				},
			},
		}

		if IsUserError(event) {
			t.Error("transaction events should not be filtered even with user error status")
		}
	})
}

func TestIsUserError_StatusBased(t *testing.T) {
	tests := []struct {
		name             string
		status           sentry.SpanStatus
		shouldBeFiltered bool
		description      string
	}{
		{
			name:             "invalid_argument status",
			status:           sentry.SpanStatusInvalidArgument,
			shouldBeFiltered: true,
			description:      "Invalid flags, validation errors",
		},
		{
			name:             "unauthenticated status",
			status:           sentry.SpanStatusUnauthenticated,
			shouldBeFiltered: true,
			description:      "Missing credentials or expired session",
		},
		{
			name:             "failed_precondition status",
			status:           sentry.SpanStatusFailedPrecondition,
			shouldBeFiltered: true,
			description:      "Preconditions not met",
		},
		{
			name:             "internal_error status",
			status:           sentry.SpanStatusInternalError,
			shouldBeFiltered: false,
			description:      "Application bugs must be reported",
		},
		{
			name:             "unavailable status",
			status:           sentry.SpanStatusUnavailable,
			shouldBeFiltered: false,
			description:      "API failures must be reported",
		},
		{
			name:             "not_found status",
			status:           sentry.SpanStatusNotFound,
			shouldBeFiltered: false,
			description:      "Missing resources must be reported",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := &sentry.Event{
				Contexts: map[string]sentry.Context{
					"trace": map[string]interface{}{
						"status": test.status.String(),
					},
				},
			}

			if got := IsUserError(event); got != test.shouldBeFiltered {
				t.Errorf("IsUserError() = %v, want %v (%s)", got, test.shouldBeFiltered, test.description)
			}
		})
	}
}

func TestIsUserError_MessageBased(t *testing.T) {
	tests := []struct {
		name             string
		message          string
		shouldBeFiltered bool
	}{
		{
			name:             "not authenticated",
			message:          "not authenticated — run 'ship auth login' first or set SHIP_API_KEY",
			shouldBeFiltered: true,
		},
		{
			name:             "missing app identifier",
			message:          "app identifier required: pass --app-id or --app-slug",
			shouldBeFiltered: true,
		},
		{
			name:             "invalid method",
			message:          "invalid method: TRACE. Must be one of: GET, POST, PUT, DELETE, PATCH",
			shouldBeFiltered: true,
		},
		{
			name:             "invalid field format",
			message:          "invalid field format: status. Expected key=value",
			shouldBeFiltered: true,
		},
		{
			name:             "missing input file",
			message:          "file not found: body.json",
			shouldBeFiltered: true,
		},
		{
			name:             "transport failure",
			message:          "sending request: connection refused",
			shouldBeFiltered: false,
		},
		{
			name:             "unexpected panic",
			message:          "runtime error: index out of range",
			shouldBeFiltered: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := &sentry.Event{Message: test.message}

			if got := IsUserError(event); got != test.shouldBeFiltered {
				t.Errorf("IsUserError(%q) = %v, want %v", test.message, got, test.shouldBeFiltered)
			}
		})
	}
}

func TestIsUserError_ExceptionMessages(t *testing.T) {
	event := &sentry.Event{
		Exception: []sentry.Exception{
			{Value: "credentials expired — run 'ship auth login' first"},
		},
	}

	if !IsUserError(event) {
		t.Error("expected an exception carrying a user error message to be filtered")
	}
}

func TestIsUserError_StatusTakesPrecedenceOverMessage(t *testing.T) {
	// A user error message must not filter an event the command marked internal.
	event := &sentry.Event{
		Message: "not authenticated",
		Contexts: map[string]sentry.Context{
			"trace": map[string]interface{}{
				"status": sentry.SpanStatusInternalError.String(),
			},
		},
	}

	if IsUserError(event) {
		t.Error("expected the transaction status to take precedence over the message pattern")
	}
}

func TestIsUserError_NoContextFallsBackToMessage(t *testing.T) {
	event := &sentry.Event{
		Contexts: nil,
		Message:  "invalid log format: yaml",
	}

	if !IsUserError(event) {
		t.Error("expected the message pattern fallback to apply when no trace context exists")
	}
}
