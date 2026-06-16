package main

import "testing"

func TestValidateExtractRequest(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		script  string
		wantErr bool
	}{
		{name: "valid public url", rawURL: "https://example.com", script: "document.title", wantErr: false},
		{name: "missing url", rawURL: "", script: "document.title", wantErr: true},
		{name: "missing script", rawURL: "https://example.com", script: "", wantErr: true},
		{name: "invalid scheme", rawURL: "file:///tmp/index.html", script: "document.title", wantErr: true},
		{name: "localhost blocked", rawURL: "http://localhost:8080", script: "document.title", wantErr: true},
		{name: "private ip blocked", rawURL: "http://192.168.1.10", script: "document.title", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExtractRequest(tc.rawURL, tc.script)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
