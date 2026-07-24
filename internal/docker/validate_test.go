package docker

import (
	"errors"
	"testing"

	"infrastructure-mcp/internal/toolerr"
)

func TestValidateContainerRef(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{"simple name", "web", false},
		{"name with dashes and dots", "my-app.v2_1", false},
		{"hex id", "abc123def456", false},
		{"empty", "", true},
		{"shell metacharacters", "web; rm -rf /", true},
		{"leading dash", "-rf", true},
		{"pipe", "web|cat", true},
		{"dollar substitution", "$(id)", true},
		{"backtick", "`id`", true},
		{"space", "web app", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContainerRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, toolerr.ErrInvalidInput) {
					t.Errorf("expected toolerr.ErrInvalidInput in chain, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
