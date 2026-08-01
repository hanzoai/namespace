// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ns := MustOrg("acme")
	got, ok := FromContext(NewContext(context.Background(), ns))
	if !ok {
		t.Fatal("namespace did not survive the context")
	}
	if got.String() != ns.String() {
		t.Fatalf("got %q, want %q", got, ns)
	}
}

// Nothing set means nothing set. Reporting ok here would let a caller treat an
// unset namespace as permission to read every entity.
func TestFromContextReportsUnset(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("an unscoped context must report ok=false")
	}
}

// Storing the zero namespace would turn "never set" into "set to nothing".
func TestZeroNamespaceIsNotCarried(t *testing.T) {
	ctx := NewContext(context.Background(), Namespace{})
	if _, ok := FromContext(ctx); ok {
		t.Fatal("the zero namespace must not be carried")
	}
}

func TestReScopingReplaces(t *testing.T) {
	ctx := NewContext(NewContext(context.Background(), MustOrg("a")), MustOrg("b"))
	got, _ := FromContext(ctx)
	if got.ID() != "b" {
		t.Fatalf("got %q, want the inner scope to win", got.ID())
	}
}

func TestIsSystem(t *testing.T) {
	if !IsSystem(NewContext(context.Background(), System())) {
		t.Fatal("system namespace must report IsSystem")
	}
	if IsSystem(NewContext(context.Background(), MustOrg("acme"))) {
		t.Fatal("an org namespace must not report IsSystem")
	}
	if IsSystem(context.Background()) {
		t.Fatal("an unscoped context must not report IsSystem")
	}
}
