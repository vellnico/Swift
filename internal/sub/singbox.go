package sub

import (
	"strings"

	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/xray"
)

// SingBox renders a sing-box client config (Hiddify's primary format).
// XHTTP is skipped: sing-box has no XHTTP transport.
func SingBox(r Request) ([]byte, error) {
	cfg := map[string]any{
		"log": map[string]any{"level": "warn", "timestamp": true},
	}
	var outbounds []any
	var tags []string
	if r.User.Enabled {
		for _, e := range r.Endpoints {
			ob := singBoxOutbound(e, r.User)
			if ob == nil {
				continue
			}
			outbounds = append(outbounds, ob)
			tags = append(tags, ob["tag"].(string))
		}
	}
	if len(tags) == 0 {
		// a valid config that connects nowhere: the client keeps the subscription
		cfg["outbounds"] = []any{map[string]any{"type": "direct", "tag": "direct"}}
		cfg["route"] = map[string]any{"rules": []any{map[string]any{"action": "reject"}}, "final": "direct"}
		return encode(cfg)
	}
	head := []any{map[string]any{"type": "selector", "tag": "proxy", "outbounds": tags, "default": tags[0]}}
	if r.AutoSelect && len(tags) > 1 {
		head = []any{
			map[string]any{"type": "selector", "tag": "proxy", "outbounds": append([]string{"auto"}, tags...), "default": "auto"},
			// a profile cut by DPI fails the probe within a minute; 100 ms of tolerance keeps it from flapping
			map[string]any{"type": "urltest", "tag": "auto", "outbounds": tags, "url": ProbeURL, "interval": "1m", "tolerance": 100},
		}
	}
	outbounds = append(head, outbounds...)
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})
	cfg["outbounds"] = outbounds
	cfg["route"] = map[string]any{"final": "proxy", "auto_detect_interface": true, "rules": []any{}}
	if r.Routing != nil {
		r.Routing.SingBox(cfg)
	}
	return encode(cfg)
}

func singBoxOutbound(e links.Endpoint, u xray.User) map[string]any {
	p := e.Profile
	tag := links.Name(p)
	switch p.Kind {
	case xray.KindVision:
		return map[string]any{
			"type":        "vless",
			"tag":         tag,
			"server":      e.Host,
			"server_port": p.Port,
			"uuid":        u.VlessUUID,
			"flow":        xray.FlowVision,
			"tls": map[string]any{
				"enabled":     true,
				"server_name": first(p.Params.ServerNames),
				"utls":        map[string]any{"enabled": true, "fingerprint": orDefault(p.Params.Fingerprint, xray.DefaultFingerprint)},
				"reality":     map[string]any{"enabled": true, "public_key": p.Params.PublicKey, "short_id": first(p.Params.ShortIDs)},
			},
		}
	case xray.KindHysteria2:
		tls := map[string]any{
			"enabled":     true,
			"server_name": e.TLS.ServerName,
			"insecure":    false,
			"alpn":        []string{"h3"},
		}
		if e.TLS.CertPEM != "" {
			// sing-box has no certificate pin; trusting the exact self-signed certificate is equivalent
			tls["certificate"] = strings.Split(strings.TrimSpace(e.TLS.CertPEM), "\n")
		}
		ob := map[string]any{
			"type":        "hysteria2",
			"tag":         tag,
			"server":      e.Host,
			"server_port": p.Port,
			"password":    u.Hy2Password,
			"tls":         tls,
		}
		if p.Params.ObfsPassword != "" {
			ob["obfs"] = map[string]any{"type": "salamander", "password": p.Params.ObfsPassword}
		}
		return ob
	}
	return nil
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
