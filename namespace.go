// Package namespace names the one database an entity's data lives in.
//
// The fleet stores OLTP data as one SQLite file per entity — an org, a user, a
// repository — rather than one per tenant, so that a file stays small and
// several can be opened at once. A namespace is the name of one of those files.
//
// It is a value with constructors and not a string with a validator, because
// the string form IS a path: it becomes <dir>/<namespace>.db locally and a key
// prefix in object storage. A name that a validator lets through late is a name
// that has already been written down somewhere by then. Here there is no route
// to a Namespace that does not go through a constructor, so the illegal ones do
// not exist to be caught.
//
// This package is what a namespace IS. Resolving one to an open handle —
// materialise, open, cache, evict — is hanzoai/orm/db.Namespaces, and it is the
// only other thing allowed an opinion about namespaces.
package namespace

import (
	"errors"
	"fmt"
	"strings"
)

// Kind is the sort of entity whose data a namespace holds.
//
// The set is closed and adding to it is a patch to this file, on purpose: which
// entities own their own data is one fleet-wide fact, and a fact with two homes
// is a fact that disagrees with itself. It is also the reason Kind cannot be
// injected — no exported function takes one — so a Kind literal that names
// nothing can never reach a Namespace.
type Kind string

const (
	KindOrg    Kind = "org"
	KindUser   Kind = "user"
	KindRepo   Kind = "repo"
	KindSystem Kind = "system"
)

// maxSegment bounds an id or a group. A filesystem component stops at 255 bytes
// and an S3 key at 1024; 64 clears both with room for the kind, the group and
// the ".db", and still holds a UUID (36) or a ULID (26) with most of it spare.
const maxSegment = 64

// Namespace names one database: an entity kind, that entity's id, and
// optionally a group naming which of its related entity types this file holds.
//
// It holds its own canonical string rather than the three parts, so String is
// free and the map key hashes as one string — Namespaces keys every open handle
// by this value and looks one up on every request. The parts are derived on the
// rare path instead, which is the right way round.
//
// The zero Namespace is not a namespace. Go gives every struct a zero value and
// no encapsulation takes that away, so it is defined rather than left to be
// discovered: IsZero reports it, String returns "", MarshalText refuses it, and
// whatever turns a namespace into a path must refuse it too. A zero value that
// resolved to a file would be one file quietly shared by every caller who
// forgot to set one — the worst failure this package exists to prevent, so it
// is the one Go forces us to state explicitly.
type Namespace struct{ s string }

// Group names a set of related entity types kept in a file of their own, so
// that an entity's data is several small databases and not one that grows
// without bound: a repository's issues need not be opened to read its settings.
//
// It is a type rather than a string because a group is CODE where an id is
// DATA. A package declares the groups it owns once —
//
//	var Issues = namespace.MustGroup("issues")
//
// — while an id arrives with a request. That is the whole reason the two are
// built differently: MustGroup fails at init or never, so there is no error to
// handle at a call site that cannot fail.
//
// The zero Group means no group, i.e. the entity's own data. That is a
// definition, not a hole: ns.WithGroup(Group{}) is ns.
type Group struct{ s string }

// Org, User and Repo name one entity's database.
//
// The id is the entity's STABLE identity, never a human-facing name. A
// namespace is a path in object storage and a replication prefix; deriving it
// from something a person can rename — a slug, an email, an owner/name pair —
// makes a rename into a file move and a broken history. Transferring a
// repository between orgs is likewise a row somewhere, not a namespace change,
// which is why a repo is repo/<id> and never org/<org>/repo/<name>.
func Org(id string) (Namespace, error)  { return of(KindOrg, id) }
func User(id string) (Namespace, error) { return of(KindUser, id) }
func Repo(id string) (Namespace, error) { return of(KindRepo, id) }

// System is the instance's own namespace: what belongs to the deployment rather
// than to any org, user or repo — app state, login sources, topics, notices.
//
// It takes no id because there is one instance. Groups keep its contents in
// separate files (System().WithGroup(Notices)), so "instance-level" costs a
// namespace and not an exception to the rule that every table lives in one.
func System() Namespace { return Namespace{string(KindSystem)} }

// MustOrg, MustUser and MustRepo are for ids fixed in the source — tests,
// seeds, a constant in a migration. They panic, which is correct for a value
// that is wrong before the program runs and wrong for anything from a request.
func MustOrg(id string) Namespace  { return must(Org(id)) }
func MustUser(id string) Namespace { return must(User(id)) }
func MustRepo(id string) Namespace { return must(Repo(id)) }

// NewGroup returns the group of that name.
func NewGroup(name string) (Group, error) {
	c, err := ident(name)
	if err != nil {
		return Group{}, fmt.Errorf("namespace: group: %w", err)
	}
	return Group{c}, nil
}

// MustGroup is NewGroup for a group named in the source, which is all of them.
func MustGroup(name string) Group {
	g, err := NewGroup(name)
	if err != nil {
		panic(err)
	}
	return g
}

// of is the one place a Namespace is built from parts, so the rule about which
// kinds exist and which of them name an entity is stated once.
func of(k Kind, id string) (Namespace, error) {
	switch k {
	case KindOrg, KindUser, KindRepo:
	case KindSystem:
		if id != "" {
			return Namespace{}, fmt.Errorf("namespace: %s names the instance and takes no id, got %q", k, id)
		}
		return System(), nil
	default:
		return Namespace{}, fmt.Errorf("namespace: %q is not an entity kind", string(k))
	}
	c, err := ident(id)
	if err != nil {
		return Namespace{}, fmt.Errorf("namespace: %s id: %w", k, err)
	}
	return join(k, c, ""), nil
}

