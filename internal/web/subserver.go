package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/vellnico/swift/internal/qr"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/sub"
)

// SubServer answers GET /s/<token> on the subscription port (SPEC 7.3).
type SubServer struct {
	Store  *store.Store
	Subs   *service.Subscriptions
	Assets fs.FS                               // built web/sub bundle
	Dict   func(lang string) map[string]string // UI strings for the page
	Log    *slog.Logger
	// HappRouting returns the Happ "routing" header value, empty when not used (SPEC 8).
	HappRouting func(r *http.Request) string

	gz gzipCache
}

func (s *SubServer) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, "/s/")
		if !ok || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			notFound(w)
			return
		}
		if strings.HasPrefix(rest, "assets/") || strings.HasPrefix(rest, "fonts/") {
			s.asset(w, r, rest)
			return
		}
		if rest == "" || strings.Contains(rest, "/") {
			notFound(w)
			return
		}
		s.subscription(w, r, rest)
	})
}

func (s *SubServer) asset(w http.ResponseWriter, r *http.Request, name string) {
	name = path.Clean("/" + name)[1:]
	if st, err := fs.Stat(s.Assets, name); err != nil || st.IsDir() {
		notFound(w)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	s.gz.serveFile(w, r, s.Assets, name)
}

func (s *SubServer) subscription(w http.ResponseWriter, r *http.Request, token string) {
	ctx := r.Context()
	u, err := s.Store.UserByToken(ctx, token)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		s.Log.Error("subscription lookup", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	subURL, err := s.Subs.URL(ctx, token, r.Host)
	if err != nil {
		s.fail(w, err)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "qr" {
		svg, err := qr.SVG(subURL)
		if err != nil {
			s.fail(w, err)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(svg)
		return
	}
	req, err := s.Subs.Request(ctx, u, r.Host)
	if err != nil {
		s.fail(w, err)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte("Swift")))
	h.Set("profile-update-interval", "12")
	h.Set("profile-web-page-url", subURL+"?format=html")

	switch sub.Detect(r.UserAgent(), format) {
	case sub.FormatHTML:
		s.page(w, r, u, subURL, req)
		return
	case sub.FormatSingBox:
		s.write(w, "application/json", func() ([]byte, error) { return sub.SingBox(req) })
	case sub.FormatXray:
		s.write(w, "application/json", func() ([]byte, error) { return sub.XrayJSON(req) })
	default:
		if s.HappRouting != nil && sub.Client(r.UserAgent()) == "happ" && u.Enabled {
			if v := s.HappRouting(r); v != "" {
				h.Set("routing", v)
			}
		}
		s.write(w, "text/plain; charset=utf-8", func() ([]byte, error) { return sub.Base64(req) })
	}
}

func (s *SubServer) write(w http.ResponseWriter, ctype string, render func() ([]byte, error)) {
	body, err := render()
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="swift"`)
	_, _ = w.Write(body)
}

func (s *SubServer) fail(w http.ResponseWriter, err error) {
	s.Log.Error("subscription", "err", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// pageData is embedded into the HTML page as JSON (no inline script runs, CSP stays strict).
type pageData struct {
	Lang    string            `json:"lang"`
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	URL     string            `json:"url"`
	Links   []pageLink        `json:"links"`
	Strings map[string]string `json:"strings"`
}

type pageLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// PageLang picks ru or en from Accept-Language (SPEC 7.4); fallback is the
// panel language chosen at install time.
func PageLang(acceptLanguage, fallback string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "ru"), strings.HasPrefix(tag, "uk"), strings.HasPrefix(tag, "be"), strings.HasPrefix(tag, "kk"):
			return "ru"
		case strings.HasPrefix(tag, "en"):
			return "en"
		}
	}
	if service.ValidLanguage(fallback) {
		return fallback
	}
	return service.DefaultLanguage
}

var htmlLangRe = regexp.MustCompile(`<html lang="[A-Za-z-]*">`)

// setHTMLLang rewrites the lang attribute of the root element.
func setHTMLLang(html []byte, lang string) []byte {
	loc := htmlLangRe.FindIndex(html)
	if loc == nil {
		return html
	}
	out := make([]byte, 0, len(html)+4)
	out = append(out, html[:loc[0]]...)
	out = append(out, `<html lang="`+lang+`">`...)
	return append(out, html[loc[1]:]...)
}

func (s *SubServer) page(w http.ResponseWriter, r *http.Request, u store.User, subURL string, req sub.Request) {
	tmpl, err := fs.ReadFile(s.Assets, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(subURL + "\n"))
		return
	}
	lang := PageLang(r.Header.Get("Accept-Language"), service.Language(r.Context(), s.Store))
	all := s.Dict(lang)
	strs := map[string]string{}
	for k, v := range all {
		if strings.HasPrefix(k, "sub.") {
			strs[k] = v
		}
	}
	data := pageData{Lang: lang, Name: u.Name, Enabled: u.Enabled, URL: subURL, Strings: strs, Links: []pageLink{}}
	if ls, err := sub.Links(req); err == nil {
		for i, l := range ls {
			data.Links = append(data.Links, pageLink{Name: req.Endpoints[i].Profile.Name, URL: l})
		}
	}
	js, err := json.Marshal(data) // escapes <, > and & so the block cannot end early
	if err != nil {
		s.fail(w, err)
		return
	}
	block := []byte(`<script id="swift-data" type="application/json">` + string(js) + `</script>`)
	html := bytes.Replace(tmpl, []byte("<!--swift-data-->"), block, 1)
	html = setHTMLLang(html, lang)
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	writeBody(w, r, html)
}
