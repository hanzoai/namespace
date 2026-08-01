// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import "context"

// A namespace names the entity a piece of work acts for, so it has to travel
// with that work: the request handler knows it, and the store that opens a
// database or builds a key is where it has to be applied. Passing it through
// every call between them means one missed parameter reads another entity's
// data, so it rides the context instead.
//
// Every service does this, so the carrier lives here rather than being
// reinvented per service — a service-local copy is how two services end up
// disagreeing about what an unset namespace means.

type ctxKey struct{}

// NewContext returns a copy of ctx carrying ns. A zero namespace is not
// carried: it names nothing, so storing it would turn "never set" into
// "set to nothing" and hide the mistake at the point where it matters.
func NewContext(ctx context.Context, ns Namespace) context.Context {
	if ns.IsZero() {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, ns)
}

// FromContext returns the namespace ctx carries. ok is false when nothing set
// one, which callers should treat as an error rather than a licence to read
// everything — see IsSystem for work that legitimately spans entities.
func FromContext(ctx context.Context) (ns Namespace, ok bool) {
	ns, ok = ctx.Value(ctxKey{}).(Namespace)
	return ns, ok && !ns.IsZero()
}

// IsSystem reports whether ctx acts for the system namespace, which spans
// every entity. Reserve it for work with no requesting user — schedulers,
// sweeps, migrations — and never derive it from request input.
func IsSystem(ctx context.Context) bool {
	ns, ok := FromContext(ctx)
	return ok && ns.Kind() == KindSystem
}
