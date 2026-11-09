// Package web embeds the built frontend bundles and the UI dictionaries.
package web

import (
	"embed"
	"encoding/json"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

//go:embed all:sub/dist
var subDist embed.FS

//go:embed src/lib/i18n/ru.json src/lib/i18n/en.json
var dicts embed.FS

func sub(f embed.FS, dir string) fs.FS {
	s, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return s
}

// Panel returns the built SPA rooted at its index.html.
func Panel() fs.FS { return sub(dist, "dist") }

// SubPage returns the built subscription page bundle.
func SubPage() fs.FS { return sub(subDist, "sub/dist") }

// Dictionary returns the UI strings of a language ("en" or "ru"); anything else gets English.
func Dictionary(lang string) map[string]string {
	if lang != "ru" {
		lang = "en"
	}
	b, err := dicts.ReadFile("src/lib/i18n/" + lang + ".json")
	if err != nil {
		panic(err)
	}
	out := map[string]string{}
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}
