package domain

import (
	"errors"
	"testing"
)

func TestErrNotFound(t *testing.T) {
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("ErrNotFound should equal itself")
	}
}

func TestErrExists(t *testing.T) {
	if !errors.Is(ErrExists, ErrExists) {
		t.Error("ErrExists should equal itself")
	}
}

func TestErrInvalidName(t *testing.T) {
	if !errors.Is(ErrInvalidName, ErrInvalidName) {
		t.Error("ErrInvalidName should equal itself")
	}
}

func TestErrInvalidPort(t *testing.T) {
	if !errors.Is(ErrInvalidPort, ErrInvalidPort) {
		t.Error("ErrInvalidPort should equal itself")
	}
}

func TestErrEmptyField(t *testing.T) {
	if !errors.Is(ErrEmptyField, ErrEmptyField) {
		t.Error("ErrEmptyField should equal itself")
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		err    error
		expect string
	}{
		{ErrNotFound, "server not found"},
		{ErrExists, "server already exists"},
		{ErrInvalidName, "invalid server name"},
		{ErrInvalidPort, "invalid port number"},
		{ErrEmptyField, "empty field not allowed"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.expect {
			t.Errorf("expected %q, got %q", tt.expect, tt.err.Error())
		}
	}
}