package webutil

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/crypto/bcrypt"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Table),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
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
