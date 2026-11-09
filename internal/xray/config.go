package xray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
)

// CertFiles points at the PEM certificate and key used by Hysteria2.
type CertFiles struct {
	CertFile string
	KeyFile  string
}

// Input is everything the generator needs; the database is the only source of it.
type Input struct {
	APIPort  int
	Profiles []Profile
	Users    []User
	Cert     *CertFiles
	LogLevel string
}

type jsonConfig struct {
	Log       jsonLog        `json:"log"`
	API       jsonAPI        `json:"api"`
	Stats     struct{}       `json:"stats"`
	Policy    jsonPolicy     `json:"policy"`
	Inbounds  []jsonInbound  `json:"inbounds"`
	Outbounds []jsonOutbound `json:"outbounds"`
	Routing   jsonRouting    `json:"routing"`
}

type jsonLog struct {
	Level  string `json:"loglevel"`
	Access string `json:"access"`
	DNSLog bool   `json:"dnsLog"`
}

type jsonAPI struct {
	Tag      string   `json:"tag"`
	Listen   string   `json:"listen"`
	Services []string `json:"services"`
}

type jsonPolicy struct {
	Levels map[string]jsonLevel `json:"levels"`
	System jsonSystemPolicy     `json:"system"`
}

type jsonLevel struct {
	Handshake    int `json:"handshake"`
	ConnIdle     int `json:"connIdle"`
	UplinkOnly   int `json:"uplinkOnly"`
	DownlinkOnly int `json:"downlinkOnly"`
}

type jsonSystemPolicy struct {
	StatsInboundUplink    bool `json:"statsInboundUplink"`
	StatsInboundDownlink  bool `json:"statsInboundDownlink"`
	StatsOutboundUplink   bool `json:"statsOutboundUplink"`
	StatsOutboundDownlink bool `json:"statsOutboundDownlink"`
}

type jsonInbound struct {
	Tag            string       `json:"tag"`
	Port           int          `json:"port"`
	Protocol       string       `json:"protocol"`
	Settings       any          `json:"settings"`
	StreamSettings jsonStream   `json:"streamSettings"`
	Sniffing       jsonSniffing `json:"sniffing"`
}

type jsonSniffing struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
	RouteOnly    bool     `json:"routeOnly"`
}

type jsonStream struct {
	Network         string         `json:"network"`
	Security        string         `json:"security"`
	RealitySettings *jsonReality   `json:"realitySettings,omitempty"`
	TLSSettings     *jsonTLS       `json:"tlsSettings,omitempty"`
	XHTTPSettings   *jsonXHTTP     `json:"xhttpSettings,omitempty"`
	HysteriaSetting *jsonHysteria  `json:"hysteriaSettings,omitempty"`
	FinalMask       *jsonFinalMask `json:"finalmask,omitempty"`
}

type jsonReality struct {
	Show        bool     `json:"show"`
	Target      string   `json:"target"`
	Xver        int      `json:"xver"`
	ServerNames []string `json:"serverNames"`
	PrivateKey  string   `json:"privateKey"`
	ShortIDs    []string `json:"shortIds"`
}

type jsonTLS struct {
	ALPN         []string   `json:"alpn"`
	MinVersion   string     `json:"minVersion"`
	Certificates []jsonCert `json:"certificates"`
}

type jsonCert struct {
	CertificateFile string `json:"certificateFile"`
	KeyFile         string `json:"keyFile"`
}

type jsonXHTTP struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type jsonHysteria struct {
	Version int `json:"version"`
}

type jsonFinalMask struct {
	UDP []jsonMask `json:"udp"`
}

type jsonMask struct {
	Type     string `json:"type"`
	Settings any    `json:"settings"`
}

type vlessSettings struct {
	Clients    []vlessClient `json:"clients"`
	Decryption string        `json:"decryption"`
}

type vlessClient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Flow  string `json:"flow,omitempty"`
}

type hysteriaSettings struct {
	Version int              `json:"version"`
	Clients []hysteriaClient `json:"clients"`
}

type hysteriaClient struct {
	Auth  string `json:"auth"`
	Email string `json:"email"`
}

type jsonOutbound struct {
	Tag      string   `json:"tag"`
	Protocol string   `json:"protocol"`
	Settings struct{} `json:"settings"`
}

type jsonRouting struct {
	DomainStrategy string     `json:"domainStrategy"`
	Rules          []jsonRule `json:"rules"`
}

