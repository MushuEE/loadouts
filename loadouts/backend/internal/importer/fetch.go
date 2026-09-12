package importer

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// Fetcher retrieves the HTML for a product page. It is an interface so tests can run the
// whole pipeline against fixture HTML with no network access.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// Default fetch limits. Product pages are large but not unbounded; 3MB comfortably fits
// the biggest retailer pages while capping the damage from a hostile response.
const (
	defaultTimeout   = 12 * time.Second
	defaultMaxBytes  = 3 << 20
	defaultRedirects = 5
)

// userAgent identifies us honestly. Pretending to be a browser would get us further with
// bot-hostile retailers, but the fallback-to-manual path means we don't need to lie.
const userAgent = "LoadoutsBot/0.1 (+https://github.com/MushuEE/loadouts)"

// HTTPFetcher fetches over the network with SSRF protection.
//
// Fetching a user-supplied URL from the server is a classic SSRF sink: without guards a
// user could point the importer at 169.254.169.254 (cloud metadata), an internal admin
// service, or localhost, and read the response back through the preview UI. The defense
// is applied at dial time via Dialer.Control, which means it covers the original host,
// every redirect hop, and DNS rebinding (where a hostname resolves to a public IP on the
// first lookup and a private one on the second) in a single place.
type HTTPFetcher struct {
	client   *http.Client
	maxBytes int64
}

func NewHTTPFetcher() *HTTPFetcher {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: unparseable address %q", ErrBlockedHost, address)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("%w: unresolvable address %q", ErrBlockedHost, host)
			}
			if !isPublicIP(ip) {
				return fmt.Errorf("%w: %s is not a public address", ErrBlockedHost, ip)
			}
			return nil
		},
	}

	return &HTTPFetcher{
		maxBytes: defaultMaxBytes,
		client: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				DialContext:           dialer.DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 8 * time.Second,
				DisableKeepAlives:     true,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= defaultRedirects {
					return fmt.Errorf("too many redirects")
				}
				if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
					return fmt.Errorf("%w: redirect to %s scheme", ErrBlockedHost, req.URL.Scheme)
				}
				return nil
			},
		},
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, unwrapFetchError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 403/503 from a bot wall is the expected Amazon case, so the message needs to
		// point the user at the manual path rather than read like a bug.
		return nil, fmt.Errorf("the store returned HTTP %d (it may be blocking automated readers)", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") {
		return nil, fmt.Errorf("expected an HTML page but got %s", ct)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes))
	if err != nil {
		return nil, fmt.Errorf("reading the page failed: %w", err)
	}
	return body, nil
}

// unwrapFetchError surfaces our own sentinel from inside net/url's wrapper so callers can
// errors.Is it, and otherwise keeps the message short enough for a UI toast.
func unwrapFetchError(err error) error {
	if strings.Contains(err.Error(), ErrBlockedHost.Error()) {
		return fmt.Errorf("%w: refusing to fetch a private or internal address", ErrBlockedHost)
	}
	return fmt.Errorf("could not reach the store: %w", err)
}

// isPublicIP rejects every range that could be used to reach infrastructure rather than
// the public internet.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsPrivate() {
		return false
	}
	// IsPrivate covers RFC1918 and RFC4193 but not these.
	blocked := []string{
		"100.64.0.0/10",   // CGNAT
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // TEST-NET-1
		"198.18.0.0/15",   // benchmarking
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved
		"::/128",          // unspecified
		"2001:db8::/32",   // documentation
	}
	for _, cidr := range blocked {
		if _, netw, err := net.ParseCIDR(cidr); err == nil && netw.Contains(ip) {
			return false
		}
	}
	return true
}

// StaticFetcher serves canned HTML. Tests use it to exercise extraction deterministically.
type StaticFetcher struct {
	Pages map[string][]byte
	Err   error
}

func (f *StaticFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	if body, ok := f.Pages[url]; ok {
		return body, nil
	}
	return nil, fmt.Errorf("no canned page for %s", url)
}
