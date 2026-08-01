// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import "strings"

// Of returns the namespace naming the data that belongs to subject — the key an
// account holds within its ledger (github.com/hanzoai/account Account.Subject):
// a bare slug for an org, "<org>/<name>" for a person or a project.
//
// It is a total function of that one string, deliberately. The alternative is a
// table mapping each account kind to a namespace kind, and a table is somewhere
// the two vocabularies can drift; deriving both the wallet and the database from
// the same value means they cannot. If a request bills "hanzo/alice" it reads and
// writes hanzo/alice's data, by construction rather than by agreement.
//
// The org half is kept rather than collapsing a person to a bare name, so
// "acme/alice" and "globex/alice" stay different namespaces — and therefore
// different databases — even where two orgs use the same username.
//
// Validation is inherited: the org must be a legal id and the name a legal
// group, so a subject with a path separator, a traversal, or an oversized
// segment is rejected here rather than reaching a filesystem or a key.
func Of(subject string) (Namespace, error) {
	org, name, hasName := strings.Cut(subject, "/")

	ns, err := Org(org)
	if err != nil {
		return Namespace{}, err
	}
	if !hasName {
		return ns, nil
	}

	g, err := NewGroup(name)
	if err != nil {
		return Namespace{}, err
	}
	return ns.WithGroup(g), nil
}
