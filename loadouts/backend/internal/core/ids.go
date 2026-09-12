package core

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// NewID returns a prefixed, URL-safe identifier, e.g. "ldt_9f2c1a...".
// Prefixed IDs make logs and API responses self-describing, which is worth more at Day 0
// than the marginal density of a bare UUID.
func NewID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is unrecoverable; a panic here is preferable to colliding IDs.
		panic("core: unable to read random bytes for ID: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

var slugCleaner = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify normalizes a display name into a URL-safe slug ("UL Backpacking" -> "ul-backpacking").
func Slugify(s string) string {
	out := slugCleaner.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	return strings.Trim(out, "-")
}
