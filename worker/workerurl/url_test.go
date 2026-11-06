package workerurl

import "testing"

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"https://example.org", true}, {"https://example.org:8443/base", true},
		{"http://localhost:3000", true}, {"http://127.0.0.1:3000", true},
		{"http://[::1]:3000", true}, {"http://example.org", false},
		{"http://192.168.1.1", false}, {"https://attacker@example.org", false},
		{"https://example.org/?secret=x", false}, {"https://example.org/#x", false}, {"", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if (Validate(tc.value) == nil) != tc.ok {
				t.Fatalf("unexpected validation result for %q", tc.value)
			}
		})
	}
}
