package toolerr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"infrastructure-mcp/internal/grafana"
	"infrastructure-mcp/internal/inventory"
	"infrastructure-mcp/internal/ssh"
)

func TestWrap_Nil(t *testing.T) {
	if Wrap(nil) != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestWrap_Classification(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCategory Category
		wantRetry    bool
	}{
		{"not found", inventory.ErrNotFound, CategoryNotFound, false},
		{"no credentials", ssh.ErrNoCredentials, CategoryAuth, false},
		{"no host key verification", ssh.ErrNoHostKeyVerification, CategoryAuth, false},
		{"grafana unauthorized", grafana.ErrUnauthorized, CategoryAuth, false},
		{"context deadline", context.DeadlineExceeded, CategoryTimeout, true},
		{"context cancelled", context.Canceled, CategoryTimeout, true},
		{"network error", &net.DNSError{IsTimeout: true, Err: "boom"}, CategoryNetwork, true},
		{"invalid input", ErrInvalidInput, CategoryInvalidInput, false},
		{"unknown error", errors.New("something broke"), CategoryInternal, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := Wrap(tt.err)
			var te *Error
			if !errors.As(wrapped, &te) {
				t.Fatalf("Wrap() did not return a *toolerr.Error: %v", wrapped)
			}
			if te.Category != tt.wantCategory {
				t.Errorf("Category = %v, want %v", te.Category, tt.wantCategory)
			}
			if te.Retryable != tt.wantRetry {
				t.Errorf("Retryable = %v, want %v", te.Retryable, tt.wantRetry)
			}
			if te.Message == "" {
				t.Error("expected non-empty message")
			}
			if !errors.Is(wrapped, tt.err) {
				t.Error("expected Unwrap chain to preserve the original error for errors.Is")
			}
		})
	}
}

func TestWrap_Idempotent(t *testing.T) {
	once := Wrap(inventory.ErrNotFound)
	twice := Wrap(once)
	if once != twice {
		t.Error("wrapping an already-wrapped error should return it unchanged")
	}
}

func TestError_JSONShape(t *testing.T) {
	wrapped := Wrap(ssh.ErrNoCredentials)
	var payload struct {
		Message        string `json:"message"`
		Recommendation string `json:"recommendation"`
		Retryable      bool   `json:"retryable"`
		Category       string `json:"category"`
	}
	if err := json.Unmarshal([]byte(wrapped.Error()), &payload); err != nil {
		t.Fatalf("Error() did not produce valid JSON: %v (%s)", err, wrapped.Error())
	}
	if payload.Category != string(CategoryAuth) {
		t.Errorf("category = %q, want %q", payload.Category, CategoryAuth)
	}
	if payload.Recommendation == "" {
		t.Error("expected a non-empty recommendation")
	}
}
