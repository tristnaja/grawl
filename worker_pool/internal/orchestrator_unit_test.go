package internal

import "testing"

func TestValidateStartURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "valid https", in: "https://example.com", wantErr: false},
		{name: "valid http", in: "http://example.com", wantErr: false},
		{name: "unsupported scheme", in: "ftp://example.com", wantErr: true},
		{name: "missing host", in: "https:///path", wantErr: true},
		{name: "invalid url", in: "http://[::1", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateStartURL(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeSchemeInput(t *testing.T) {
	t.Parallel()

	if got := normalizeSchemeInput("http"); got != "http" {
		t.Fatalf("expected http, got %s", got)
	}

	if got := normalizeSchemeInput("HTTPS"); got != "https" {
		t.Fatalf("expected https, got %s", got)
	}

	if got := normalizeSchemeInput("bad"); got != "https" {
		t.Fatalf("expected fallback https, got %s", got)
	}
}

func TestBuildFinalURL(t *testing.T) {
	t.Parallel()

	parsed, err := validateStartURL("https://example.com")
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	out, err := buildFinalURL(parsed)
	if err != nil {
		t.Fatalf("buildFinalURL returned error: %v", err)
	}

	if out != "https://example.com" {
		t.Fatalf("unexpected final url: %s", out)
	}
}
