// Copyright (C) 2026, Hanzo AI. All rights reserved.
// See the file LICENSE for licensing terms.

package namespace

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// Sanitize reduces an externally-chosen name — an IAM org, a project — to a
// lowercase [a-z0-9-] slug, and it is INJECTIVE: two distinct inputs never
// share an output.
//
// Injectivity is the whole point, because the slug becomes the name of a
// DATABASE. A lossy fold is not a cosmetic defect there, it is two entities
// reading and writing one file. The three ways a fold loses information are
// each closed:
//
//   - Invisible runes are REFUSED (→ ""). A name carrying whitespace, a
//     control rune or a zero-width format rune cannot be folded injectively,
//     because strings.TrimSpace and an HTTP header's own OWS trim drop them
//     later regardless of what this function does — "acme " would arrive as
//     "acme". Refusing is the only answer that survives transport. Callers gate
//     on "", so this is fail-secure.
//   - Case and punctuation are folded, then DISAMBIGUATED: every accepted name
//     that is not already a clean slug maps to its fold plus "-" and the first
//     16 hex of SHA-256 over the RAW input bytes. Without the suffix, ToLower,
//     every non-[a-z0-9-] rune → "-", and the 32-byte truncation would collapse
//     "Acme"/"acme" and "team.a"/"team-a" onto one name.
//   - A clean slug that ITSELF looks like this function's own output —
//     <label>-<16 lowercase hex> — is denied the identity fast path and
//     re-suffixed, so an entity that squats on the rendered form of another
//     name cannot alias it. That is the only collision class the fast path
//     could otherwise admit.
//
// It is the identity on a clean, unambiguous, ≤32-byte DNS-1123 label, which is
// what almost every real name is — so the common case reads as itself on disk.
//
// It does NOT return an error, and that is deliberate: "" is the one refusal,
// every caller already has to decide what to do about a name it cannot use, and
// a second failure mode would only be a second thing to forget to check.
func Sanitize(s string) string {
	if s == "" || hasUnsafeRune(s) {
		return ""
	}
	lower := strings.ToLower(s)
	if isDNSLabel(lower) && lower == s && len(lower) <= 32 && !looksSuffixed(lower) {
		return lower // already a clean, unambiguous slug: identity, no suffix needed
	}
	var b strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	folded := strings.Trim(b.String(), "-")
	if len(folded) > 32 {
		folded = strings.Trim(folded[:32], "-")
	}
	// The suffix is derived from the EXACT input bytes — never a trimmed or
	// folded copy — so two names that fold together stay apart unless SHA-256
	// itself collides on them.
	sum := sha256.Sum256([]byte(s))
	return folded + "-" + hex.EncodeToString(sum[:8])
}

// hasUnsafeRune reports whether s carries any whitespace, control, or
// zero-width/format rune: the class no injective fold survives, because
// something downstream trims it. Case, "-", "." and other VISIBLE punctuation
// are deliberately not in it — those fold injectively through the hash above.
func hasUnsafeRune(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}

// isDNSLabel reports whether s is a non-empty [a-z0-9-] string that is not "."
// or "..". Such a name needs no disambiguation — Sanitize is the identity on it.
func isDNSLabel(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

// looksSuffixed reports whether s ends in "-" and exactly 16 lowercase-hex
// characters — the shape of Sanitize's own disambiguation.
func looksSuffixed(s string) bool {
	if len(s) < 17 || s[len(s)-17] != '-' {
		return false
	}
	for _, r := range s[len(s)-16:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
