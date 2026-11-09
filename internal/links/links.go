// Package links builds share links (vless://, hysteria2://) for one user and profile.
package links

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/vellnico/swift/internal/xray"
)

// TLS describes the certificate clients see for Hysteria2.
type TLS struct {
	ServerName string // SNI: the domain, or the self-signed certificate's CN
	PinSHA256  string // lowercase hex of the leaf; empty for publicly trusted (ACME) certs
	CertPEM    string // self-signed certificate, for clients that trust a PEM instead of a pin
}

// Endpoint is everything needed to describe one profile to a client.
type Endpoint struct {
	Host    string // server address or domain clients connect to
	Profile xray.Profile
	TLS     TLS
}

// Name is the label shown in client apps: "Swift · <profile>".
func Name(p xray.Profile) string { return "Swift · " + p.Name }

// Link returns the share link of e for user u.
func Link(e Endpoint, u xray.User) (string, error) {
	p := e.Profile
	hostPort := net.JoinHostPort(e.Host, strconv.Itoa(p.Port))
	switch p.Kind {
	case xray.KindVision, xray.KindXHTTP:
		q := orderedQuery{}
		if p.Kind == xray.KindVision {
			q.add("type", "tcp")
		} else {
			q.add("type", "xhttp")
		}
		q.add("security", "reality")
		q.add("pbk", p.Params.PublicKey)
		q.add("fp", fingerprint(p))
		q.add("sni", firstOr(p.Params.ServerNames, ""))
		q.add("sid", firstOr(p.Params.ShortIDs, ""))
		if p.Kind == xray.KindVision {
			q.add("flow", xray.FlowVision)
		} else {
			q.add("path", p.Params.Path)
			q.add("mode", mode(p))
		}
		return fmt.Sprintf("vless://%s@%s?%s#%s", u.VlessUUID, hostPort, q.encode(), fragment(Name(p))), nil
	case xray.KindHysteria2:
		q := orderedQuery{}
		q.add("sni", e.TLS.ServerName)
		if p.Params.ObfsPassword != "" {
			q.add("obfs", "salamander")
			q.add("obfs-password", p.Params.ObfsPassword)
		}
		q.add("insecure", "0")
		if e.TLS.PinSHA256 != "" {
			q.add("pinSHA256", strings.ToLower(e.TLS.PinSHA256))
		}
		return fmt.Sprintf("hysteria2://%s@%s/?%s#%s", url.PathEscape(u.Hy2Password), hostPort, q.encode(), fragment(Name(p))), nil
	}
	return "", fmt.Errorf("unknown kind %q", p.Kind)
}

func fingerprint(p xray.Profile) string {
	if p.Params.Fingerprint == "" {
		return "chrome"
	}
	return p.Params.Fingerprint
}

func mode(p xray.Profile) string {
	if p.Params.Mode == "" {
		return "auto"
	}
	return p.Params.Mode
}

func firstOr(s []string, def string) string {
	if len(s) == 0 {
		return def
	}
	return s[0]
}

// fragment percent-encodes the link name; spaces become %20, not '+'.
func fragment(s string) string { return url.PathEscape(s) }

// orderedQuery keeps parameters in the order of SPEC 7.2 (url.Values sorts them).
type orderedQuery [][2]string

func (q *orderedQuery) add(k, v string) { *q = append(*q, [2]string{k, v}) }

func (q orderedQuery) encode() string {
	parts := make([]string, 0, len(q))
	for _, kv := range q {
		parts = append(parts, url.QueryEscape(kv[0])+"="+url.QueryEscape(kv[1]))
	}
	return strings.Join(parts, "&")
}
