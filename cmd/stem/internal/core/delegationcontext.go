package core

import (
	"context"
	"strings"
)

// The Pollen — the Pollinator's bound identity — travels from the
// surface that authenticated it to the execution adapter through the request
// context, and ONLY through the request context.
//
// It is deliberately not a field on any capability input. Those structs are
// decoded from caller-supplied JSON, so a Pollen field would be a Pollen the
// caller could name — and since the Pollinator decides which isolated workspace an
// operation runs in (and which grants apply to it), a caller that could name it
// could claim another subject's workspace. The pollen is bound by the trusted
// launch configuration (the Model Context Protocol connection) or by an
// authenticated header (the Representational State Transfer surface), stamped
// here after authorization, and is unforgeable from request content.
//
// The Core itself never reads it: it passes the context through to the
// injected execution port, exactly as it passes the context to any other port.
// That keeps the Core transport-free — a context value carries no transport
// type — while letting the adapter layer resolve a per-Pollinator workspace.

type pollenKey struct{}

// WithPollen returns a context carrying the authorized Pollen. Surfaces call this AFTER the delegation gate has authorized the
// invocation; a blank pollen is not stored, so a non-delegated call is
// indistinguishable from one that never set it.
func WithPollen(ctx context.Context, pollen string) context.Context {
	trimmed := strings.TrimSpace(pollen)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, pollenKey{}, trimmed)
}

// PollenFromContext returns the authorized Pollen, or ""
// when the invocation is not delegated (a direct command line run, for
// instance). Callers treat "" as "no isolation required": the operator's own
// workspace, which is today's behaviour for a human at a terminal.
func PollenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	pollen, _ := ctx.Value(pollenKey{}).(string)
	return pollen
}
