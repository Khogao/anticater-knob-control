package main

// Two UI languages: Vietnamese and English, written inline as L("vi", "en").

import (
	"os"
	"strings"
)

var langEN bool

func L(vi, en string) string {
	if langEN {
		return en
	}
	return vi
}

// setLanguage applies "vi" or "en"; anything else follows $LANGUAGE / $LC_ALL /
// $LC_MESSAGES / $LANG: Vietnamese when that is Vietnamese, English otherwise.
func setLanguage(code string) {
	switch code {
	case "vi":
		langEN = false
	case "en":
		langEN = true
	default:
		langEN = true
		for _, v := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
			if s := os.Getenv(v); s != "" {
				langEN = !strings.HasPrefix(strings.ToLower(s), "vi")
				break
			}
		}
	}
}

func languageCode() string {
	if langEN {
		return "en"
	}
	return "vi"
}
