// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import "fmt"

// OrgProject names the database an org's records live in — or, when project is
// non-empty, the database that org's records for one project live in.
//
// It is the door for names a PERSON chose. Org and MustOrg take an id that is
// already legal; OrgProject takes an IAM org and a project as they are written
// down, folds each through Sanitize — the one injective slugger — and only then
// builds the namespace. So two distinct orgs can never share a database: a case
// fold on a case-insensitive filesystem, or a "-"/"." fold, would otherwise
// collapse them onto one file.
//
// org MUST be a value the caller has already authenticated — a validated claim,
// or a server-side resolution the caller states as its contract — never a raw
// request body or header. Sanitize makes a hostile name HARMLESS (it cannot
// escape its directory) but it cannot make it AUTHORISED, and those are
// different questions answered in different places.
//
// The project rides in the GROUP slot, which is what a group is for: which of
// this entity's related types this file holds.
//
// It is stricter than folding alone in exactly one place, and the strictness is
// the point. Sanitize emits a leading "-" for the two names that fold to
// nothing printable (one that folds away entirely, such as "!!!", and one
// literally written "-abc", which takes the identity fast path). A namespace
// segment must begin with [a-z0-9], so those error here rather than quietly
// minting a directory named "-<hash>".
func OrgProject(org, project string) (Namespace, error) {
	slug := Sanitize(org)
	if slug == "" {
		return Namespace{}, fmt.Errorf("namespace: invalid org %q", org)
	}
	ns, err := Org(slug)
	if err != nil {
		return Namespace{}, fmt.Errorf("namespace: invalid org %q: %w", org, err)
	}
	if project == "" {
		return ns, nil
	}
	projSlug := Sanitize(project)
	if projSlug == "" {
		return Namespace{}, fmt.Errorf("namespace: invalid project %q", project)
	}
	g, err := NewGroup(projSlug)
	if err != nil {
		return Namespace{}, fmt.Errorf("namespace: invalid project %q: %w", project, err)
	}
	return ns.WithGroup(g), nil
}

// MustOrgProject is OrgProject for names fixed in the source — a test, a seed, a
// constant in a migration. It panics, which is correct for a value that is wrong
// before the program runs and wrong for anything from a request.
func MustOrgProject(org, project string) Namespace { return must(OrgProject(org, project)) }
