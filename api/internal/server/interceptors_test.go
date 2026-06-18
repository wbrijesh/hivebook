package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

// TestSanitizeError locks P3-5: server-fault detail is scrubbed before leaving the
// process (code preserved, message generic), while client-error codes pass through
// unchanged so the frontend keeps the messages it acts on.
func TestSanitizeError(t *testing.T) {
	ctx := context.Background()

	// Internal: detail scrubbed, code preserved.
	internal := connect.NewError(connect.CodeInternal, errors.New("pq: relation secret_table does not exist"))
	got := sanitizeError(ctx, "svc/M", internal)
	if connect.CodeOf(got) != connect.CodeInternal {
		t.Fatalf("code must be preserved, got %v", connect.CodeOf(got))
	}
	if strings.Contains(got.Error(), "secret_table") {
		t.Fatalf("internal detail leaked to the client: %v", got)
	}

	// Unavailable: also generic-ized.
	unavail := connect.NewError(connect.CodeUnavailable, errors.New("dial tcp 10.0.0.1:5432: connection refused"))
	got = sanitizeError(ctx, "svc/M", unavail)
	if connect.CodeOf(got) != connect.CodeUnavailable || strings.Contains(got.Error(), "10.0.0.1") {
		t.Fatalf("unavailable detail leaked: %v", got)
	}

	// Default-deny: a code outside the whitelist carrying a raw message is scrubbed,
	// not passed through (this is the leak-by-default a blacklist would have missed).
	aborted := connect.NewError(connect.CodeAborted, errors.New("pq: deadlock detected on table secrets"))
	got = sanitizeError(ctx, "svc/M", aborted)
	if connect.CodeOf(got) != connect.CodeAborted || strings.Contains(got.Error(), "secrets") {
		t.Fatalf("non-whitelisted code must be scrubbed: %v", got)
	}

	// Client-error codes pass through unchanged (frontend-meaningful messages).
	for _, code := range []connect.Code{
		connect.CodeNotFound, connect.CodeFailedPrecondition,
		connect.CodePermissionDenied, connect.CodeUnauthenticated, connect.CodeInvalidArgument,
		connect.CodeAlreadyExists, connect.CodeResourceExhausted,
	} {
		in := connect.NewError(code, errors.New("a meaningful client message"))
		if out := sanitizeError(ctx, "svc/M", in); out != in {
			t.Fatalf("%v must pass through unchanged", code)
		}
	}

	// nil stays nil.
	if sanitizeError(ctx, "svc/M", nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
