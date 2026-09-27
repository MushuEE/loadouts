package core

import (
	"fmt"
	"net/url"
	"strings"
)

// MaxImageURLLen bounds stored image URLs. Retailer CDN URLs with signed query strings
// run long, but nothing legitimate approaches this.
const MaxImageURLLen = 2048

// ValidateImageURL accepts an empty string (no image), an absolute http(s) URL, or a
// root-relative path to an asset the app serves itself (the preset loadout covers live at
// /covers/...).
//
// Every image URL is rendered straight into an <img src> for other people, so this is
// the one gate that keeps javascript:, data:, and file: URLs out of shared rows.
// Protocol-relative "//host/x" is rejected along with them: it is an absolute URL in
// disguise, and the importer already resolves those to https before they get here.
func ValidateImageURL(raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > MaxImageURLLen {
		return fmt.Errorf("%w: image URL is longer than %d characters", ErrInvalid, MaxImageURLLen)
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		if strings.ContainsAny(raw, "\\\n\r") {
			return fmt.Errorf("%w: image path %q is malformed", ErrInvalid, raw)
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%w: image must be an http(s) URL", ErrInvalid)
	}
	return nil
}
