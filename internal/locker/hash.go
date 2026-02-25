package locker

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// ImplEntry holds an impl's name and body text for plan hashing.
type ImplEntry struct {
	Name     string
	BodyText string
}

// HashPlan computes a stable hash for a plan's source-level text.
// The hash covers the plan name, target paths, spec texts, constraint texts,
// and impl names + body texts — all normalized to reduce false invalidations.
func HashPlan(name string, targets []string, specTexts, constraintTexts []string, implEntries []ImplEntry) string {
	var parts []string
	parts = append(parts, name)
	parts = append(parts, targets...)
	parts = append(parts, specTexts...)
	parts = append(parts, constraintTexts...)
	for _, e := range implEntries {
		parts = append(parts, e.Name, e.BodyText)
	}
	return computeHash(parts...)
}

// HashImpl computes a stable hash for an impl's source-level text.
// The hash covers the impl name, body text, and target path.
func HashImpl(name, bodyText, targetPath string) string {
	return computeHash(name, bodyText, targetPath)
}

// normalize converts text to lowercase and strips whitespace and punctuation.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if !unicode.IsSpace(r) && !unicode.IsPunct(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// computeHash normalizes each part and computes SHA-256 of the concatenation.
// Parts are separated by null bytes to prevent collisions.
func computeHash(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(normalize(p)))
	}
	return hex.EncodeToString(h.Sum(nil))
}
