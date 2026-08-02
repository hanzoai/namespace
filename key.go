// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import (
	"fmt"
	"path"
	"path/filepath"
)

// orgsRoot is the single directory every namespace's files live under, and
// platformSlug is how the system namespace renders inside it — a leading "_",
// which Sanitize never emits, so the deployment's own partition cannot collide
// with an entity's however the slugger changes.
//
// Both are spelled once, here, because these strings are on disk and in object
// storage: a second spelling is a second answer to "where is this store", and
// the files that already exist only have one place.
const (
	orgsRoot     = "orgs"
	platformSlug = "_platform"
)

// Key renders a namespace as the place one subsystem's file for it lives:
// relative to a data directory on disk, and byte-identically the object key
// durability ships it to. One rendering, two consumers, so the local file and
// the remote slot are the same name by construction.
//
//	org/{id}                 →  orgs/{id}/{subsystem}.db
//	org/{id}/{group}         →  orgs/{id}/projects/{group}/{subsystem}.db
//	system                   →  orgs/_platform/{subsystem}.db
//
// The subsystem is a separate argument rather than part of the namespace
// because one store is one subsystem: which subsystem is a property of the
// registry that opens the file, not of the entity the file belongs to.
//
// Every other kind is an error. User and repo namespaces are real namespaces,
// but this layout has no place for them, and inventing one silently is how a
// second convention starts.
func Key(ns Namespace, subsystem string) (string, error) {
	if subsystem == "" {
		return "", fmt.Errorf("namespace: empty subsystem")
	}
	switch ns.Kind() {
	case KindSystem:
		return path.Join(orgsRoot, platformSlug, subsystem+".db"), nil
	case KindOrg:
		if g := ns.Group().String(); g != "" {
			return path.Join(orgsRoot, ns.ID(), "projects", g, subsystem+".db"), nil
		}
		return path.Join(orgsRoot, ns.ID(), subsystem+".db"), nil
	case "":
		return "", fmt.Errorf("namespace: the zero namespace names no database")
	default:
		return "", fmt.Errorf("namespace: no database layout for a %s namespace", ns.Kind())
	}
}

// Path is Key resolved against a deployment's data directory, in the host's own
// separator. An empty dir is an error rather than a relative path, because a
// store that opened relative to whatever the working directory happened to be
// would be a different file per invocation.
func Path(dir string, ns Namespace, subsystem string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("namespace: empty dir")
	}
	key, err := Key(ns, subsystem)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.FromSlash(key)), nil
}
