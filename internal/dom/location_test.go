package dom

import "testing"

func TestLocation(t *testing.T) {
	urlStr := "https://example.com:8080/path/to/page?query=123#section-1"
	loc := NewLocation(urlStr)

	if loc.Href != urlStr {
		t.Errorf("Expected Href %s, got %s", urlStr, loc.Href)
	}
	if loc.Protocol != "https:" {
		t.Errorf("Expected Protocol 'https:', got '%s'", loc.Protocol)
	}
	if loc.Host != "example.com:8080" {
		t.Errorf("Expected Host 'example.com:8080', got '%s'", loc.Host)
	}
	if loc.Hostname != "example.com" {
		t.Errorf("Expected Hostname 'example.com', got '%s'", loc.Hostname)
	}
	if loc.Port != "8080" {
		t.Errorf("Expected Port '8080', got '%s'", loc.Port)
	}
	if loc.Pathname != "/path/to/page" {
		t.Errorf("Expected Pathname '/path/to/page', got '%s'", loc.Pathname)
	}
	if loc.Search != "?query=123" {
		t.Errorf("Expected Search '?query=123', got '%s'", loc.Search)
	}
	if loc.Hash != "#section-1" {
		t.Errorf("Expected Hash '#section-1', got '%s'", loc.Hash)
	}
}

func TestLocationDefault(t *testing.T) {
	loc := NewLocation("")
	if loc.Href != "http://localhost/" {
		t.Errorf("Expected default Href 'http://localhost/', got %s", loc.Href)
	}
	if loc.Pathname != "/" {
		t.Errorf("Expected default Pathname '/', got %s", loc.Pathname)
	}
}
