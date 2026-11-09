package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"

	"github.com/vellnico/swift/internal/certs"
	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/xray"
)

// setup prepares a fresh server for install.sh: address, ports, certificate,
// default profiles (VLESS Reality Vision 443/tcp and Hysteria2 443/udp) and a
// tested xray config. Running it again changes nothing.
func setup(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath, "config file")
	address := fs.String("address", "", "public IP address of the server")
	domain := fs.String("domain", "", "domain pointing at the server (Let's Encrypt)")
	noProfiles := fs.Bool("no-profiles", false, "do not create the default profiles")
	lang := fs.String("lang", "", "default panel language: en or ru")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *lang != "" && !service.ValidLanguage(*lang) {
		return fmt.Errorf("invalid -lang %q: use en or ru", *lang)
	}
	ctx := context.Background()
	cfg, st, err := openStore(ctx, *cfgPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// applied on every run, so running the installer again can switch the language
	if *lang != "" {
		if err := st.SetSetting(ctx, "language", *lang); err != nil {
			return err
		}
	}
	if done, _ := st.Setting(ctx, "setup_done"); done == "1" {
		fmt.Fprintln(stdout, "Swift is already set up; nothing to do.")
		return nil
	}
	if *address != "" {
		if net.ParseIP(*address) == nil {
			return fmt.Errorf("invalid --address %q", *address)
		}
		if err := st.SetSetting(ctx, "server_address", *address); err != nil {
			return err
		}
	}
	panelPort, _, err := panelAddress(ctx, cfg, st)
	if err != nil {
		return err
	}
	if _, err := service.EnsureSubPort(ctx, st, cfg.SubPort, panelPort, func() int { return 20000 + randInt(40001) }); err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	certDir := filepath.Join(cfg.DataDir, "certs")
	x := &service.Xray{Store: st, Runner: xray.ExecRunner{}, Bin: cfg.XrayPath(), ConfigPath: filepath.Join(cfg.DataDir, "xray", "config.json"), Log: log}
	c := &service.Certs{Store: st, Dir: certDir, Issuer: &certs.LegoIssuer{Dir: certDir, Email: cfg.ACMEEmail, Staging: cfg.ACMEStaging}}
	x.Certs = c.Files

	if *domain != "" {
		fmt.Fprintf(stdout, "Requesting a Let's Encrypt certificate for %s...\n", *domain)
		if _, err := c.IssueACME(ctx, *domain); err != nil {
			fmt.Fprintf(stdout, "Could not get a certificate (%v); using a self-signed one. Retry later in Settings.\n", err)
		}
	}
	if _, err := c.Ensure(ctx); err != nil {
		return fmt.Errorf("certificate: %w", err)
	}

	if !*noProfiles {
		ps := &service.Profiles{Store: st, Xray: x, Ports: &service.Ports{Store: st, Checker: service.OSPorts{}}, Prober: xray.TLSProber{}, Certs: c}
		existing, err := st.Profiles(ctx)
		if err != nil {
			return err
		}
		if len(existing) == 0 {
			for _, k := range []xray.Kind{xray.KindVision, xray.KindHysteria2} {
				p, err := ps.CreateOffline(ctx, k)
				if err != nil {
					return fmt.Errorf("default profile %s: %w", k, err)
				}
				fmt.Fprintf(stdout, "Profile %s on %s port %d\n", p.Name, k.Network(), p.Port)
			}
		}
	}
	if err := x.Prepare(ctx); err != nil {
		return fmt.Errorf("xray config: %w", err)
	}
	return st.SetSetting(ctx, "setup_done", "1")
}

// ports prints "<proto> <port>" lines the firewall must allow (used by install.sh).
func ports(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("ports", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath, "config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	_, st, err := openStore(ctx, *cfgPath)
	if err != nil {
		return err
	}
	defer st.Close()
	seen := map[string]bool{}
	emit := func(proto, port string) {
		if port == "" || seen[proto+port] {
			return
		}
		seen[proto+port] = true
		fmt.Fprintln(stdout, proto, port)
	}
	for _, key := range []string{"panel_port", "sub_port"} {
		v, _ := st.Setting(ctx, key)
		emit("tcp", v)
	}
	if d, _ := st.Setting(ctx, "domain"); d != "" {
		emit("tcp", "80") // ACME HTTP-01 renewals
	}
	ps, err := st.Profiles(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		emit(p.Kind.Network(), fmt.Sprint(p.Port))
	}
	return nil
}

// dbCommand implements `swift db backup <file>`.
func dbCommand(args []string, stdout io.Writer) error {
	if len(args) < 2 || args[0] != "backup" {
		return errors.New("usage: swift db backup <file> [-config path]")
	}
	fs := flag.NewFlagSet("db", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath, "config file")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	ctx := context.Background()
	_, st, err := openStore(ctx, *cfgPath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Backup(ctx, args[1]); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Backup written to", args[1])
	return nil
}
