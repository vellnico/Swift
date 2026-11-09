package service

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strconv"

	"github.com/vellnico/swift/internal/store"
)

// Panel changes where the panel answers and the public server address.
type Panel struct {
	Store *store.Store
	Ports *Ports
	// Pinned is true when swift.toml or SWIFT_* fixes the port or path,
	// or the panel shares its port with subscriptions.
	Pinned bool
	// Move switches the running panel to the new port and path.
	Move  func(port int, path string)
	Event EventFunc
}

type PanelInfo struct {
	Port    int    `json:"port"`
	Path    string `json:"path"`
	SubPort int    `json:"subPort"`
	Pinned  bool   `json:"pinned"`
	Address string `json:"address"`
}

var panelPathRe = regexp.MustCompile(`^[a-z0-9]{8,32}$`)

func (p *Panel) setting(ctx context.Context, key string) string {
	v, _ := p.Store.Setting(ctx, key)
	return v
}

func (p *Panel) Info(ctx context.Context, port int, path string) PanelInfo {
	sub, _ := strconv.Atoi(p.setting(ctx, "sub_port"))
	return PanelInfo{Port: port, Path: path, SubPort: sub, Pinned: p.Pinned, Address: p.setting(ctx, "server_address")}
}

// Change stores a new panel port and path; Move applies it after the reply.
func (p *Panel) Change(ctx context.Context, curPort int, curPath string, port int, path string) error {
	if p.Pinned {
		return &ValidationError{Fields: map[string]string{"port": "pinned"}}
	}
	fields := map[string]string{}
	if !panelPathRe.MatchString(path) {
		fields["path"] = "invalid"
	}
	var detail map[string]string
	switch {
	case port < 1024 || port > 65535:
		fields["port"] = "range"
	case port != curPort:
		var c *ErrPortConflict
		if err := p.Ports.Check(ctx, "tcp", port, 0); errors.As(err, &c) {
			fields["port"] = "busy"
			detail = map[string]string{"by": c.By}
		} else if err != nil {
			return err
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields, Detail: detail}
	}
	if port == curPort && path == curPath {
		return nil
	}
	if err := p.Store.SetSetting(ctx, "panel_port", strconv.Itoa(port)); err != nil {
		return err
	}
	if err := p.Store.SetSetting(ctx, "panel_path", path); err != nil {
		return err
	}
	if p.Event != nil {
		p.Event("warn", "panel", "panel.moved", map[string]any{"port": port})
	}
	if p.Move != nil {
		p.Move(port, path)
	}
	return nil
}

// DefaultLanguage is the panel language when the installer did not choose one.
const DefaultLanguage = "en"

// ValidLanguage reports whether the UI has a dictionary for lang.
func ValidLanguage(lang string) bool { return lang == "en" || lang == "ru" }

// Language is the default UI language chosen at install time.
func Language(ctx context.Context, st *store.Store) string {
	if st != nil {
		if v, err := st.Setting(ctx, "language"); err == nil && ValidLanguage(v) {
			return v
		}
	}
	return DefaultLanguage
}

// SetAddress changes the public IP that links and subscriptions use.
func (p *Panel) SetAddress(ctx context.Context, address string) error {
	ip := net.ParseIP(address)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return &ValidationError{Fields: map[string]string{"address": "invalid"}}
	}
	if err := p.Store.SetSetting(ctx, "server_address", ip.String()); err != nil {
		return err
	}
	if p.Event != nil {
		p.Event("info", "panel", "panel.address_changed", map[string]any{"address": ip.String()})
	}
	return nil
}
