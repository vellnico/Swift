package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Every schema field needs a label and a hint in both UI languages (SPEC 6.3).
func TestSchemaFieldsHaveTranslations(t *testing.T) {
	for _, lang := range []string{"ru", "en"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "i18n", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		dict := map[string]string{}
		if err := json.Unmarshal(body, &dict); err != nil {
			t.Fatal(err)
		}
		for kind, fields := range Schema() {
			if dict["profiles.kind."+string(kind)+".title"] == "" || dict["profiles.kind."+string(kind)+".desc"] == "" {
				t.Errorf("%s: kind %s has no title/description", lang, kind)
			}
			for _, f := range fields {
				for _, suffix := range []string{".label", ".hint"} {
					if dict["profiles.fields."+f.Name+suffix] == "" {
						t.Errorf("%s: missing profiles.fields.%s%s", lang, f.Name, suffix)
					}
				}
			}
		}
	}
}

func TestSchemaOrder(t *testing.T) {
	s := Schema()
	names := func(fs []Field) []string {
		out := []string{}
		for _, f := range fs {
			out = append(out, f.Name)
		}
		return out
	}
	got := names(s[KindXHTTP])
	want := []string{"name", "port", "serverNames", "target", "path", "mode", "fingerprint", "shortIds", "privateKey"}
	if len(got) != len(want) {
		t.Fatalf("xhttp fields %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("xhttp fields %v", got)
		}
	}
	if s[KindVision][1].Recommended != 443 || s[KindXHTTP][1].Recommended != 8443 {
		t.Fatal("recommended ports")
	}
}
