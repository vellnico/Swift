// Package sub renders subscriptions for client apps in their preferred format.
package sub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/xray"
)

type Format string

const (
	FormatSingBox Format = "singbox" // Hiddify, sing-box, SFA/SFI/SFM
	FormatBase64  Format = "base64"  // Happ, v2RayTun, everything else
	FormatXray    Format = "xray"    // v2rayNG: array of full Xray configs
	FormatHTML    Format = "html"    // a browser
)

// Detect picks the format from ?format= or else from the User-Agent (SPEC 7.3).
func Detect(userAgent, override string) Format {
	switch Format(strings.ToLower(override)) {
	case FormatSingBox, FormatBase64, FormatXray, FormatHTML:
		return Format(strings.ToLower(override))
	}
	ua := strings.ToLower(userAgent)
	switch {
	case containsAny(ua, "hiddify", "sing-box", "singbox", "sfa/", "sfi/", "sfm/", "sfa ", "sfi ", "sfm "):
		return FormatSingBox
	case strings.Contains(ua, "happ"):
		return FormatBase64
	case strings.Contains(ua, "v2rayng"):
		return FormatXray
	case strings.Contains(ua, "v2raytun"):
		return FormatBase64
	case strings.Contains(ua, "mozilla"):
		return FormatHTML
	}
	return FormatBase64
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// Client says which app asked; it decides app-specific details such as the Happ routing header.
func Client(userAgent string) string {
	ua := strings.ToLower(userAgent)
	for _, c := range []string{"hiddify", "happ", "v2rayng", "v2raytun"} {
		if strings.Contains(ua, c) {
			return c
		}
	}
	return ""
}

// Routing lets the split-tunnelling preset (SPEC 8) extend client configs.
// A nil Routing sends everything through the VPN.
type Routing interface {
	SingBox(cfg map[string]any)
	Xray(cfg map[string]any)
}

// Request is one user's subscription.
type Request struct {
	User      xray.User
	Endpoints []links.Endpoint // enabled profiles only
	Routing   Routing
	// AutoSelect lets clients switch to whichever profile works (SPEC 7.5).
	AutoSelect bool
}

// ProbeURL is what clients fetch to tell a working profile from a broken one.
const ProbeURL = "https://www.gstatic.com/generate_204"

// Links returns the share links, empty for a disabled user.
func Links(r Request) ([]string, error) {
	if !r.User.Enabled {
		return nil, nil
	}
	out := make([]string, 0, len(r.Endpoints))
	for _, e := range r.Endpoints {
		l, err := links.Link(e, r.User)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// Base64 is the classic subscription: newline-separated links, base64-encoded.
func Base64(r Request) ([]byte, error) {
	ls, err := Links(r)
	if err != nil {
		return nil, err
	}
	body := strings.Join(ls, "\n")
	return []byte(base64.StdEncoding.EncodeToString([]byte(body))), nil
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
