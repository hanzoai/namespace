package namespace

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

var (
	issues = MustGroup("issues")
	labels = MustGroup("labels")
)

// every returns one of each shape a namespace comes in, which is what the
// properties below are asserted over.
func every() []Namespace {
	return []Namespace{
		MustOrg("acme"),
		MustOrg("acme").WithGroup(MustGroup("members")),
		MustUser("01hxyz9k7m"),
		MustUser("01hxyz9k7m").WithGroup(MustGroup("api_keys")),
		MustRepo("42"),
		MustRepo("42").WithGroup(issues),
		System(),
		System().WithGroup(MustGroup("notices")),
	}
}

// dbPath is what orm/db.pathFor does with a namespace, reproduced so the
// escape property is asserted against the real consumer and not a paraphrase.
func dbPath(t *testing.T, dir string, ns Namespace) string {
	t.Helper()
	return filepath.Clean(filepath.Join(dir, filepath.FromSlash(ns.String())+".db"))
}

func TestConstructors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ns    Namespace
		s     string
		kind  Kind
		id    string
		group string
	}{
		{"org", MustOrg("acme"), "org/acme", KindOrg, "acme", ""},
		{"org grouped", MustOrg("acme").WithGroup(MustGroup("members")), "org/acme/members", KindOrg, "acme", "members"},
		{"user", MustUser("z1"), "user/z1", KindUser, "z1", ""},
		{"user grouped", MustUser("z1").WithGroup(labels), "user/z1/labels", KindUser, "z1", "labels"},
		{"repo", MustRepo("42"), "repo/42", KindRepo, "42", ""},
		{"repo grouped", MustRepo("42").WithGroup(issues), "repo/42/issues", KindRepo, "42", "issues"},
		{"system", System(), "system", KindSystem, "", ""},
		{"system grouped", System().WithGroup(issues), "system/issues", KindSystem, "", "issues"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ns.String(); got != tc.s {
				t.Errorf("String() = %q, want %q", got, tc.s)
			}
			if got := tc.ns.Kind(); got != tc.kind {
				t.Errorf("Kind() = %q, want %q", got, tc.kind)
			}
			if got := tc.ns.ID(); got != tc.id {
				t.Errorf("ID() = %q, want %q", got, tc.id)
			}
			if got := tc.ns.Group().String(); got != tc.group {
				t.Errorf("Group() = %q, want %q", got, tc.group)
			}
			if tc.ns.IsZero() {
				t.Error("reports zero")
			}
		})
	}
}

// An id is runtime data, so every constructor refuses the same set at the same
// moment — before the value exists, not after it has been written down.
func TestConstructorsRejectIllegalIDs(t *testing.T) {
	ctors := map[string]func(string) (Namespace, error){"Org": Org, "User": User, "Repo": Repo}
	for _, id := range []string{
		"", "..", ".", ".hidden", "a/b", "a\\b", "/a", "a/", "A B", "a.b", "a@b",
		"-x", "_x", "café", "a\x00b", "%2e%2e", "acme ", "acme.",
		strings.Repeat("a", maxSegment+1),
	} {
		for name, ctor := range ctors {
			if got, err := ctor(id); err == nil {
				t.Errorf("%s(%q) = %q, want an error", name, id, got)
			}
		}
		if got, err := NewGroup(id); err == nil {
			t.Errorf("NewGroup(%q) = %q, want an error", id, got)
		}
	}
	if got := len(strings.Repeat("a", maxSegment)); got != maxSegment { // the limit itself is legal
		t.Fatal("bad fixture")
	}
	if _, err := Org(strings.Repeat("a", maxSegment)); err != nil {
		t.Errorf("an id of exactly %d bytes: %v", maxSegment, err)
	}
}

// The instance names no entity, so there is no id that could qualify it. of is
// the only door to that error because System takes no argument at all.
func TestSystemTakesNoID(t *testing.T) {
	if got, err := of(KindSystem, "acme"); err == nil {
		t.Errorf("system with an id = %q, want an error", got)
	}
	if got, err := of(Kind("project"), "x"); err == nil {
		t.Errorf("an unknown kind = %q, want an error", got)
	}
}

