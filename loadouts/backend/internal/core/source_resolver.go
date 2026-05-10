package core

import (
	"bytes"
	"text/template"
)

// ResolveSourceURL takes a supplier template and item-specific data to create the final affiliate link.
func ResolveSourceURL(supplier Supplier, source ItemSource) (string, error) {
	tmpl, err := template.New("url").Parse(supplier.AffiliateTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		BaseURL   string
		ProductID string
	}{
		BaseURL:   supplier.BaseURL,
		ProductID: source.ProductID,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
