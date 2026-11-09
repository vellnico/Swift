package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/bus"
	certsPkg "github.com/vellnico/swift/internal/certs"
	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/routing"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/sysstat"
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
	stats    *sysstat.Collector
	holder   *certsPkg.Holder
	updater  *service.XrayUpdater
	panel    *service.Panel

	xrayVersion atomic.Pointer[string]
	panelPort   atomic.Int64
	move        chan int // new panel port
}

func (a *app) version() string {
	if v := a.xrayVersion.Load(); v != nil {
		return *v
	}
	return xray.Version
}

func (a *app) setVersion(v string) {
	a.xrayVersion.Store(&v)
	a.bus.PublishSticky("xray_status", xrayStatus{a.sup.Status(), v})
}

// startCerts loads (or creates) the certificate for TLS listeners and keeps it renewed.
func (a *app) startCerts(ctx context.Context) error {
	if _, err := a.certs.Ensure(ctx); err != nil {
		return fmt.Errorf("certificate: %w", err)
	}
	go a.certs.RunRenewal(ctx, 24*time.Hour)
	return nil
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

func newApp(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger, panelPort int, panelPath string, pinned bool) (*app, error) {
	a := &app{cfg: cfg, st: st, log: log, bus: bus.New(), move: make(chan int, 1)}

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
		OnStatus:   func(i xray.StatusInfo) { a.bus.PublishSticky("xray_status", xrayStatus{i, a.version()}) },
		OnEvent:    func(level, key string, p map[string]any) { a.event(level, "xray", key, p) },
	}
	a.bus.PublishSticky("xray_status", xrayStatus{a.sup.Status(), a.version()})
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

	certDir := filepath.Join(cfg.DataDir, "certs")
	a.holder = &certsPkg.Holder{}
	certs := &service.Certs{
		Store:  st,
		Dir:    certDir,
		Holder: a.holder,
		Issuer: &certsPkg.LegoIssuer{Dir: certDir, Email: cfg.ACMEEmail, Staging: cfg.ACMEStaging},
		Apply:  a.xray.Reload,
		Event:  a.event,
	}
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
	a.subs = &service.Subscriptions{Store: st, Certs: certs, Scheme: func(context.Context) string { return service.Scheme(cfg.InsecureHTTP) }}
	a.subs.Routing = service.RoutingFor(st)
	a.sub = &web.SubServer{Store: st, Subs: a.subs, Assets: webassets.SubPage(), Dict: webassets.Dictionary, Log: log,
		HappRouting: func(r *http.Request) string {
			if p, err := service.Preset(r.Context(), st); err == nil && p == routing.PresetRUDirect {
				return routing.RUDirect{}.HappHeader()
			}
			return ""
		},
	}

	asset, _ := xray.AssetName(runtime.GOOS, runtime.GOARCH)
	a.updater = &service.XrayUpdater{
		Store: st, Xray: a.xray, Source: xray.GitHubReleases{}, Asset: asset, Binary: xray.BinaryName(runtime.GOOS),
		Probe: xray.BinaryVersion, Event: a.event, OnVersion: a.setVersion,
		Progress: func(stage, detail string) {
			a.bus.Publish("xray_update", map[string]string{"stage": stage, "detail": detail})
		},
	}
	a.panelPort.Store(int64(panelPort))
	a.panel = &service.Panel{
		Store: st, Ports: &service.Ports{Store: st, Checker: service.OSPorts{}}, Pinned: pinned, Event: a.event,
		Move: func(port int, path string) {
			// let the reply reach the browser before the old address goes away
			time.AfterFunc(500*time.Millisecond, func() {
				a.srv.SetPanelPath(path)
				if int64(port) != a.panelPort.Load() {
					a.move <- port
				}
			})
		},
	}

	a.srv = web.New(web.Deps{
		Panel:     a.panel,
		Updater:   a.updater,
		PanelPort: func() int { return int(a.panelPort.Load()) },
		PanelPath: panelPath,
		Assets:    webassets.Panel(),
		Log:       log,
		Store:     st,
		Auth:      auth.NewManager(st),
		Bus:       a.bus,
		Profiles:  a.profiles,
		Users:     a.users,
		Subs:      a.subs,
		Certs:     certs,
		OnEvent:   func(ev store.Event) { a.bus.Publish("event", ev) },
	})
	a.srv.Handle("GET /api/xray/status", func(w http.ResponseWriter, _ *http.Request) {
		web.WriteJSON(w, http.StatusOK, xrayStatus{a.sup.Status(), a.version()})
	})

	switch {
	case runtime.GOOS == "linux":
		a.stats = &sysstat.Collector{Source: sysstat.ProcSource{Proc: os.DirFS("/proc"), DiskPath: cfg.DataDir}}
	case cfg.Dev:
		a.stats = &sysstat.Collector{Source: &sysstat.Fake{}}
	}
	a.srv.Handle("GET /api/sysstat", func(w http.ResponseWriter, _ *http.Request) {
		if a.stats == nil {
			web.WriteJSON(w, http.StatusOK, []sysstat.Sample{})
			return
		}
		web.WriteJSON(w, http.StatusOK, a.stats.History())
	})
	a.srv.Handle("GET /api/overview", a.overview)
	return a, nil
}

// startGeoUpdates keeps the server's geo files fresh (production only: the files are ~100 MB).
func (a *app) startGeoUpdates(ctx context.Context) {
	if a.cfg.Dev {
		return
	}
	g := &routing.GeoUpdater{
		Dir:   filepath.Dir(a.cfg.XrayPath()),
		Event: func(level, key string, p map[string]any) { a.event(level, "geo", key, p) },
	}
	go g.Run(ctx, 24*time.Hour)
}

func (a *app) startStats(ctx context.Context) {
	if a.stats == nil {
		return
	}
	a.stats.OnSample = func(s sysstat.Sample) { a.bus.Publish("sysstat", s) }
	go a.stats.Run(ctx)
}

// overview feeds the dashboard cards.
func (a *app) overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ps, err := a.st.Profiles(ctx)
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	us, err := a.st.Users(ctx)
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := map[string]any{"profiles": len(ps), "users": len(us)}
	enabled := 0
	for _, p := range ps {
		if p.Enabled {
			enabled++
		}
	}
	out["profilesEnabled"] = enabled
	enabled = 0
	for _, u := range us {
		if u.Enabled {
			enabled++
		}
	}
	out["usersEnabled"] = enabled
	if c, err := a.st.Certificate(ctx); err == nil {
		out["cert"] = c
	}
	if info := a.updater.Info(ctx); info.Available {
		out["xrayUpdate"] = info.Latest
	}
	web.WriteJSON(w, http.StatusOK, out)
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
	a.setVersion(a.updater.Current(ctx))
	go a.sup.Run(ctx)
	go a.updater.RunChecks(ctx, 24*time.Hour)
}
