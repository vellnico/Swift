package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/web"
	"github.com/vellnico/swift/internal/xray"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "swift:", err)
		os.Exit(1)
	}
}

const usage = `Usage:
  swift serve [-config path]
  swift admin show|reset [-config path]
  swift setup [-address ip] [-domain name] [-lang en|ru] [-config path]
  swift db backup <file> [-config path]
  swift ports [-config path]
  swift version [--xray]
`

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("no command")
	}
	switch args[0] {
	case "version":
		if len(args) > 1 && args[1] == "--xray" {
			fmt.Fprintln(stdout, xray.Version)
			return nil
		}
		fmt.Fprintln(stdout, "swift", version, "xray", xray.Version)
		return nil
	case "setup":
		return setup(args[1:], stdout)
	case "db":
		return dbCommand(args[1:], stdout)
	case "ports":
		return ports(args[1:], stdout)
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		cfgPath := fs.String("config", config.DefaultPath, "config file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return serve(*cfgPath, stdout)
	case "admin":
		return admin(args[1:], stdout)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	}
	fmt.Fprint(stderr, usage)
	return fmt.Errorf("unknown command %q", args[0])
}

func openStore(ctx context.Context, cfgPath string) (config.Config, *store.Store, error) {
	cfg, err := config.Load(cfgPath, os.Getenv)
	if err != nil {
		return cfg, nil, err
	}
	st, err := store.Open(ctx, cfg.DBPath())
	if err != nil {
		return cfg, nil, fmt.Errorf("open database: %w", err)
	}
	return cfg, st, nil
}

func serve(cfgPath string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, st, err := openStore(ctx, cfgPath)
	if err != nil {
		return err
	}
	defer st.Close()

	port, path, err := panelAddress(ctx, cfg, st)
	if err != nil {
		return err
	}
	subPort, err := service.EnsureSubPort(ctx, st, cfg.SubPort, port, func() int { return 20000 + randInt(40001) })
	if err != nil {
		return err
	}
	pinned := cfg.PanelPort != 0 || cfg.PanelPath != "" || subPort == port
	a, err := newApp(ctx, cfg, st, log, port, path, pinned)
	if err != nil {
		return err
	}
	if _, err := st.FirstAdmin(ctx); errors.Is(err, store.ErrNotFound) {
		fmt.Fprintln(stdout, "No administrator yet: run `swift admin reset` to create one.")
	}
	go sessionGC(ctx, st, log)
	if err := a.startCerts(ctx); err != nil {
		return err
	}
	a.startXray(ctx)
	a.startStats(ctx)
	a.startGeoUpdates(ctx)
	a.swift.Restart = stop // systemd (Restart=always) starts the new binary
	a.swift.ReportStartup(ctx)
	go a.swift.RunChecks(ctx)
	go a.health.Run(ctx)

	panelH, subH := a.handlers(port, subPort)
	errc := make(chan error, 4)
	start := func(p int, h http.Handler) (*http.Server, error) {
		hs := newHTTPServer(cfg.Listen, p, h)
		ln, err := net.Listen("tcp", hs.Addr)
		if err != nil {
			return nil, fmt.Errorf("listen %s: %w", hs.Addr, err)
		}
		go func() {
			var err error
			if cfg.InsecureHTTP {
				err = hs.Serve(ln)
			} else {
				// certificates come from GetCertificate and can change while serving
				hs.TLSConfig = a.holder.TLSConfig()
				hs.ErrorLog = stdlog.New(io.Discard, "", 0) // handshake noise from scanners
				err = hs.ServeTLS(web.TLSOnly(ln), "", "")
			}
			if !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
		return hs, nil
	}
	panel, err := start(port, panelH)
	if err != nil {
		return err
	}
	servers := []*http.Server{panel}
	if subH != nil {
		sub, err := start(subPort, subH)
		if err != nil {
			return err
		}
		servers = append(servers, sub)
	}
	host := cfg.Listen
	if host == "" {
		host = "localhost"
	}
	fmt.Fprintf(stdout, "Swift panel: %s://%s/%s/\n", service.Scheme(cfg.InsecureHTTP), net.JoinHostPort(host, strconv.Itoa(port)), path)
	fmt.Fprintf(stdout, "Subscriptions: port %d\n", subPort)
	if cfg.InsecureHTTP {
		log.Warn("serving without TLS (insecure_http); use only for local development")
	}

	shutdown := func(hs *http.Server) {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if hs.Shutdown(sctx) != nil {
			_ = hs.Close() // SSE streams never go idle
		}
	}
	for {
		select {
		case err := <-errc:
			return fmt.Errorf("http server: %w", err)
		case p := <-a.move:
			next, err := start(p, panelH)
			if err != nil {
				log.Error("move panel", "err", err)
				a.event("error", "panel", "panel.move_failed", map[string]any{"port": p})
				continue
			}
			shutdown(servers[0])
			servers[0] = next
			a.panelPort.Store(int64(p))
			log.Info("panel moved", "port", p)
		case <-ctx.Done():
			for _, hs := range servers {
				shutdown(hs)
			}
			return nil
		}
	}
}

func newHTTPServer(host string, port int, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              net.JoinHostPort(host, strconv.Itoa(port)),
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// panelAddress resolves the panel port and path: config/env first, then the
// database, otherwise generates and stores new random values.
func panelAddress(ctx context.Context, cfg config.Config, st *store.Store) (int, string, error) {
	port := cfg.PanelPort
	if port == 0 {
		v, err := st.Setting(ctx, "panel_port")
		switch {
		case err == nil:
			port, _ = strconv.Atoi(v)
		case errors.Is(err, store.ErrNotFound):
			port = 20000 + randInt(40001)
			if err := st.SetSetting(ctx, "panel_port", strconv.Itoa(port)); err != nil {
				return 0, "", err
			}
		default:
			return 0, "", err
		}
	}
	path := cfg.PanelPath
	if path == "" {
		v, err := st.Setting(ctx, "panel_path")
		switch {
		case err == nil:
			path = v
		case errors.Is(err, store.ErrNotFound):
			path = randomPath(16)
			if err := st.SetSetting(ctx, "panel_path", path); err != nil {
				return 0, "", err
			}
		default:
			return 0, "", err
		}
	}
	return port, path, nil
}

func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

func randomPath(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[randInt(len(alphabet))]
	}
	return string(b)
}

func sessionGC(ctx context.Context, st *store.Store, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := st.DeleteExpiredSessions(ctx); err != nil && ctx.Err() == nil {
			log.Error("session cleanup", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
