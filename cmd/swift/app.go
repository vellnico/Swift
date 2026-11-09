package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/bus"
	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/web"
	"github.com/vellnico/swift/internal/xray"
	webassets "github.com/vellnico/swift/web"
)

// app wires the long-lived parts of `swift serve` together.
type app struct {
	cfg      config.Config
	st       *store.Store
	log      *slog.Logger
	bus      *bus.Bus
	srv      *web.Server
	sup      *xray.Supervisor
	xray     *service.Xray
	certs    *service.Certs
	profiles *service.Profiles
	users    *service.Users
	subs     *service.Subscriptions
	sub      *web.SubServer
}

// handlers returns the panel and subscription handlers; with one shared port
// the subscription server answers /s/ and the panel everything else.
func (a *app) handlers(panelPort, subPort int) (panel, sub http.Handler) {
	if panelPort != subPort {
		return a.srv.Handler(), a.sub.Handler()
	}
	ph, sh := a.srv.Handler(), a.sub.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/s/") {
			sh.ServeHTTP(w, r)
			return
		}
		ph.ServeHTTP(w, r)
	}), nil
}

type xrayStatus struct {
	xray.StatusInfo
	Version string `json:"version"`
}

func newApp(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger, panelPath string) (*app, error) {
	a := &app{cfg: cfg, st: st, log: log, bus: bus.New()}

	apiPort, err := service.EnsureAPIPort(ctx, st)
	if err != nil {
		return nil, err
	}
	client, err := xray.Dial("127.0.0.1:" + strconv.Itoa(apiPort))
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(cfg.DataDir, "xray", "config.json")
	a.sup = &xray.Supervisor{
		Runner:     xray.ExecRunner{},
		Bin:        cfg.XrayPath(),
		ConfigPath: cfgPath,
		Log:        log,
		OnStatus:   func(i xray.StatusInfo) { a.bus.PublishSticky("xray_status", xrayStatus{i, xray.Version}) },
		OnEvent:    func(level, key string, p map[string]any) { a.event(level, "xray", key, p) },
	}
	a.bus.PublishSticky("xray_status", xrayStatus{a.sup.Status(), xray.Version})
	a.xray = &service.Xray{
		Store:      st,
		Runner:     xray.ExecRunner{},
		Sup:        a.sup,
		API:        client,
		Bin:        cfg.XrayPath(),
		ConfigPath: cfgPath,
		Log:        log,
		Event:      a.event,
		Progress: func(stage, detail string) {
			a.bus.Publish("apply", map[string]string{"stage": stage, "detail": detail})
		},
	}

	certs := &service.Certs{Store: st, Dir: filepath.Join(cfg.DataDir, "certs")}
	a.xray.Certs = certs.Files
	a.certs = certs
	a.profiles = &service.Profiles{
		Store:  st,
		Xray:   a.xray,
		Ports:  &service.Ports{Store: st, Checker: service.OSPorts{}},
		Prober: xray.TLSProber{},
		Certs:  certs,
	}

	a.users = &service.Users{Store: st, Xray: a.xray}
	a.subs = &service.Subscriptions{Store: st, Certs: certs}
	a.sub = &web.SubServer{Store: st, Subs: a.subs, Assets: webassets.SubPage(), Dict: webassets.Dictionary, Log: log}

	a.srv = web.New(web.Deps{
		PanelPath: panelPath,
		Assets:    webassets.Panel(),
		Log:       log,
		Store:     st,
		Auth:      auth.NewManager(st),
		Bus:       a.bus,
		Profiles:  a.profiles,
		Users:     a.users,
		Subs:      a.subs,
		OnEvent:   func(ev store.Event) { a.bus.Publish("event", ev) },
	})
	a.srv.Handle("GET /api/xray/status", func(w http.ResponseWriter, _ *http.Request) {
		web.WriteJSON(w, http.StatusOK, xrayStatus{a.sup.Status(), xray.Version})
	})
	return a, nil
}

// event writes a journal entry and pushes it to connected browsers.
func (a *app) event(level, source, key string, params map[string]any) {
	ev, err := a.st.AddEvent(context.Background(), level, source, key, params)
	if err != nil {
		a.log.Error("journal", "err", err)
		return
	}
	a.bus.Publish("event", ev)
}

// startXray installs the config from the database and keeps xray running.
func (a *app) startXray(ctx context.Context) {
	if _, err := os.Stat(a.sup.Bin); err != nil {
		a.log.Error("xray binary not found; panel runs without it", "path", a.sup.Bin)
		a.event("error", "xray", "xray.binary_missing", map[string]any{"path": a.sup.Bin})
		return
	}
	if err := a.xray.Prepare(ctx); err != nil {
		a.log.Error("xray config", "err", err)
		if _, statErr := os.Stat(a.sup.ConfigPath); statErr != nil {
			return
		}
	}
	go a.sup.Run(ctx)
}
