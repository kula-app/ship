package errors

import (
	"io"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestIsCLIUsageErrorMessage_UpstreamAPIErrors(t *testing.T) {
	tests := []struct {
		name         string
		expected     string
		produceError func() error
	}{
		{
			name:     "Cobra unknown empty command",
			expected: `unknown command "" for "ship"`,
			produceError: func() error {
				return cobra.NoArgs(&cobra.Command{Use: "ship"}, []string{""})
			},
		},
		{
			name:     "Cobra invalid argument with suggestion",
			expected: "invalid argument \"deplo\" for \"ship\"\n\nDid you mean this?\n\tdeploy\n",
			produceError: func() error {
				command := &cobra.Command{Use: "ship", ValidArgs: []string{"deploy"}}
				command.AddCommand(&cobra.Command{Use: "deploy", Run: func(*cobra.Command, []string) {}})
				return cobra.OnlyValidArgs(command, []string{"deplo"})
			},
		},
		{
			name:     "Cobra invalid empty argument",
			expected: `invalid argument "" for "ship deploy"`,
			produceError: func() error {
				root := &cobra.Command{Use: "ship"}
				command := &cobra.Command{Use: "deploy", ValidArgs: []string{"production"}}
				root.AddCommand(command)
				return cobra.OnlyValidArgs(command, []string{""})
			},
		},
		{
			name:     "Cobra minimum argument count",
			expected: "requires at least 2 arg(s), only received 1",
			produceError: func() error {
				return cobra.MinimumNArgs(2)(&cobra.Command{Use: "api"}, []string{"apps"})
			},
		},
		{
			name:     "Cobra maximum argument count",
			expected: "accepts at most 1 arg(s), received 2",
			produceError: func() error {
				return cobra.MaximumNArgs(1)(&cobra.Command{Use: "api"}, []string{"apps", "extra"})
			},
		},
		{
			name:     "Cobra exact argument count used by API command",
			expected: "accepts 1 arg(s), received 0",
			produceError: func() error {
				return cobra.ExactArgs(1)(&cobra.Command{Use: "api"}, nil)
			},
		},
		{
			name:     "Cobra argument count range",
			expected: "accepts between 1 and 2 arg(s), received 3",
			produceError: func() error {
				return cobra.RangeArgs(1, 2)(&cobra.Command{Use: "api"}, []string{"one", "two", "three"})
			},
		},
		{
			name:     "Cobra required flag",
			expected: `required flag(s) "token" not set`,
			produceError: func() error {
				command := newUsageTestCommand()
				command.Flags().String("token", "", "token")
				if err := command.MarkFlagRequired("token"); err != nil {
					return err
				}
				return command.Execute()
			},
		},
		{
			name:     "Cobra flags required together",
			expected: "if any flags in the group [password username] are set they must all be set; missing [password]",
			produceError: func() error {
				command := newUsageTestCommand()
				command.Flags().String("username", "", "username")
				command.Flags().String("password", "", "password")
				command.MarkFlagsRequiredTogether("password", "username")
				command.SetArgs([]string{"--username", "user"})
				return command.Execute()
			},
		},
		{
			name:     "Cobra one flag required",
			expected: "at least one of the flags in the group [json yaml] is required",
			produceError: func() error {
				command := newUsageTestCommand()
				command.Flags().Bool("json", false, "json")
				command.Flags().Bool("yaml", false, "yaml")
				command.MarkFlagsOneRequired("json", "yaml")
				return command.Execute()
			},
		},
		{
			name:     "Cobra mutually exclusive flags",
			expected: "if any flags in the group [json yaml] are set none of the others can be; [json yaml] were all set",
			produceError: func() error {
				command := newUsageTestCommand()
				command.Flags().Bool("json", false, "json")
				command.Flags().Bool("yaml", false, "yaml")
				command.MarkFlagsMutuallyExclusive("json", "yaml")
				command.SetArgs([]string{"--json", "--yaml"})
				return command.Execute()
			},
		},
		{
			name:     "pflag unknown long flag containing spaces",
			expected: "unknown flag: --feature mode",
			produceError: func() error {
				return newPFlagSet().Parse([]string{"--feature mode"})
			},
		},
		{
			name:     "pflag unknown shorthand flag",
			expected: "unknown shorthand flag: 'x' in -x",
			produceError: func() error {
				return newPFlagSet().Parse([]string{"-x"})
			},
		},
		{
			name:     "pflag missing long flag argument",
			expected: "flag needs an argument: --app-id",
			produceError: func() error {
				flags := newPFlagSet()
				flags.String("app-id", "", "app ID")
				return flags.Parse([]string{"--app-id"})
			},
		},
		{
			name:     "pflag invalid empty argument",
			expected: `invalid argument "" for "--count" flag: strconv.ParseInt: parsing "": invalid syntax`,
			produceError: func() error {
				flags := newPFlagSet()
				flags.Int("count", 0, "count")
				return flags.Parse([]string{"--count="})
			},
		},
		{
			name:     "pflag bad syntax",
			expected: "bad flag syntax: ---count",
			produceError: func() error {
				return newPFlagSet().Parse([]string{"---count"})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.produceError()
			if err == nil {
				t.Fatal("upstream API returned nil error")
			}
			if err.Error() != test.expected {
				t.Fatalf("upstream error = %q, want %q", err.Error(), test.expected)
			}
			if !isCLIUsageErrorMessage(err.Error()) {
				t.Errorf("isCLIUsageErrorMessage(%q) = false, want true", err)
			}
			if !IsUserError(&sentry.Event{Message: err.Error()}) {
				t.Errorf("IsUserError(%q) = false, want true", err)
			}
		})
	}
}

func TestIsCLIUsageErrorMessage_OtherUpstreamMessages(t *testing.T) {
	tests := []string{
		"unable to find a command for arguments: [deployy]",
		"Error while parsing flags from args [--count many]: invalid argument \"many\" for \"--count\" flag: strconv.ParseInt: parsing \"many\": invalid syntax",
	}

	for _, message := range tests {
		t.Run(message, func(t *testing.T) {
			if !isCLIUsageErrorMessage(message) {
				t.Errorf("isCLIUsageErrorMessage(%q) = false, want true", message)
			}
		})
	}
}

func TestIsUserError_UntracedInternalMessagesAreRetained(t *testing.T) {
	tests := []string{
		"database migration validation failed after schema introspection",
		"creating cache record: key already exists in internal index",
		"required flag metadata missing from generated request",
		"remote capability response contains unknown flag data",
		"invalid argument returned by storage adapter",
		`required flag(s) "token" not set while loading server configuration`,
		`invalid argument "value" caused an internal database failure`,
		"if any flags in the group parser crashes, restart the worker",
	}

	for _, message := range tests {
		t.Run(message, func(t *testing.T) {
			if isCLIUsageErrorMessage(message) {
				t.Errorf("isCLIUsageErrorMessage(%q) = true, want false", message)
			}
			if IsUserError(&sentry.Event{Message: message}) {
				t.Errorf("IsUserError(%q) = true, want false", message)
			}
		})
	}
}

func newUsageTestCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "ship",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return nil
		},
	}
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	return command
}

func newPFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("ship", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.SetInterspersed(true)
	return flags
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
