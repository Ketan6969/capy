package main

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

func validateExtractRequest(rawURL, script string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("url is required")
	}
	if strings.TrimSpace(script) == "" {
		return errors.New("script is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("url must use http or https")
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "", "localhost":
		return errors.New("localhost targets are not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
			return errors.New("private network targets are not allowed")
		}
	}

	return nil
}
