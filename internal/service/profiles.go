package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// Profiles implements creating and editing inbound profiles.
type Profiles struct {
	Store  *store.Store
	Xray   *Xray
	Ports  *Ports
	Prober xray.Prober
	Certs  *Certs
}

// ValidationError maps field names to i18n error codes (profiles.errors.<code>).
type ValidationError struct {
	Fields map[string]string
	Detail map[string]string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for f, c := range e.Fields {
		parts = append(parts, f+": "+c)
	}
	slices.Sort(parts)
	return "invalid profile: " + strings.Join(parts, ", ")
}

func (e *ValidationError) add(field, code string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	if _, ok := e.Fields[field]; !ok {
		e.Fields[field] = code
	}
}

var defaultNames = map[xray.Kind]string{
	xray.KindVision:    "VLESS Reality",
	xray.KindXHTTP:     "VLESS XHTTP",
	xray.KindHysteria2: "Hysteria2",
}

func (s *Profiles) uniqueName(ctx context.Context, base string) (string, error) {
	profiles, err := s.Store.Profiles(ctx)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, p := range profiles {
		taken[p.Name] = true
	}
	name := base
	for i := 2; taken[name]; i++ {
		name = base + " " + strconv.Itoa(i)
	}
	return name, nil
}

func randomPath() string {
	return "/" + auth.RandomString("abcdefghijklmnopqrstuvwxyz0123456789", 10)
}

func randomSecret() string {
	return auth.RandomString("abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789", 20)
}

// Defaults fills automatic parameters for a new profile of kind k.
func (s *Profiles) Defaults(ctx context.Context, k xray.Kind) (xray.Params, error) {
	var p xray.Params
	switch k {
	case xray.KindVision, xray.KindXHTTP:
		priv, pub, err := xray.RealityKeyPair()
		if err != nil {
			return p, err
		}
		sni, err := xray.PickSNI(ctx, s.Prober, xray.SNICandidates)
		if err != nil {
			return p, err
		}
		p = xray.Params{PrivateKey: priv, PublicKey: pub, ShortIDs: xray.ShortIDs(3), Target: sni + ":443", ServerNames: []string{sni}, Fingerprint: "chrome"}
		if k == xray.KindXHTTP {
			p.Path, p.Mode = randomPath(), "auto"
		}
	case xray.KindHysteria2:
		if s.Certs != nil {
			if _, err := s.Certs.Ensure(ctx); err != nil {
				return p, fmt.Errorf("certificate: %w", err)
			}
		}
		p.ObfsPassword = randomSecret()
	default:
		return p, fmt.Errorf("unknown kind %q", k)
	}
	return p, nil
}

// Create makes a working profile of kind k with one call (SPEC 6.2).
func (s *Profiles) Create(ctx context.Context, k xray.Kind) (xray.Profile, error) {
	if !k.Valid() {
		return xray.Profile{}, &ValidationError{Fields: map[string]string{"kind": "invalid"}}
	}
	port, err := s.Ports.Pick(ctx, k)
	if err != nil {
		return xray.Profile{}, err
	}
	params, err := s.Defaults(ctx, k)
	if err != nil {
		return xray.Profile{}, err
	}
	name, err := s.uniqueName(ctx, defaultNames[k])
	if err != nil {
		return xray.Profile{}, err
	}
	p, err := s.Store.CreateProfile(ctx, xray.Profile{Name: name, Kind: k, Port: port, Enabled: true, Params: params})
	if err != nil {
		return p, err
	}
	if err := s.Xray.Apply(ctx); err != nil {
		_ = s.Store.DeleteProfile(context.WithoutCancel(ctx), p.ID)
		return p, err
	}
	return p, nil
}

// Update is the editable part of a profile plus fields to regenerate on the server.
type Update struct {
	Name       *string      `json:"name"`
	Port       *int         `json:"port"`
	Enabled    *bool        `json:"enabled"`
	Params     *xray.Params `json:"params"`
	Regenerate []string     `json:"regenerate"`
}

// Update validates, saves and applies the change; the row is restored if xray rejects it.
func (s *Profiles) Update(ctx context.Context, id int64, u Update) (xray.Profile, error) {
	old, err := s.Store.Profile(ctx, id)
	if err != nil {
		return old, err
	}
	next := old
	if u.Name != nil {
		next.Name = strings.TrimSpace(*u.Name)
	}
	if u.Port != nil {
		next.Port = *u.Port
	}
	if u.Enabled != nil {
		next.Enabled = *u.Enabled
	}
	if u.Params != nil {
		next.Params = *u.Params
		// fields that do not belong to this kind are ignored
		next.Params = keepKindParams(next.Kind, next.Params)
	}
	for _, f := range u.Regenerate {
		if err := regenerate(&next, f); err != nil {
			return old, err
		}
	}
	if next.Kind.Reality() {
		if pub, err := xray.PublicKeyFor(next.Params.PrivateKey); err == nil {
			next.Params.PublicKey = pub
		}
	}
	if err := s.Validate(ctx, next); err != nil {
		return old, err
	}
	if !samePlumbing(old, next) {
		next.Manual = true
	}
	if err := s.Store.UpdateProfile(ctx, next); err != nil {
		return old, err
	}
	if err := s.Xray.Apply(ctx); err != nil {
		_ = s.Store.UpdateProfile(context.WithoutCancel(ctx), old)
		return old, err
	}
	return next, nil
}