// One logical namespace has ONE value, so equality is the whole of comparison
// and a map keyed by it cannot open one file under two entries.
func TestCanonicalisationIsTotal(t *testing.T) {
	for _, spellings := range [][]string{
		{"acme", "Acme", "ACME", "aCmE"},
		{"01hxyz", "01HXYZ"},
		{"a-b_c", "A-B_C"},
	} {
		want := MustOrg(spellings[0])
		for _, id := range spellings[1:] {
			if got := MustOrg(id); got != want {
				t.Errorf("Org(%q) = %q, want %q", id, got, want)
			}
		}
	}
	if a, b := MustRepo("R7").WithGroup(MustGroup("Issues")), MustRepo("r7").WithGroup(issues); a != b {
		t.Errorf("%q != %q", a, b)
	}
	// Distinct values name distinct files: the aliasing bug, stated as a
	// property rather than trusted to the naming rules.
	seen := map[string]Namespace{}
	dir := t.TempDir()
	for _, ns := range every() {
		p := dbPath(t, dir, ns)
		if other, dup := seen[p]; dup {
			t.Errorf("%q and %q both name %q", ns, other, p)
		}
		seen[p] = ns
	}
}

// A namespace survives a wire hop, a config file or a database column. If it
// did not, it would decay into a string with conventions attached and every
// consumer would re-derive them.
func TestRoundTrip(t *testing.T) {
	for _, ns := range every() {
		got, err := Parse(ns.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", ns, err)
		}
		if got != ns {
			t.Errorf("Parse(%q) = %q, want %q", ns, got, ns)
		}
		b, err := json.Marshal(ns)
		if err != nil {
			t.Fatalf("Marshal(%q): %v", ns, err)
		}
		if want := `"` + ns.String() + `"`; string(b) != want {
			t.Errorf("Marshal(%q) = %s, want %s", ns, b, want)
		}
		var back Namespace
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("Unmarshal(%s): %v", b, err)
		}
		if back != ns {
			t.Errorf("json round trip of %q gave %q", ns, back)
		}
	}
	var bad Namespace
	if err := json.Unmarshal([]byte(`"org/../etc"`), &bad); err == nil {
		t.Errorf("unmarshalled an illegal namespace as %q", bad)
	}
	if !bad.IsZero() {
		t.Errorf("a failed unmarshal left %q behind", bad)
	}
}

func TestParseFoldsCaseAndNothingElse(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"org/acme", "org/acme"},
		{"Org/Acme", "org/acme"},
		{"ORG/ACME/MEMBERS", "org/acme/members"},
		{"SYSTEM", "system"},
		{"System/Notices", "system/notices"},
		{"repo/42/issues", "repo/42/issues"},
	} {
		got, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.in, err)
		}
		if got.String() != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if again, err := Parse(got.String()); err != nil || again != got {
			t.Errorf("Parse(%q) = %q, %v — not idempotent", got, again, err)
		}
	}
}

// Every name orm's canonical() had to reject is now unconstructible, and Parse
// is the only door a raw string can come through.
func TestParseRejects(t *testing.T) {
	for _, s := range append(escapeCorpus(),
		"", "org", "user", "repo", "ORG", "project/x", "system/acme/x",
		"org/acme/issues/extra", "org//acme", "org/acme/", "org/acme ",
		"org/a b", "org/a@b", "org/café", "org/a\x00b", "org/-x", "org/_x",
		"org/"+strings.Repeat("a", maxSegment+1),
	) {
		if got, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", s, got)
		}
	}
}

// escapeCorpus is every way a string has ever been used to leave a directory.
// None of them may become a Namespace, and TestNamespaceCannotEscape proves the
// stronger claim: nothing that parses can, whatever it is spelled.
func escapeCorpus() []string {
	return []string{
		"/", "//", "..", ".", "./org/acme", "/org/acme", "//org/acme",
		"../org/acme", "org/../../etc", "a/../../b", "org/./acme",
		"org/x/../acme", "org/..", "org/../..", "\\org\\acme",
		"..\\..\\etc", "org/%2e%2e/etc", "org/%2f/etc", "-org/acme",
		".hidden/acme", "org/.hidden", "c:/org/acme", "org/acme/../../..",
		"\u002e\u002e/etc", "org/\u2044etc", "org/acme\x00/../etc",
	}
}

