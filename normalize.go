package main

import (
	"net/url"
	"strings"
)

// normalizeURL prepends http:// when no scheme is present, then validates http(s) with a host.
func normalizeURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errInvalidURL
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", errInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errInvalidURL
	}
	if u.Host == "" {
		return "", errInvalidURL
	}
	return u.String(), nil
}
