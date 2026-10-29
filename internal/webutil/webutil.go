package webutil

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"golang.org/x/crypto/bcrypt"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Table),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func Markdown(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(buf.String())
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func RandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func IsLoopback(ip string) bool {
	host := ip
	if h, _, err := net.SplitHostPort(ip); err == nil {
		host = h
	}
	parsed := net.ParseIP(host)
	if parsed == nil {
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	}
	return parsed.IsLoopback()
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func ValidEmail(s string) bool {
	return emailRe.MatchString(s)
}

// SafeNext keeps a same-site path. Protocol-relative and backslash targets are dropped.
func SafeNext(next, fallback string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return fallback
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return fallback
	}
	if strings.ContainsAny(next, "\\\t\r\n") {
		return fallback
	}
	u, err := url.Parse(next)
	if err != nil || u.IsAbs() || u.Host != "" || u.Scheme != "" {
		return fallback
	}
	return next
}

// AllowedServiceHost is a lab address a checker may dial. Loopback, link-local,
// and the platform's own service names are rejected. Private lab ranges stay allowed.
func AllowedServiceHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 {
		return false
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, " \t\r\n/\\@?#") {
		return false
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return false
	}
	switch strings.ToLower(host) {
	case "localhost", "postgres", "redis", "web", "nginx", "host.docker.internal":
		return false
	}
	if strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false
		}
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

// UploadPath joins a stored file name onto the upload directory and rejects traversal.
func UploadPath(dir, location string) (string, error) {
	loc := filepath.Clean(location)
	if loc == "." || loc == "" || filepath.IsAbs(loc) || loc == ".." || strings.HasPrefix(loc, ".."+string(os.PathSeparator)) {
		return "", os.ErrInvalid
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, loc)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", os.ErrInvalid
	}
	return abs, nil
}

func ValidName(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

type RateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	window  time.Duration
	maxHits int
}

func NewRateLimiter(window time.Duration, maxHits int) *RateLimiter {
	return &RateLimiter{hits: map[string][]time.Time{}, window: window, maxHits: maxHits}
}

func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cut := now.Add(-r.window)
	arr := r.hits[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.maxHits {
		r.hits[key] = kept
		return false
	}
	r.hits[key] = append(kept, now)
	return true
}

var Countries = []struct {
	Code string
	Name string
}{
	{"AF", "Afghanistan"}, {"AL", "Albania"}, {"DZ", "Algeria"}, {"AR", "Argentina"},
	{"AM", "Armenia"}, {"AU", "Australia"}, {"AT", "Austria"}, {"AZ", "Azerbaijan"},
	{"BY", "Belarus"}, {"BE", "Belgium"}, {"BR", "Brazil"}, {"BG", "Bulgaria"},
	{"CA", "Canada"}, {"CL", "Chile"}, {"CN", "China"}, {"CO", "Colombia"},
	{"HR", "Croatia"}, {"CY", "Cyprus"}, {"CZ", "Czechia"}, {"DK", "Denmark"},
	{"EG", "Egypt"}, {"EE", "Estonia"}, {"FI", "Finland"}, {"FR", "France"},
	{"GE", "Georgia"}, {"DE", "Germany"}, {"GR", "Greece"}, {"HK", "Hong Kong"},
	{"HU", "Hungary"}, {"IS", "Iceland"}, {"IN", "India"}, {"ID", "Indonesia"},
	{"IR", "Iran"}, {"IQ", "Iraq"}, {"IE", "Ireland"}, {"IL", "Israel"},
	{"IT", "Italy"}, {"JP", "Japan"}, {"KZ", "Kazakhstan"}, {"KE", "Kenya"},
	{"KR", "South Korea"}, {"KG", "Kyrgyzstan"}, {"LV", "Latvia"}, {"LT", "Lithuania"},
	{"LU", "Luxembourg"}, {"MY", "Malaysia"}, {"MX", "Mexico"}, {"MD", "Moldova"},
	{"MN", "Mongolia"}, {"ME", "Montenegro"}, {"MA", "Morocco"}, {"NL", "Netherlands"},
	{"NZ", "New Zealand"}, {"NG", "Nigeria"}, {"NO", "Norway"}, {"PK", "Pakistan"},
	{"PE", "Peru"}, {"PH", "Philippines"}, {"PL", "Poland"}, {"PT", "Portugal"},
	{"RO", "Romania"}, {"RU", "Russia"}, {"SA", "Saudi Arabia"}, {"RS", "Serbia"},
	{"SG", "Singapore"}, {"SK", "Slovakia"}, {"SI", "Slovenia"}, {"ZA", "South Africa"},
	{"ES", "Spain"}, {"SE", "Sweden"}, {"CH", "Switzerland"}, {"TW", "Taiwan"},
	{"TH", "Thailand"}, {"TR", "Turkey"}, {"UA", "Ukraine"}, {"AE", "United Arab Emirates"},
	{"GB", "United Kingdom"}, {"US", "United States"}, {"UZ", "Uzbekistan"}, {"VN", "Vietnam"},
}
