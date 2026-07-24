// Package toolerr implements the structured error contract required by
// README.md's Error Handling section: MCP tools must never surface a raw
// Go error, only a message, recommendation, retryable flag, and category.
package toolerr

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"infrastructure-mcp/internal/grafana"
	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/ssh"
)

// Category classifies the kind of failure, so a caller (human or AI agent)
// can decide how to react without parsing free text.
type Category string

const (
	CategoryNotFound     Category = "not_found"
	CategoryAuth         Category = "auth"
	CategoryNetwork      Category = "network"
	CategoryTimeout      Category = "timeout"
	CategoryInvalidInput Category = "invalid_input"
	CategoryInternal     Category = "internal"
)

// ErrInvalidInput is a sentinel for input-validation failures (e.g. a
// container name/ID that fails the docker naming pattern). Adapters wrap
// it with fmt.Errorf("...: %w", ErrInvalidInput) so Wrap can classify it
// without string matching.
var ErrInvalidInput = errors.New("toolerr: invalid input")

// Error is the structured shape every tool returns on failure. Its
// Error() method renders as JSON, so it survives the MCP SDK's default
// error packing (which calls Error() to produce the result's text
// content) without losing structure.
type Error struct {
	Message        string   `json:"message"`
	Recommendation string   `json:"recommendation,omitempty"`
	Retryable      bool     `json:"retryable"`
	Category       Category `json:"category"`

	cause error
}

func (e *Error) Error() string {
	b, err := json.Marshal(e)
	if err != nil {
		// Marshaling a struct of strings/bool never fails; this is
		// unreachable in practice but keeps Error() total.
		return e.Message
	}
	return string(b)
}

// Unwrap allows errors.Is/As to see through to the original cause.
func (e *Error) Unwrap() error { return e.cause }

// Wrap classifies err into the structured contract. nil in, nil out.
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	var alreadyWrapped *Error
	if errors.As(err, &alreadyWrapped) {
		return err
	}

	switch {
	case errors.Is(err, ErrInvalidInput):
		return &Error{
			Message:        err.Error(),
			Recommendation: "Fix the request parameters and try again.",
			Retryable:      false,
			Category:       CategoryInvalidInput,
			cause:          err,
		}
	case errors.Is(err, inventory.ErrNotFound):
		return &Error{
			Message:        err.Error(),
			Recommendation: "Check the target name against the list_servers tool's output.",
			Retryable:      false,
			Category:       CategoryNotFound,
			cause:          err,
		}
	case errors.Is(err, ssh.ErrNoCredentials):
		return &Error{
			Message:        err.Error(),
			Recommendation: "Set a key or password for this server in the inventory, or make an SSH agent available.",
			Retryable:      false,
			Category:       CategoryAuth,
			cause:          err,
		}
	case errors.Is(err, grafana.ErrUnauthorized):
		return &Error{
			Message:        err.Error(),
			Recommendation: "Check the token/user/password for this Grafana instance in the inventory.",
			Retryable:      false,
			Category:       CategoryAuth,
			cause:          err,
		}
	case errors.Is(err, ssh.ErrNoHostKeyVerification):
		return &Error{
			Message:        err.Error(),
			Recommendation: "Add the host's key to known_hosts, or explicitly enable insecure host-key mode for lab use.",
			Retryable:      false,
			Category:       CategoryAuth,
			cause:          err,
		}
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return &Error{
			Message:        err.Error(),
			Recommendation: "Retry the request; if it keeps timing out, check connectivity to the target.",
			Retryable:      true,
			Category:       CategoryTimeout,
			cause:          err,
		}
	default:
		var netErr net.Error
		if errors.As(err, &netErr) {
			return &Error{
				Message:        err.Error(),
				Recommendation: "Retry the request; if it persists, check network connectivity to the target.",
				Retryable:      true,
				Category:       CategoryNetwork,
				cause:          err,
			}
		}
		return &Error{
			Message:   err.Error(),
			Retryable: false,
			Category:  CategoryInternal,
			cause:     err,
		}
	}
}