func join(k Kind, id, group string) Namespace {
	s := string(k)
	if id != "" {
		s += "/" + id
	}
	if group != "" {
		s += "/" + group
	}
	return Namespace{s}
}

// parts splits the canonical form back into what built it.
func (ns Namespace) parts() (k Kind, id, group string) {
	kind, rest, _ := strings.Cut(ns.s, "/")
	a, b, _ := strings.Cut(rest, "/")
	if Kind(kind) == KindSystem {
		return KindSystem, "", a // the instance names no entity, so its one segment is a group
	}
	return Kind(kind), a, b
}

// WithGroup returns this entity's namespace for one group of its related types.
//
// It REPLACES any group already set instead of nesting, so a namespace is
// always kind[/id][/group] and there is never a second shape to parse. The zero
// Namespace absorbs, because there is no entity to name a group of.
func (ns Namespace) WithGroup(g Group) Namespace {
	k, id, _ := ns.parts()
	if k == "" {
		return ns
	}
	return join(k, id, g.s)
}

func (ns Namespace) Kind() Kind     { k, _, _ := ns.parts(); return k }
func (ns Namespace) ID() string     { _, id, _ := ns.parts(); return id }
func (ns Namespace) Group() Group   { _, _, g := ns.parts(); return Group{g} }
func (ns Namespace) IsZero() bool   { return ns.s == "" }
func (ns Namespace) String() string { return ns.s }

func (g Group) String() string { return g.s }

// Parse reads a namespace back from its string form, for the strings that
// arrive from outside a Go program: a request, a config file, a queue message.
//
// Parse(ns.String()) == ns for every ns a constructor returns, so a namespace
// survives a round trip instead of degrading into a string with conventions
// attached to it.
//
// It folds case exactly as the constructors do and repairs nothing else.
// "Org/Acme" is org/acme because a namespace is a value and "Acme" and "acme"
// are one org — one value, one spelling, which is canonicalisation. "org//acme"
// and "org/acme/" are errors, because String never emits them and accepting
// them would make one namespace have several encodings — which is the aliasing
// this package removes, arriving through the back door.
func Parse(s string) (Namespace, error) {
	kind, rest, hasRest := strings.Cut(s, "/")
	k, err := ident(kind)
	if err != nil {
		return Namespace{}, fmt.Errorf("namespace: %q: kind: %w", s, err)
	}
	id, group, hasGroup := strings.Cut(rest, "/")
	if Kind(k) == KindSystem {
		id, group, hasGroup = "", rest, hasRest // no entity, so all of rest is the group
	}
	if strings.Contains(group, "/") {
		return Namespace{}, fmt.Errorf("namespace: %q has too many segments, want kind[/id][/group]", s)
	}
	ns, err := of(Kind(k), id)
	if err != nil {
		return Namespace{}, err
	}
	if !hasGroup {
		return ns, nil
	}
	g, err := NewGroup(group)
	if err != nil {
		return Namespace{}, fmt.Errorf("namespace: %q: %w", s, err)
	}
	return ns.WithGroup(g), nil
}

// MarshalText and UnmarshalText put every encoder that speaks text — JSON, YAML,
// a flag, a database column — through Parse and String rather than around them.
// Without MarshalText, json.Marshal of a Namespace emits {}: the fields that
// make illegal values unrepresentable also make them invisible to reflection,
// so this is not a convenience, it is the difference between a namespace and
// silent data loss.
//
// Marshalling the zero Namespace is an error rather than "": the mistake is at
// the write, and "" would only fail later at whoever reads it back.
func (ns Namespace) MarshalText() ([]byte, error) {
	if ns.IsZero() {
		return nil, errors.New("namespace: the zero Namespace names no database")
	}
	return []byte(ns.s), nil
}

func (ns *Namespace) UnmarshalText(b []byte) error {
	p, err := Parse(string(b))
	if err != nil {
		return err
	}
	*ns = p
	return nil
}

// ident canonicalises one segment — an id, a group or a kind — and is the whole
// of what a namespace may be made of.
//
// Lowercase, because a namespace is a FILE and half the fleet develops on a
// case-insensitive filesystem. There "org/Acme" and "org/acme" are two
// namespaces over one file: two open handles, two replication streams, one
// history, and it works on the laptop. Folding at construction removes the
// difference rather than hoping every caller normalises first. ASCII-only, so
// the fold is the same everywhere — Unicode lowercases Turkish "I" to a dotless
// "ı", which would name a different file on a Turkish laptop than on a server.
//
// [a-z0-9][a-z0-9_-]* then settles what a segment cannot be by construction
// instead of by denylist: no separator, so no path; no dot, so no ".." and no
// hidden name; no trailing dot or space, which Windows and some object stores
// drop; no empty segment; no percent- or unicode-encoded form of any of those.
func ident(s string) (string, error) {
	if s == "" {
		return "", errors.New("must not be empty")
	}
	if len(s) > maxSegment {
		return "", fmt.Errorf("%q is %d bytes, over the %d-byte limit", s, len(s), maxSegment)
	}
	folded := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c >= 'A' && c <= 'Z':
			folded = true
		case (c == '-' || c == '_') && i > 0:
		default:
			return "", fmt.Errorf("%q: %q is not allowed, want [a-z0-9][a-z0-9_-]*", s, string(rune(c)))
		}
	}
	if !folded {
		return s, nil
	}
	return strings.ToLower(s), nil // provably ASCII by the loop above, so this is the ASCII fold
}

func must(ns Namespace, err error) Namespace {
	if err != nil {
		panic(err)
	}
	return ns
}
