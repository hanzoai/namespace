// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import "testing"

// TestKeyIsTheStringOnDisk is a GOLDEN test, and the golden values are not
// aspirational: every one of them was produced by the implementation that has
// been writing these files in production, captured before the code moved here.
//
// It is a table of literals rather than a property, because a property test
// proves the new code is self-consistent and this has to prove something else —
// that the new code agrees with the old one to the byte. These strings are
// directory names on a live volume and object keys in a live bucket. A rendering
// that changed by one character would not fail; it would silently open an empty
// database next to an org's real one.
//
// The paths are the POSIX rendering, which is what every host in the fleet runs.
func TestKeyIsTheStringOnDisk(t *testing.T) {
	for _, tc := range []struct{ org, project, subsystem, key, path string }{
		// A clean slug is itself: the common case reads as the org's own name.
		{"acme", "", "git", "orgs/acme/git.db", "/data/orgs/acme/git.db"},
		{"acme", "", "kms", "orgs/acme/kms.db", "/data/orgs/acme/kms.db"},
		{"9lives", "", "git", "orgs/9lives/git.db", "/data/orgs/9lives/git.db"},

		// A project is a real nested segment, including the one called "default".
		{"acme", "web", "tracker", "orgs/acme/projects/web/tracker.db", "/data/orgs/acme/projects/web/tracker.db"},
		{"acme", "default", "tracker", "orgs/acme/projects/default/tracker.db", "/data/orgs/acme/projects/default/tracker.db"},

		// The folding path: case, "_" and "." each fold to the same shape and are
		// then held apart by a hash of the raw bytes. "acme"/"ACME"/"Acme" are
		// three orgs and three files.
		{"ACME", "", "git", "orgs/acme-a8308e8e2e4b7330/git.db", "/data/orgs/acme-a8308e8e2e4b7330/git.db"},
		{"Acme", "", "git", "orgs/acme-37036cd8f9746d33/git.db", "/data/orgs/acme-37036cd8f9746d33/git.db"},
		{"acme_corp", "", "git", "orgs/acme-corp-cca8c7942f8c15a2/git.db", "/data/orgs/acme-corp-cca8c7942f8c15a2/git.db"},
		{"acme.corp", "", "git", "orgs/acme-corp-80a1d5cc623b165f/git.db", "/data/orgs/acme-corp-80a1d5cc623b165f/git.db"},
		{"MaxPower", "", "kms", "orgs/maxpower-a3e10398a4a44f2e/kms.db", "/data/orgs/maxpower-a3e10398a4a44f2e/kms.db"},
		{"x-y_z", "", "git", "orgs/x-y-z-988a16a3150c2188/git.db", "/data/orgs/x-y-z-988a16a3150c2188/git.db"},
		{"über", "", "git", "orgs/ber-b51c854170449c38/git.db", "/data/orgs/ber-b51c854170449c38/git.db"},
		{"Ünïcödé", "", "git", "orgs/n-c-d-39af95d07d82b5d6/git.db", "/data/orgs/n-c-d-39af95d07d82b5d6/git.db"},

		// Over 32 bytes: the fold truncates and the hash keeps two long names
		// that share a prefix apart.
		{"this-is-a-very-long-org-name-that-exceeds-thirty-two-characters", "", "git",
			"orgs/this-is-a-very-long-org-name-tha-d6c796465b2f097c/git.db",
			"/data/orgs/this-is-a-very-long-org-name-tha-d6c796465b2f097c/git.db"},
		{"THIS-IS-A-VERY-LONG-ORG-NAME-THAT-EXCEEDS-THIRTY-TWO-CHARACTERS", "", "git",
			"orgs/this-is-a-very-long-org-name-tha-e1e451cbed85af65/git.db",
			"/data/orgs/this-is-a-very-long-org-name-tha-e1e451cbed85af65/git.db"},

		// A name shaped like this function's own output is re-suffixed; one that
		// merely resembles it (a "g" is not hex) is not.
		{"foo-0123456789abcdef", "", "git",
			"orgs/foo-0123456789abcdef-1225a522cf51bf26/git.db",
			"/data/orgs/foo-0123456789abcdef-1225a522cf51bf26/git.db"},
		{"foo-0123456789abcdeg", "", "git", "orgs/foo-0123456789abcdeg/git.db", "/data/orgs/foo-0123456789abcdeg/git.db"},

		// A name written as a path becomes ONE segment and stays distinct from an
		// org actually called "etc".
		{"../etc", "", "git", "orgs/etc-f7f9121fdb53d8de/git.db", "/data/orgs/etc-f7f9121fdb53d8de/git.db"},
		{"a/b", "", "git", "orgs/a-b-c14cddc033f64b9d/git.db", "/data/orgs/a-b-c14cddc033f64b9d/git.db"},
		{"..%2f..%2fvictim", "", "git",
			"orgs/2f---2fvictim-26573cabaaa3ba65/git.db",
			"/data/orgs/2f---2fvictim-26573cabaaa3ba65/git.db"},

		// The project half folds by the same rule, in the group slot.
		{"acme", "Alpha", "git",
			"orgs/acme/projects/alpha-b1a96dd646bccaa2/git.db",
			"/data/orgs/acme/projects/alpha-b1a96dd646bccaa2/git.db"},
		{"acme", "beta.1", "tracker",
			"orgs/acme/projects/beta-1-e6156cf33db02273/tracker.db",
			"/data/orgs/acme/projects/beta-1-e6156cf33db02273/tracker.db"},
		{"acme", "../etc", "git",
			"orgs/acme/projects/etc-f7f9121fdb53d8de/git.db",
			"/data/orgs/acme/projects/etc-f7f9121fdb53d8de/git.db"},
		{"Acme", "Alpha", "kms",
			"orgs/acme-37036cd8f9746d33/projects/alpha-b1a96dd646bccaa2/kms.db",
			"/data/orgs/acme-37036cd8f9746d33/projects/alpha-b1a96dd646bccaa2/kms.db"},
	} {
		ns, err := OrgProject(tc.org, tc.project)
		if err != nil {
			t.Errorf("OrgProject(%q, %q): %v", tc.org, tc.project, err)
			continue
		}
		key, err := Key(ns, tc.subsystem)
		if err != nil {
			t.Errorf("Key(%q, %q): %v", ns, tc.subsystem, err)
			continue
		}
		if key != tc.key {
			t.Errorf("Key(OrgProject(%q, %q), %q) = %q, want %q", tc.org, tc.project, tc.subsystem, key, tc.key)
		}
		p, err := Path("/data", ns, tc.subsystem)
		if err != nil {
			t.Errorf("Path(%q, %q): %v", ns, tc.subsystem, err)
			continue
		}
		if p != tc.path {
			t.Errorf("Path(\"/data\", OrgProject(%q, %q), %q) = %q, want %q", tc.org, tc.project, tc.subsystem, p, tc.path)
		}
	}
}

