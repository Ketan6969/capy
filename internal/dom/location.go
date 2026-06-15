package dom

import (
	"net/url"
)

// Location represents the browser's window.location API.
type Location struct {
	Href     string `json:"href"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Hostname string `json:"hostname"`
	Port     string `json:"port"`
	Pathname string `json:"pathname"`
	Search   string `json:"search"`
	Hash     string `json:"hash"`
}

// NewLocation parses a URL string and constructs a browser-compliant Location instance.
func NewLocation(urlStr string) *Location {
	if urlStr == "" {
		urlStr = "http://localhost/"
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return &Location{
			Href:     "http://localhost/",
			Protocol: "http:",
			Host:     "localhost",
			Hostname: "localhost",
			Pathname: "/",
		}
	}

	search := ""
	if u.RawQuery != "" {
		search = "?" + u.RawQuery
	}

	hash := ""
	if u.Fragment != "" {
		hash = "#" + u.Fragment
	}

	pathname := u.Path
	if pathname == "" {
		pathname = "/"
	}

	return &Location{
		Href:     u.String(),
		Protocol: u.Scheme + ":",
		Host:     u.Host,
		Hostname: u.Hostname(),
		Port:     u.Port(),
		Pathname: pathname,
		Search:   search,
		Hash:     hash,
	}
}
