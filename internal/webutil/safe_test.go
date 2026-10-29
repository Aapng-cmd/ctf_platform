package webutil

import "testing"

func TestSafeNext(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/challenges", "/challenges"},
		{"/teams/join?x=1", "/teams/join?x=1"},
		{"", "/challenges"},
		{"https://evil.example", "/challenges"},
		{"//evil.example", "/challenges"},
		{`/\evil.example`, "/challenges"},
		{"/foo\nbar", "/challenges"},
	}
	for _, tc := range cases {
		if got := SafeNext(tc.in, "/challenges"); got != tc.want {
			t.Errorf("SafeNext(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAllowedServiceHost(t *testing.T) {
	if !AllowedServiceHost("10.0.1.10") || !AllowedServiceHost("vulnbox.team") {
		t.Fatal("lab addresses should be allowed")
	}
	for _, host := range []string{"", "localhost", "127.0.0.1", "::1", "169.254.169.254", "postgres", "redis", "10.0.0.1:8080", "http://10.0.0.1"} {
		if AllowedServiceHost(host) {
			t.Errorf("host %q should be rejected", host)
		}
	}
}
