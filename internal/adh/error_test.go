package adh_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
)

func TestErrorString(t *testing.T) {
	tests := []struct {
		name string
		err  *adh.Error
		want string
	}{
		{
			name: "leaf with message",
			err:  &adh.Error{Code: adh.EINVALID, Message: "score must be numeric"},
			want: "score must be numeric",
		},
		{
			name: "wrapper over leaf",
			err:  &adh.Error{Op: "state.Store.Close", Err: &adh.Error{Message: "no such arc"}},
			want: "state.Store.Close: no such arc",
		},
		{
			name: "nested wrappers",
			err: &adh.Error{
				Op:  "cmd.gate.exec",
				Err: &adh.Error{Op: "state.Store.Close", Err: errors.New("disk full")},
			},
			want: "cmd.gate.exec: state.Store.Close: disk full",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{
			name: "leaf code",
			err:  &adh.Error{Code: adh.ENOTFOUND, Message: "gone"},
			want: adh.ENOTFOUND,
		},
		{
			name: "code through wrapper",
			err:  &adh.Error{Op: "a.B.C", Err: &adh.Error{Code: adh.ECONFLICT, Message: "busy"}},
			want: adh.ECONFLICT,
		},
		{name: "foreign error", err: errors.New("boom"), want: adh.EINTERNAL},
		{
			name: "wrapper over foreign error",
			err:  &adh.Error{Op: "a.B.C", Err: fmt.Errorf("wrap: %w", errors.New("boom"))},
			want: adh.EINTERNAL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adh.ErrorCode(tt.err); got != tt.want {
				t.Errorf("ErrorCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "leaf message", err: &adh.Error{Code: adh.EINVALID, Message: "bad"}, want: "bad"},
		{name: "foreign error", err: errors.New("boom"), want: "an internal error occurred"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adh.ErrorMessage(tt.err); got != tt.want {
				t.Errorf("ErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := &adh.Error{Op: "a.B.C", Err: sentinel}
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is did not find the wrapped sentinel")
	}
}
