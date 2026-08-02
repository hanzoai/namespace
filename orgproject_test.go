// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import (
	"path"
	"strings"
	"testing"
)

// TestOrgProjectRefuses pins the door shut on the names that must never become
// a database. Each of these would, if admitted, be a file two distinct entities
// could share or a segment that escapes the data directory.
func TestOrgProjectRefuses(t *testing.T) {
	for _, tc := range []struct{ org, project, why string }{
		{"", "", "an empty org names no entity"},
		{"acme ", "", "a trailing space folds onto acme after any trim"},
		{"acme\u200b", "", "a zero-width rune is invisible in an identifier"},
		{"acme", " ", "a whitespace-only project names no group"},
		{"!!!", "", "an org that folds to nothing would be named by its hash alone"},
		{"-abc", "", "a slug starting with '-' is not a legal segment"},
		{"acme", "!!!", "a project that folds to nothing would be named by its hash alone"},
	} {
		if ns, err := OrgProject(tc.org, tc.project); err == nil {
			t.Errorf("OrgProject(%q, %q) = %q, want an error: %s", tc.org, tc.project, ns, tc.why)
		}
	}
}

// TestOrgProjectNeutralisesTraversal pins what happens to a name that looks
// like a path. It is not refused — Sanitize folds every separator and dot to
// "-" and then disambiguates with a hash of the raw name, so "../etc" becomes
// one safe segment that is still distinct from an org actually called "etc".
// The property that matters is not "rejected" but "cannot leave its directory",
// and asserting the wrong one of those would be a test that passes for the
// wrong reason.
func TestOrgProjectNeutralisesTraversal(t *testing.T) {
	hostile := []string{"../etc", "..", ".", "a/b", "..%2f..%2fvictim", `a\b`}
	for _, org := range hostile {
		ns, err := OrgProject(org, "")
		if err != nil {
			continue // refused outright is also fine
		}
		id := ns.ID()
		if id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
			t.Errorf("OrgProject(%q) = %q, whose id %q can leave its directory", org, ns, id)
		}
		key, err := Key(ns, "widget")
		if err != nil {
			t.Fatalf("Key(%q): %v", ns, err)
		}
		if want := path.Join(orgsRoot, id, "widget.db"); key != want {
			t.Errorf("OrgProject(%q) rendered to %q, want %q", org, key, want)
		}
	}
	for _, project := range hostile {
		ns, err := OrgProject("acme", project)
		if err != nil {
			continue
		}
		if g := ns.Group().String(); g == "." || g == ".." || strings.ContainsAny(g, `/\`) {
			t.Errorf("project %q became group %q, which can leave its directory", project, g)
		}
		if ns.ID() != "acme" {
			t.Errorf("project %q changed the org to %q", project, ns.ID())
		}
	}
}

// TestOrgProjectIsInjective proves the property the whole boundary rests on:
// two distinct orgs never name one database. Sanitize carries it, and Org's own
// case fold cannot undo it, because Sanitize has already lowercased everything
// it emits.
func TestOrgProjectIsInjective(t *testing.T) {
	seen := map[string]string{}
	for _, org := range []string{
		"acme", "ACME", "Acme", "acme-corp", "acme_corp", "acme.corp",
		"team-a", "team.a", "team_a", "platform", "system", "org",
	} {
		ns, err := OrgProject(org, "")
		if err != nil {
			t.Fatalf("OrgProject(%q): %v", org, err)
		}
		if prev, dup := seen[ns.String()]; dup {
			t.Fatalf("orgs %q and %q name the SAME database %q", prev, org, ns)
		}
		seen[ns.String()] = org
	}
	// And no org can reach the deployment's own partition, whatever it is called.
	for _, org := range []string{"_platform", "platform", "system", "_system"} {
		ns, err := OrgProject(org, "")
		if err != nil {
			continue
		}
		if ns == System() {
			t.Fatalf("org %q named the system partition", org)
		}
		if k, err := Key(ns, "kms"); err == nil {
			if p, _ := Key(System(), "kms"); k == p {
				t.Fatalf("org %q rendered to the system partition's file %q", org, k)
			}
		}
	}
}

// TestMustOrgProjectPanics states what the Must form is for: a name fixed in
// the source, wrong before the program runs.
func TestMustOrgProjectPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustOrgProject on an invalid org must panic")
		}
	}()
	MustOrgProject("", "")
}