// The reason the type exists: its string form is a path, and no value of it can
// name a file outside the cache directory however it is spelled.
func TestNamespaceCannotEscape(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Clean(dir) + string(filepath.Separator)
	check := func(ns Namespace) {
		t.Helper()
		if p := dbPath(t, dir, ns); !strings.HasPrefix(p, root) {
			t.Errorf("%q resolved to %q, outside %q", ns, p, root)
		}
		if n := strings.Count(ns.String(), "/"); n > 2 {
			t.Errorf("%q is %d deep, want at most kind/id/group", ns, n)
		}
	}
	for _, ns := range every() {
		check(ns)
	}
	for _, s := range escapeCorpus() {
		ns, err := Parse(s)
		if err != nil {
			continue
		}
		t.Errorf("Parse(%q) = %q", s, ns)
		check(ns) // and if it ever does parse, it still may not escape
	}
}

// The property, over anything at all rather than a corpus somebody thought of.
func FuzzParse(f *testing.F) {
	for _, s := range append(escapeCorpus(), "org/acme", "repo/42/issues", "system", "System/Notices") {
		f.Add(s)
	}
	dir := filepath.FromSlash("/ns")
	root := filepath.Clean(dir) + string(filepath.Separator)
	f.Fuzz(func(t *testing.T, s string) {
		ns, err := Parse(s)
		if err != nil {
			return
		}
		if ns.IsZero() {
			t.Fatalf("Parse(%q) succeeded with the zero Namespace", s)
		}
		if p := filepath.Clean(filepath.Join(dir, filepath.FromSlash(ns.String())+".db")); !strings.HasPrefix(p, root) {
			t.Fatalf("Parse(%q) = %q, which resolves to %q", s, ns, p)
		}
		again, err := Parse(ns.String())
		if err != nil || again != ns {
			t.Fatalf("Parse(%q) = %q, %v — not idempotent", ns, again, err)
		}
		if got := join(ns.Kind(), ns.ID(), ns.Group().String()); got != ns {
			t.Fatalf("%q split into %q/%q/%q, which rejoins as %q", ns, ns.Kind(), ns.ID(), ns.Group(), got)
		}
	})
}

// A group replaces, never nests, so there is exactly one shape to parse.
func TestGroupReplaces(t *testing.T) {
	r := MustRepo("42")
	if got, want := r.WithGroup(issues).WithGroup(labels), r.WithGroup(labels); got != want {
		t.Errorf("regrouped to %q, want %q", got, want)
	}
	if got := r.WithGroup(issues).WithGroup(Group{}); got != r {
		t.Errorf("the zero Group gave %q, want the entity's own namespace %q", got, r)
	}
	if got := System().WithGroup(issues).WithGroup(Group{}); got != System() {
		t.Errorf("the zero Group on system gave %q", got)
	}
}

// The zero value is the one illegal Namespace Go permits. It is defined, and it
// can never be mistaken for a real one.
func TestZero(t *testing.T) {
	var ns Namespace
	if !ns.IsZero() || ns.String() != "" || ns.Kind() != "" || ns.ID() != "" {
		t.Errorf("zero Namespace = %q, kind %q, id %q", ns.String(), ns.Kind(), ns.ID())
	}
	if ns.WithGroup(issues) != ns {
		t.Error("the zero Namespace grew a group")
	}
	if _, err := ns.MarshalText(); err == nil {
		t.Error("MarshalText of the zero Namespace succeeded, want an error")
	}
	if b, err := json.Marshal(struct{ NS Namespace }{}); err == nil {
		t.Errorf("json.Marshal of a zero Namespace gave %s, want an error", b)
	}
}

func TestMustPanics(t *testing.T) {
	for name, fn := range map[string]func(){
		"MustOrg":   func() { MustOrg("../etc") },
		"MustUser":  func() { MustUser("") },
		"MustRepo":  func() { MustRepo("a/b") },
		"MustGroup": func() { MustGroup(".hidden") },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("did not panic")
				}
			}()
			fn()
		})
	}
}

func BenchmarkOrg(b *testing.B) {
	for b.Loop() {
		if _, err := Org("01hxyz9k7mq4"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		if _, err := Parse("repo/01hxyz9k7mq4/issues"); err != nil {
			b.Fatal(err)
		}
	}
}
