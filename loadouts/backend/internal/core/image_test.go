package core

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateImageURL(t *testing.T) {
	for _, ok := range []string{
		"",
		"https://cdn.shopify.com/s/files/1/sack.jpg?v=123",
		"http://example.com/a.png",
		"/covers/alpine.svg",
	} {
		if err := ValidateImageURL(ok); err != nil {
			t.Errorf("ValidateImageURL(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{
		"javascript:alert(1)",
		"data:image/svg+xml;base64,PHN2Zz4=",
		"file:///etc/passwd",
		"//evil.example.com/x.png",
		"/\\evil.example.com",
		"cdn.example.com/x.png",
		"https://",
		"https://example.com/" + strings.Repeat("a", MaxImageURLLen),
	} {
		if err := ValidateImageURL(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("ValidateImageURL(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}
