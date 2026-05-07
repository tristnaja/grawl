package main

import "testing"

func TestParseYesNo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    bool
		wantErr bool
	}{
		{name: "y", in: "y", want: true},
		{name: "yes mixed case", in: "YeS", want: true},
		{name: "n", in: "n", want: false},
		{name: "no padded", in: " no ", want: false},
		{name: "invalid", in: "maybe", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseYesNo(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestParsePositiveIntOrDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		fallback int
		want     int
	}{
		{name: "valid", in: "42", fallback: 10, want: 42},
		{name: "empty", in: "", fallback: 10, want: 10},
		{name: "whitespace", in: "   ", fallback: 10, want: 10},
		{name: "invalid", in: "abc", fallback: 10, want: 10},
		{name: "zero", in: "0", fallback: 10, want: 10},
		{name: "negative", in: "-5", fallback: 10, want: 10},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parsePositiveIntOrDefault(tc.in, tc.fallback)
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