type jsonRule struct {
	IP          []string `json:"ip,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

const FlowVision = "xtls-rprx-vision"

// Generate renders the full Xray config for the enabled profiles and users.
func Generate(in Input) ([]byte, error) {
	if in.APIPort <= 0 {
		return nil, fmt.Errorf("api port is not set")
	}
	level := in.LogLevel
	if level == "" {
		level = "warning"
	}
	cfg := jsonConfig{
		Log: jsonLog{Level: level, Access: "none"},
		API: jsonAPI{Tag: "api", Listen: net.JoinHostPort("127.0.0.1", strconv.Itoa(in.APIPort)), Services: []string{"HandlerService", "StatsService"}},
		Policy: jsonPolicy{
			Levels: map[string]jsonLevel{"0": {Handshake: 4, ConnIdle: 300, UplinkOnly: 2, DownlinkOnly: 5}},
			System: jsonSystemPolicy{true, true, true, true},
		},
		Inbounds: []jsonInbound{},
		Outbounds: []jsonOutbound{
			{Tag: "direct", Protocol: "freedom"},
			{Tag: "block", Protocol: "blackhole"},
		},
		Routing: jsonRouting{
			DomainStrategy: "AsIs",
			// clients must not reach the server's own private networks
			Rules: []jsonRule{{IP: []string{"geoip:private"}, OutboundTag: "block"}},
		},
	}

	users := make([]User, 0, len(in.Users))
	for _, u := range in.Users {
		if u.Enabled {
			users = append(users, u)
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })

	profiles := append([]Profile(nil), in.Profiles...)
	sort.SliceStable(profiles, func(i, j int) bool { return profiles[i].Position < profiles[j].Position })
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		ib, err := inbound(p, users, in.Cert)
		if err != nil {
			return nil, fmt.Errorf("profile %q: %w", p.Name, err)
		}
		cfg.Inbounds = append(cfg.Inbounds, ib)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return buf.Bytes(), nil
}

var sniffing = jsonSniffing{Enabled: true, DestOverride: []string{"http", "tls", "quic"}, RouteOnly: true}

func inbound(p Profile, users []User, cert *CertFiles) (jsonInbound, error) {
	if p.Port < 1 || p.Port > 65535 {
		return jsonInbound{}, fmt.Errorf("invalid port %d", p.Port)
	}
	ib := jsonInbound{Tag: p.Tag(), Port: p.Port, Sniffing: sniffing}
	switch p.Kind {
	case KindVision, KindXHTTP:
		if p.Params.PrivateKey == "" || p.Params.Target == "" || len(p.Params.ServerNames) == 0 || len(p.Params.ShortIDs) == 0 {
			return ib, fmt.Errorf("incomplete Reality parameters")
		}
		flow := ""
		if p.Kind == KindVision {
			flow = FlowVision
		}
		clients := make([]vlessClient, 0, len(users))
		for _, u := range users {
			clients = append(clients, vlessClient{ID: u.VlessUUID, Email: u.Email(), Flow: flow})
		}
		ib.Protocol = "vless"
		ib.Settings = vlessSettings{Clients: clients, Decryption: "none"}
		ib.StreamSettings = jsonStream{
			Security: "reality",
			RealitySettings: &jsonReality{
				Target:      p.Params.Target,
				ServerNames: p.Params.ServerNames,
				PrivateKey:  p.Params.PrivateKey,
				ShortIDs:    p.Params.ShortIDs,
			},
		}
		if p.Kind == KindVision {
			ib.StreamSettings.Network = "raw"
		} else {
			mode := p.Params.Mode
			if mode == "" {
				mode = "auto"
			}
			ib.StreamSettings.Network = "xhttp"
			ib.StreamSettings.XHTTPSettings = &jsonXHTTP{Path: p.Params.Path, Mode: mode}
		}
	case KindHysteria2:
		if cert == nil {
			return ib, fmt.Errorf("hysteria2 needs a certificate")
		}
		clients := make([]hysteriaClient, 0, len(users))
		for _, u := range users {
			clients = append(clients, hysteriaClient{Auth: u.Hy2Password, Email: u.Email()})
		}
		ib.Protocol = "hysteria"
		ib.Settings = hysteriaSettings{Version: 2, Clients: clients}
		ib.StreamSettings = jsonStream{
			Network:         "hysteria",
			Security:        "tls",
			HysteriaSetting: &jsonHysteria{Version: 2},
			TLSSettings: &jsonTLS{
				ALPN:         []string{"h3"},
				MinVersion:   "1.3",
				Certificates: []jsonCert{{CertificateFile: cert.CertFile, KeyFile: cert.KeyFile}},
			},
		}
		if p.Params.ObfsPassword != "" {
			ib.StreamSettings.FinalMask = &jsonFinalMask{UDP: []jsonMask{{
				Type:     "salamander",
				Settings: map[string]string{"password": p.Params.ObfsPassword},
			}}}
		}
	default:
		return ib, fmt.Errorf("unknown kind %q", p.Kind)
	}
	return ib, nil
}
