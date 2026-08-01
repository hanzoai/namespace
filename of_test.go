// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import "testing"

func TestOfOrgSubject(t *testing.T) {
	ns, err := Of("acme")
	if err != nil {
		t.Fatal(err)
	}
	if ns.Kind() != KindOrg || ns.ID() != "acme" || ns.Group().String() != "" {
		t.Fatalf("got %q", ns)
	}
}

func TestOfPersonSubjectKeepsBothHalves(t *testing.T) {
	ns, err := Of("hanzo/alice")
	if err != nil {
		t.Fatal(err)
	}
	if ns.ID() != "hanzo" || ns.Group().String() != "alice" {
		t.Fatalf("got id=%q group=%q", ns.ID(), ns.Group())
	}
}

// Two orgs may use the same username; collapsing to a bare name would put both
// in one database.
func TestOfKeepsUsersInDifferentOrgsApart(t *testing.T) {
	a, err := Of("acme/alice")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Of("globex/alice")
	if err != nil {
		t.Fatal(err)
	}
	if a.String() == b.String() {
		t.Fatalf("acme/alice and globex/alice share a namespace: %q", a)
	}
}

// A person's namespace must differ from their org's, or every personal account
// in the signup org shares one database.
func TestOfPersonDiffersFromItsOrg(t *testing.T) {
	person, _ := Of("hanzo/alice")
	org, _ := Of("hanzo")
	if person.String() == org.String() {
		t.Fatalf("person and org share a namespace: %q", org)
	}
}

func TestOfRejectsBadSubjects(t *testing.T) {
	for _, s := range []string{"", "/", "acme/", "/alice", "acme/alice/extra", "../etc", "acme/../etc", "a b"} {
		if ns, err := Of(s); err == nil {
			t.Fatalf("Of(%q) = %q, want an error", s, ns)
		}
	}
}
