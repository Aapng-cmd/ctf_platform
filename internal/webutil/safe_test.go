package webutil

import (
	"context"
	"errors"
	"net"
	"testing"
)

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

func TestResolveServiceHost(t *testing.T) {
	ip, err := ResolveServiceHost(context.Background(), "10.1.2.3")
	if err != nil || ip.String() != "10.1.2.3" {
		t.Fatalf("ip: %v %v", ip, err)
	}
	if _, err := ResolveServiceHost(context.Background(), "127.0.0.1"); !errors.Is(err, ErrHostDenied) {
		t.Fatalf("loopback: %v", err)
	}
	if _, err := ResolveServiceHost(context.Background(), "::ffff:169.254.169.254"); !errors.Is(err, ErrHostDenied) {
		t.Fatalf("mapped metadata: %v", err)
	}
	old := lookupHostIPs
	t.Cleanup(func() { lookupHostIPs = old })
	lookupHostIPs = func(ctx context.Context, host string) ([]net.IP, error) {
		switch host {
		case "ok.lab":
			return []net.IP{net.ParseIP("10.2.0.5")}, nil
		case "meta.lab":
			return []net.IP{net.ParseIP("169.254.169.254")}, nil
		case "mixed.lab":
			return []net.IP{net.ParseIP("10.2.0.5"), net.ParseIP("127.0.0.1")}, nil
		default:
			return nil, errors.New("nx")
		}
	}
	ip, err = ResolveServiceHost(context.Background(), "ok.lab")
	if err != nil || ip.String() != "10.2.0.5" {
		t.Fatalf("name: %v %v", ip, err)
	}
	if _, err := ResolveServiceHost(context.Background(), "meta.lab"); !errors.Is(err, ErrHostDenied) {
		t.Fatalf("metadata name: %v", err)
	}
	if _, err := ResolveServiceHost(context.Background(), "mixed.lab"); !errors.Is(err, ErrHostDenied) {
		t.Fatalf("mixed name: %v", err)
	}
	if _, err := ResolveServiceHost(context.Background(), "missing.lab"); !errors.Is(err, ErrHostUnreachable) {
		t.Fatalf("missing: %v", err)
	}
}
