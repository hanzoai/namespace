// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import "testing"

// sanitizeGolden is the same corpus twice over: it is the table
// TestSanitizeIsTheStringOnDisk asserts, and the input set the injectivity and
// safety proofs below sweep. One list, so a case added to pin a rendering is
// also a case the properties must survive.
//
// Every want was captured from the implementation that produced the slugs
// already on disk, before the code moved into this package.
var sanitizeGolden = []struct{ in, want string }{
	// Identity on a clean, unambiguous, ≤32-byte label.
	{"acme", "acme"},
	{"acme-corp", "acme-corp"},
	{"team-a", "team-a"},
	{"my-team-42", "my-team-42"},
	{"maxpower", "maxpower"},
	{"platform", "platform"},
	{"system", "system"},
	{"org", "org"},
	{"a", "a"},
	{"0", "0"},
	{"z9", "z9"},
	{"9lives", "9lives"},

	// Case folds, and the hash of the RAW bytes keeps the spellings apart.
	{"ACME", "acme-a8308e8e2e4b7330"},
	{"Acme", "acme-37036cd8f9746d33"},
	{"MaxPower", "maxpower-a3e10398a4a44f2e"},

	// Punctuation folds to "-", so these three would otherwise be one name.
	{"acme_corp", "acme-corp-cca8c7942f8c15a2"},
	{"acme.corp", "acme-corp-80a1d5cc623b165f"},
	{"team.a", "team-a-dc6273296fb8abe9"},
	{"team_a", "team-a-112cd0cb52eea635"},
	{"x-y_z", "x-y-z-988a16a3150c2188"},

	// A leading "_" is folded away, which is why the system partition's
	// "_platform" directory is unreachable from any name.
	{"_platform", "platform-7583da5924deda8d"},
	{"_system", "system-ee3df3979817a58d"},

	// Non-ASCII is dropped to "-" and disambiguated; a name that folds to
	// nothing at all is the hash alone, behind a leading "-".
	{"über", "ber-b51c854170449c38"},
	{"Ünïcödé", "n-c-d-39af95d07d82b5d6"},
	{"日本語", "-77710aedc74ecfa3"},
	{"!!!", "-e84c538e7fe25073"},
	{".", "-cdb4ee2aea69cc6a"},
	{"..", "-5ec1f7e700f37c3d"},

	// A name written as a path becomes one segment.
	{"../etc", "etc-f7f9121fdb53d8de"},
	{"a/b", "a-b-c14cddc033f64b9d"},
	{`a\b`, "a-b-c62016d0f8ee3333"},
	{"..%2f..%2fvictim", "2f---2fvictim-26573cabaaa3ba65"},

	// Over 32 bytes: truncate the fold, keep the hash of the whole input.
	{"this-is-a-very-long-org-name-that-exceeds-thirty-two-characters", "this-is-a-very-long-org-name-tha-d6c796465b2f097c"},
	{"THIS-IS-A-VERY-LONG-ORG-NAME-THAT-EXCEEDS-THIRTY-TWO-CHARACTERS", "this-is-a-very-long-org-name-tha-e1e451cbed85af65"},

	// A clean label shaped like this function's own output is re-suffixed, so
	// it cannot alias the folded name that renders that way. One with a "g" in
	// the tail is not that shape, and keeps the fast path.
	{"foo-0123456789abcdef", "foo-0123456789abcdef-1225a522cf51bf26"},
	{"foo-0123456789abcdeg", "foo-0123456789abcdeg"},

	// "-abc" is a clean label, so it is the identity — and therefore a slug no
	// namespace segment will accept. Refusing it is OrgProject's job, not this
	// one's; pinned here so the division stays visible.
	{"-abc", "-abc"},

	// Refused: empty, or carrying a rune something downstream trims away.
	{"", ""},
	{"acme ", ""},
	{" acme", ""},
	{"ACME CORP", ""},
	{"acme\u200b", ""},
	{"acme corp", ""},
	{"acme\tcorp", ""},
	{"acme\ncorp", ""},
	{"acme\x00", ""},
}

// TestSanitizeIsTheStringOnDisk is the byte-identity proof. A slug is a
// directory name on a live volume and a key prefix in a live bucket, so a
// rendering that drifts by one character does not fail — it opens an empty
// database beside an entity's real one.
func TestSanitizeIsTheStringOnDisk(t *testing.T) {
	for _, tc := range sanitizeGolden {
		if got := Sanitize(tc.in); got != tc.want {
			t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSanitizeIsInjective is the property the whole boundary rests on: two
// distinct names never render to one database. Refusals are excluded — "" is
// not an output, it is the absence of one, and every caller gates on it.
func TestSanitizeIsInjective(t *testing.T) {
	seen := map[string]string{}
	for _, tc := range sanitizeGolden {
		got := Sanitize(tc.in)
		if got == "" {
			continue
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("%q and %q both render to %q — one database, two entities", prev, tc.in, got)
		}
		seen[got] = tc.in
	}
}

// TestSanitizeOutputIsOneSafeSegment pins what an accepted name can BE: lower
// ASCII, digits and "-", never a separator, a dot or a traversal. This is what
// makes a hostile name harmless rather than merely unusual.
func TestSanitizeOutputIsOneSafeSegment(t *testing.T) {
	for _, tc := range sanitizeGolden {
		got := Sanitize(tc.in)
		if got == "" {
			continue
		}
		if got == "." || got == ".." {
			t.Errorf("Sanitize(%q) = %q, which names a directory above its own", tc.in, got)
		}
		for _, r := range got {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			default:
				t.Errorf("Sanitize(%q) = %q, which carries %q", tc.in, got, string(r))
			}
		}
	}
}

// TestSanitizeIsIdempotentOnlyWhereItIsTheIdentity is the trap the "looks
// suffixed" rule exists for. Sanitize is NOT idempotent on a folded name, and
// must not be re-applied to a slug it already produced: doing so would mint a
// second, different name for the same entity. Feeding its output back in is a
// bug, and this states which way round it fails.
func TestSanitizeIsIdempotentOnlyWhereItIsTheIdentity(t *testing.T) {
	for _, tc := range sanitizeGolden {
		once := Sanitize(tc.in)
		if once == "" {
			continue
		}
		twice := Sanitize(once)
		if once == tc.in {
			if twice != once {
				t.Errorf("Sanitize is the identity on %q but Sanitize(Sanitize(%q)) = %q", tc.in, tc.in, twice)
			}
			continue
		}
		if twice == once {
			t.Errorf("Sanitize(%q) = %q survived re-folding — a folded slug must never "+
				"be indistinguishable from a raw name that needs one", tc.in, once)
		}
	}
}
