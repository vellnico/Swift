package sub

import (
	"strings"

	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/xray"
)

// XrayJSON renders an array of complete Xray client configs, one per
// profile, which v2rayNG imports as separate servers.
func XrayJSON(r Request) ([]byte, error) {
	configs := []any{}
	if r.User.Enabled {
		for _, e := range r.Endpoints {
			cfg := xrayClient(e, r.User)
			if r.Routing != nil {
				r.Routing.Xray(cfg)
			}
			configs = append(configs, cfg)
		}
	}
	return encode(configs)
}

func xrayClient(e links.Endpoint, u xray.User) map[string]any {
	sniff := map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true}
	return map[string]any{
		"remarks": links.Name(e.Profile),
		"log":     map[string]any{"loglevel": "warning"},
		"inbounds": []any{
			map[string]any{"tag": "socks", "listen": "127.0.0.1", "port": 10808, "protocol": "socks", "settings": map[string]any{"udp": true}, "sniffing": sniff},
			map[string]any{"tag": "http", "listen": "127.0.0.1", "port": 10809, "protocol": "http", "sniffing": sniff},
		},
		"outbounds": []any{
			xrayOutbound(e, u),
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		},
		"routing": map[string]any{"domainStrategy": "AsIs", "rules": []any{}},
	}
}

func xrayOutbound(e links.Endpoint, u xray.User) map[string]any {
	p := e.Profile
	switch p.Kind {
	case xray.KindVision, xray.KindXHTTP:
		settings := map[string]any{"address": e.Host, "port": p.Port, "id": u.VlessUUID, "encryption": "none"}
		stream := map[string]any{
			"security": "reality",
			"realitySettings": map[string]any{
				"serverName":  first(p.Params.ServerNames),
				"fingerprint": orDefault(p.Params.Fingerprint, "chrome"),
				"publicKey":   p.Params.PublicKey,
				"shortId":     first(p.Params.ShortIDs),
				"spiderX":     "/",
			},
		}
		if p.Kind == xray.KindVision {
			settings["flow"] = xray.FlowVision
			stream["network"] = "raw"
		} else {
			stream["network"] = "xhttp"
			stream["xhttpSettings"] = map[string]any{"path": p.Params.Path, "mode": orDefault(p.Params.Mode, "auto")}
		}
		return map[string]any{"tag": "proxy", "protocol": "vless", "settings": settings, "streamSettings": stream}
	case xray.KindHysteria2:
		tls := map[string]any{"serverName": e.TLS.ServerName, "alpn": []string{"h3"}}
		if e.TLS.PinSHA256 != "" {
			tls["pinnedPeerCertSha256"] = strings.ToLower(e.TLS.PinSHA256)
		}
		stream := map[string]any{
			"network":          "hysteria",
			"hysteriaSettings": map[string]any{"version": 2, "auth": u.Hy2Password},
			"security":         "tls",
			"tlsSettings":      tls,
		}
		if p.Params.ObfsPassword != "" {
			stream["finalmask"] = map[string]any{"udp": []any{map[string]any{"type": "salamander", "settings": map[string]any{"password": p.Params.ObfsPassword}}}}
		}
		return map[string]any{
			"tag":            "proxy",
			"protocol":       "hysteria",
			"settings":       map[string]any{"version": 2, "address": e.Host, "port": p.Port},
			"streamSettings": stream,
		}
	}
	return map[string]any{"tag": "proxy", "protocol": "blackhole"}
}
