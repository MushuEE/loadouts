package importer

import "errors"

var (
	// ErrInvalidURL means the string isn't a usable http(s) URL.
	ErrInvalidURL = errors.New("invalid url")
	// ErrNotAProduct means the host is a retailer we know, but the URL points at a
	// search/category/home page rather than a specific product.
	ErrNotAProduct = errors.New("not a product page")
	// ErrBlockedHost means the URL resolves somewhere we refuse to fetch (loopback,
	// private ranges, link-local). See fetch.go for why this matters.
	ErrBlockedHost = errors.New("blocked host")
)