// samePlumbing is true when only the name or the on/off switch changed.
func samePlumbing(a, b xray.Profile) bool {
	a.Name, b.Name, a.Enabled, b.Enabled, a.Manual, b.Manual = "", "", false, false, false, false
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func keepKindParams(k xray.Kind, p xray.Params) xray.Params {
	switch k {
	case xray.KindVision:
		p.Path, p.Mode, p.ObfsPassword = "", "", ""
	case xray.KindXHTTP:
		p.ObfsPassword = ""
	case xray.KindHysteria2:
		p = xray.Params{ObfsPassword: p.ObfsPassword}
	}
	return p
}

func regenerate(p *xray.Profile, field string) error {
	switch {
	case field == "privateKey" && p.Kind.Reality():
		priv, pub, err := xray.RealityKeyPair()
		if err != nil {
			return err
		}
		p.Params.PrivateKey, p.Params.PublicKey = priv, pub
	case field == "shortIds" && p.Kind.Reality():
		p.Params.ShortIDs = xray.ShortIDs(3)
	case field == "path" && p.Kind == xray.KindXHTTP:
		p.Params.Path = randomPath()
	case field == "obfsPassword" && p.Kind == xray.KindHysteria2:
		p.Params.ObfsPassword = randomSecret()
	default:
		return &ValidationError{Fields: map[string]string{field: "not_regenerable"}}
	}
	return nil
}

var (
	hostRe    = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
	shortIDRe = regexp.MustCompile(`^([0-9a-f]{2}){0,8}$`)
	pathRe    = regexp.MustCompile(`^/[A-Za-z0-9/_-]{0,63}$`)
)

// Validate checks every field of p, including port conflicts.
func (s *Profiles) Validate(ctx context.Context, p xray.Profile) error {
	ve := &ValidationError{}
	if n := utf8.RuneCountInString(p.Name); n == 0 || n > 64 {
		ve.add("name", "required")
	}
	if err := s.Ports.Check(ctx, p.Kind.Network(), p.Port, p.ID); err != nil {
		var c *ErrPortConflict
		if errors.As(err, &c) {
			ve.add("port", "port_conflict")
			ve.Detail = map[string]string{"by": c.By, "network": strings.ToUpper(c.Network)}
		} else {
			ve.add("port", "invalid")
		}
	}
	switch p.Kind {
	case xray.KindVision, xray.KindXHTTP:
		if len(p.Params.ServerNames) == 0 {
			ve.add("serverNames", "required")
		}
		for _, sn := range p.Params.ServerNames {
			if !hostRe.MatchString(sn) {
				ve.add("serverNames", "invalid")
			}
		}
		host, port, err := net.SplitHostPort(p.Params.Target)
		if n, _ := strconv.Atoi(port); err != nil || !hostRe.MatchString(host) || n < 1 || n > 65535 {
			ve.add("target", "invalid")
		}
		if !slices.Contains(xray.Fingerprints, p.Params.Fingerprint) {
			ve.add("fingerprint", "invalid")
		}
		if len(p.Params.ShortIDs) == 0 {
			ve.add("shortIds", "required")
		}
		for _, sid := range p.Params.ShortIDs {
			if !shortIDRe.MatchString(sid) {
				ve.add("shortIds", "invalid")
			}
		}
		if k, err := base64.RawURLEncoding.DecodeString(p.Params.PrivateKey); err != nil || len(k) != 32 {
			ve.add("privateKey", "invalid")
		}
		if p.Kind == xray.KindXHTTP {
			if !pathRe.MatchString(p.Params.Path) {
				ve.add("path", "invalid")
			}
			if !slices.Contains(xray.XHTTPModes, p.Params.Mode) {
				ve.add("mode", "invalid")
			}
		}
	case xray.KindHysteria2:
		if p.Params.ObfsPassword != "" && len(p.Params.ObfsPassword) < 8 {
			ve.add("obfsPassword", "too_short")
		}
	}
	if len(ve.Fields) > 0 {
		return ve
	}
	return nil
}

// Delete removes the profile and reloads xray.
func (s *Profiles) Delete(ctx context.Context, id int64) error {
	if err := s.Store.DeleteProfile(ctx, id); err != nil {
		return err
	}
	return s.Xray.Apply(ctx)
}
