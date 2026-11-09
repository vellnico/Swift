package xray

import "fmt"

type Kind string

const (
	KindVision    Kind = "vless_reality_vision"
	KindXHTTP     Kind = "vless_reality_xhttp"
	KindHysteria2 Kind = "hysteria2"
)

func (k Kind) Valid() bool {
	switch k {
	case KindVision, KindXHTTP, KindHysteria2:
		return true
	}
	return false
}

// Network is the transport-level protocol the profile listens on.
func (k Kind) Network() string {
	if k == KindHysteria2 {
		return "udp"
	}
	return "tcp"
}

func (k Kind) Reality() bool { return k == KindVision || k == KindXHTTP }

// Params are the typed profile parameters stored in profiles.params_json.
type Params struct {
	// Reality (vision and xhttp)
	PrivateKey  string   `json:"privateKey,omitempty"`
	PublicKey   string   `json:"publicKey,omitempty"`
	ShortIDs    []string `json:"shortIds,omitempty"`
	Target      string   `json:"target,omitempty"` // dest host:port probed by Reality
	ServerNames []string `json:"serverNames,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"` // uTLS fingerprint for clients

	// XHTTP
	Path string `json:"path,omitempty"`
	Mode string `json:"mode,omitempty"`

	// Hysteria2
	ObfsPassword string `json:"obfsPassword,omitempty"` // Salamander; empty disables obfuscation
}

type Profile struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Kind     Kind   `json:"kind"`
	Port     int    `json:"port"`
	Enabled  bool   `json:"enabled"`
	Manual   bool   `json:"manual"`
	Params   Params `json:"params"`
	Position int    `json:"position"`
}

// Tag identifies the profile's inbound inside Xray.
func (p Profile) Tag() string { return fmt.Sprintf("p%d", p.ID) }

type User struct {
	ID          int64
	Name        string
	VlessUUID   string
	Hy2Password string
	Enabled     bool
}

// Email is the identifier Xray uses for a user; names never reach Xray.
func (u User) Email() string { return fmt.Sprintf("u%d", u.ID) }
