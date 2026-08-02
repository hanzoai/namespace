# namespace

Names the one database an entity's data lives in.

```go
ns := namespace.MustRepo("42").WithGroup(Issues) // repo/42/issues
ns, err := namespace.Org(id)                     // org/<id>
namespace.System().WithGroup(Notices)            // system/notices
```

A namespace is `kind[/id][/group]`, and its string form IS a path: `<dir>/<ns>.db`
on disk, a key prefix in object storage. So it is a value with constructors, not
a string with a validator — `Namespace` wraps an unexported field, and the only
routes to one are the constructors and `Parse`. Illegal names do not exist to be
caught.

Two rules do the work. Segments are `[a-z0-9][a-z0-9_-]*`, which leaves no
spelling of a separator, a dot-segment or a hidden name to reject later. And ids
fold to lowercase at construction, because on a case-insensitive filesystem
`org/Acme` and `org/acme` are two namespaces over one file — two handles, two
replication streams, one history, and it works on the laptop.

An id is the entity's stable identity, never a human-facing name: a rename must
be a row somewhere, not a file move and a severed history.

## From a chosen name, to a place

```go
slug := namespace.Sanitize(org)                        // one injective slug
ns, err := namespace.OrgProject(org, project)          // org/<slug>[/<project>]
key, err := namespace.Key(ns, "tracker")               // orgs/<slug>/tracker.db
path, err := namespace.Path(dataDir, ns, "tracker")    // <dataDir>/orgs/<slug>/tracker.db
```

`Sanitize` reduces a name a person chose — an IAM org, a project — to a
`[a-z0-9-]` slug and is **injective**: names carrying an invisible rune are
refused outright (nothing downstream can be trusted not to trim them), and every
other fold is disambiguated by a hash of the raw bytes, so `Acme` and `acme` stay
two orgs and therefore two files. `OrgProject` is the door those names walk
through to become a namespace.

`Key` renders a namespace as the place one subsystem's file for it lives, and
`Path` resolves that against a data directory. One rendering, two consumers: the
local file and the object key durability ships it to are the same string by
construction, so they cannot drift apart.

`hanzoai/orm/db.Namespaces` resolves a namespace to an open handle — materialise,
open, cache, evict. That is the only other thing with an opinion about namespaces.
