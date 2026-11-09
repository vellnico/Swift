package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/vellnico/swift/internal/auth"
	"github.com/vellnico/swift/internal/config"
	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
)

func admin(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: swift admin show|reset")
	}
	fs := flag.NewFlagSet("admin", flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath, "config file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	ctx := context.Background()
	cfg, st, err := openStore(ctx, *cfgPath)
	if err != nil {
		return err
	}
	defer st.Close()

	switch args[0] {
	case "show":
		return adminShow(ctx, cfg, st, stdout)
	case "reset":
		return adminReset(ctx, cfg, st, stdout)
	}
	return fmt.Errorf("unknown admin command %q", args[0])
}

func panelURL(ctx context.Context, cfg config.Config, st *store.Store) (string, error) {
	port, path, err := panelAddress(ctx, cfg, st)
	if err != nil {
		return "", err
	}
	host, err := st.Setting(ctx, "domain")
	if err != nil || host == "" {
		host, err = st.Setting(ctx, "server_address")
	}
	if err != nil || host == "" {
		host = "<server-ip>"
	}
	return fmt.Sprintf("%s://%s/%s/", service.Scheme(cfg.InsecureHTTP), net.JoinHostPort(host, strconv.Itoa(port)), path), nil
}

func adminShow(ctx context.Context, cfg config.Config, st *store.Store, w io.Writer) error {
	url, err := panelURL(ctx, cfg, st)
	if err != nil {
		return err
	}
	a, err := st.FirstAdmin(ctx)
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintf(w, "URL:   %s\nLogin: not created yet, run: swift admin reset\n", url)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "URL:   %s\nLogin: %s\n", url, a.Username)
	return nil
}

func adminReset(ctx context.Context, cfg config.Config, st *store.Store, w io.Writer) error {
	url, err := panelURL(ctx, cfg, st)
	if err != nil {
		return err
	}
	user, pass, err := auth.NewManager(st).ResetAdmin(ctx)
	if err != nil {
		return fmt.Errorf("reset admin: %w", err)
	}
	// The password goes to the terminal only, never to logs.
	fmt.Fprintf(w, "\n  URL:      %s\n  Login:    %s\n  Password: %s\n\n  Save the password: it is shown only once.\n\n", url, user, pass)
	return nil
}
