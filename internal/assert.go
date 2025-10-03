package internal

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// AssertError asserts that an error matches the expected error string.
// If wantErr is empty, it expects no error. Otherwise, it expects the error
// message to exactly match wantErr.
func AssertError(t *testing.T, wantErr string, err error) {
	t.Helper()
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	if diff := cmp.Diff(wantErr, errStr); diff != "" {
		t.Errorf("error mismatch (-want +got):\n%s", diff)
	}
}

// Assert asserts that two values are equal.
func Assert(t *testing.T, what string, want, got any) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", what, diff)
	}
}
