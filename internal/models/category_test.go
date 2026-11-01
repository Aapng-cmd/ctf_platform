package models

import "testing"

func TestCanonicalCategory(t *testing.T) {
	cases := map[string]string{
		"Web":         "Веб",
		"Crypto":      "Криптография",
		"Forensics":   "Форензика",
		"Osint":       "OSINT",
		"PWN":         "PWN",
		"Reverse":     "Реверс",
		"PPC":         "PPC",
		"Misc":        "Misc",
		"Stego":       "Стеганография",
		"Квест":       "Квест",
		"Web - Quest": "Квест",
		"Web - Квест": "Квест",
		"  ":          "Misc",
		"Custom lab":  "Custom lab",
	}
	for in, want := range cases {
		if got := CanonicalCategory(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}
