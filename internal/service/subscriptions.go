package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/vellnico/swift/internal/links"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/sub"
	"github.com/vellnico/swift/internal/xray"
)

// Subscriptions knows how clients reach this server.
type Subscriptions struct {
	Store *store.Store
	Certs *Certs
	// Routing returns the active split-tunnelling preset (SPEC 8); nil = everything via VPN.
	Routing func(ctx context.Context) (sub.Routing, error)
	// Scheme of the subscription URL: "http" until certificates are served (phase 7).
	Scheme func(ctx context.Context) string
}

// Host is what clients connect to: the domain, else the server address, else fallback
// (the host the request came to).
func (s *Subscriptions) Host(ctx context.Context, fallback string) string {
	for _, key := range []string{"domain", "server_address"} {
		if v, err := s.Store.Setting(ctx, key); err == nil && v != "" {
			return v
		}
	}
	if h, _, err := net.SplitHostPort(fallback); err == nil {
		return h
	}
	return fallback
}

// URL is the subscription link of a token.
func (s *Subscriptions) URL(ctx context.Context, token, fallbackHost string) (string, error) {
	port, err := s.Store.Setting(ctx, "sub_port")
	if err != nil {
		return "", fmt.Errorf("subscription port: %w", err)
	}
	scheme := "http"
	if s.Scheme != nil {
		scheme = s.Scheme(ctx)
	}
	return fmt.Sprintf("%s://%s/s/%s", scheme, net.JoinHostPort(s.Host(ctx, fallbackHost), port), token), nil
}

// TLS describes the certificate Hysteria2 clients will see.
func (s *Subscriptions) TLS(ctx context.Context) (links.TLS, error) {
	c, err := s.Store.Certificate(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return links.TLS{}, nil
	}
	if err != nil {
		return links.TLS{}, err
	}
	t := links.TLS{ServerName: c.Domain}
	if c.Kind == "self_signed" {
		t.PinSHA256 = c.SHA256Hex
		if pem, err := os.ReadFile(c.CertPath); err == nil {
			t.CertPEM = string(pem)
		}
	}
	return t, nil
}

// Endpoints lists the enabled profiles as seen from a client.
func (s *Subscriptions) Endpoints(ctx context.Context, fallbackHost string) ([]links.Endpoint, error) {
	profiles, err := s.Store.Profiles(ctx)
	if err != nil {
		return nil, err
	}
	tls, err := s.TLS(ctx)
	if err != nil {
		return nil, err
	}
	host := s.Host(ctx, fallbackHost)
	out := []links.Endpoint{}
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		e := links.Endpoint{Host: host, Profile: p}
		if p.Kind == xray.KindHysteria2 {
			e.TLS = tls
		}
		out = append(out, e)
	}
	return out, nil
}

// Request builds the subscription of u.
func (s *Subscriptions) Request(ctx context.Context, u store.User, fallbackHost string) (sub.Request, error) {
	eps, err := s.Endpoints(ctx, fallbackHost)
	if err != nil {
		return sub.Request{}, err
	}
	r := sub.Request{User: ToXrayUser(u), Endpoints: eps}
	if s.Routing != nil {
		if r.Routing, err = s.Routing(ctx); err != nil {
			return r, err
		}
	}
	return r, nil
}

// ProfileLinks returns a share link per enabled profile for the user's card.
func (s *Subscriptions) ProfileLinks(ctx context.Context, u store.User, fallbackHost string) ([]ProfileLink, error) {
	eps, err := s.Endpoints(ctx, fallbackHost)
	if err != nil {
		return nil, err
	}
	out := make([]ProfileLink, 0, len(eps))
	for _, e := range eps {
		l, err := links.Link(e, ToXrayUser(u))
		if err != nil {
			return nil, err
		}
		out = append(out, ProfileLink{ProfileID: e.Profile.ID, Name: e.Profile.Name, Kind: string(e.Profile.Kind), URL: l})
	}
	return out, nil
}

type ProfileLink struct {
	ProfileID int64  `json:"profileId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
}

// EnsureSubPort picks the subscription port once (different from the panel's).
func EnsureSubPort(ctx context.Context, st *store.Store, configured, panelPort int, randPort func() int) (int, error) {
	if configured != 0 {
		return configured, st.SetSetting(ctx, "sub_port", strconv.Itoa(configured))
	}
	if v, err := st.Setting(ctx, "sub_port"); err == nil {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			return p, nil
		}
	}
	p := randPort()
	for p == panelPort {
		p = randPort()
	}
	if err := st.SetSetting(ctx, "sub_port", strconv.Itoa(p)); err != nil {
		return 0, err
	}
	return p, nil
}