// TestKeySystemPartition pins the deployment's own partition. Its directory
// carries a "_", which Sanitize never emits, so no entity's namespace can be
// rendered onto it — the reason it survived the move to a distinct KIND
// unchanged, and the reason the files already on disk keep their path.
func TestKeySystemPartition(t *testing.T) {
	for _, tc := range []struct{ subsystem, key, path string }{
		{"kms", "orgs/_platform/kms.db", "/data/orgs/_platform/kms.db"},
		{"git", "orgs/_platform/git.db", "/data/orgs/_platform/git.db"},
	} {
		key, err := Key(System(), tc.subsystem)
		if err != nil {
			t.Fatalf("Key(system, %q): %v", tc.subsystem, err)
		}
		if key != tc.key {
			t.Errorf("Key(system, %q) = %q, want %q", tc.subsystem, key, tc.key)
		}
		p, err := Path("/data", System(), tc.subsystem)
		if err != nil {
			t.Fatalf("Path(system, %q): %v", tc.subsystem, err)
		}
		if p != tc.path {
			t.Errorf("Path(\"/data\", system, %q) = %q, want %q", tc.subsystem, p, tc.path)
		}
	}
	// This layout has no nesting under the system partition, so a group on the
	// system namespace does not reach the rendering: system/notices and system
	// name the SAME file. That is what the code on disk does today and it is
	// pinned here rather than left to be discovered — the instance's own data is
	// kept apart by SUBSYSTEM here, not by group.
	if got, err := Key(System().WithGroup(MustGroup("notices")), "kms"); err != nil || got != "orgs/_platform/kms.db" {
		t.Errorf("Key(system/notices, %q) = %q, %v; want orgs/_platform/kms.db", "kms", got, err)
	}
}

// TestKeyRefuses pins the fail-closed half. Each of these would, if it returned
// a string, be a file quietly shared by callers who meant different things.
func TestKeyRefuses(t *testing.T) {
	if _, err := Key(MustOrg("acme"), ""); err == nil {
		t.Error("an empty subsystem must error — it would name orgs/acme/.db")
	}
	if _, err := Key(Namespace{}, "git"); err == nil {
		t.Error("the zero namespace must error — it names no database")
	}
	for _, ns := range []Namespace{
		MustUser("alice"),
		MustRepo("r1"),
		MustUser("alice").WithGroup(MustGroup("issues")),
	} {
		if _, err := Key(ns, "git"); err == nil {
			t.Errorf("Key(%q) must error: this layout has no place for a %s namespace", ns, ns.Kind())
		}
	}
	if _, err := Path("", System(), "git"); err == nil {
		t.Error("an empty dir must error — the store would move with the working directory")
	}
}
