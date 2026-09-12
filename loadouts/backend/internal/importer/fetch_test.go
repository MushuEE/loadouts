package importer

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"0.0.0.0",         // unspecified
		"10.1.2.3",        // RFC1918
		"172.16.5.4",      // RFC1918
		"192.168.1.1",     // RFC1918
		"169.254.169.254", // cloud metadata: the one that really matters
		"100.64.0.1",      // CGNAT
		"::1",             // IPv6 loopback
		"fd00::1",         // IPv6 ULA
		"fe80::1",         // IPv6 link-local
		"240.0.0.1",       // reserved
	}
	for _, s := range blocked {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("isPublicIP(%s) = true, want false", s)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "104.16.0.1", "2606:4700::1111"}
	for _, s := range allowed {
		if !isPublicIP(net.ParseIP(s)) {
			t.Errorf("isPublicIP(%s) = false, want true", s)
		}
	}
}

// The importer fetches a URL the user supplied, so pointing it at a loopback service must
// fail at dial time rather than returning that service's response through the preview UI.
func TestHTTPFetcherBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><title>internal admin console</title></html>"))
	}))
	defer srv.Close()

	_, err := NewHTTPFetcher().Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("fetching a loopback address succeeded, want it blocked")
	}
	if !errors.Is(err, ErrBlockedHost) {
		t.Errorf("error = %v, want ErrBlockedHost", err)
	}
}

func TestStaticFetcher(t *testing.T) {
	f := &StaticFetcher{Pages: map[string][]byte{"https://x/1": []byte("hi")}}
	got, err := f.Fetch(context.Background(), "https://x/1")
	if err != nil || string(got) != "hi" {
		t.Fatalf("Fetch = (%q, %v)", got, err)
	}
	if _, err := f.Fetch(context.Background(), "https://x/missing"); err == nil {
		t.Error("expected an error for an uncanned page")
	}
}
