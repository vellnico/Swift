//go:build integration

package service

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// TestRealXray covers the phase 3 acceptance criteria with the real binary:
// a seeded VLESS Reality profile comes up, a killed xray is back within 2 s,
// and users are added through the API without a restart.
func TestRealXray(t *testing.T) {
	bin := xray.TestBinary()
	if bin == "" {
		t.Fatal("xray binary not found: run `make xray`")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	apiPort := freePort(t)
	if err := st.SetSetting(ctx, "xray_api_port", strconv.Itoa(apiPort)); err != nil {
		t.Fatal(err)
	}
	vlessPort := freePort(t)
	priv, _, _ := xray.RealityKeyPair()
	if _, err := st.CreateProfile(ctx, xray.Profile{Name: "Vision", Kind: xray.KindVision, Port: vlessPort, Enabled: true, Params: xray.Params{
		PrivateKey: priv, ShortIDs: xray.ShortIDs(3), Target: "www.example.com:443", ServerNames: []string{"www.example.com"},
	}}); err != nil {
		t.Fatal(err)
	}
	u1, _ := st.CreateUser(ctx, store.User{Name: "First", SubToken: "a", VlessUUID: "6f1b0b8e-6d1f-4a7e-9a52-1b2f3c4d5e6f", Hy2Password: "x", Enabled: true})

	client, err := xray.Dial("127.0.0.1:" + strconv.Itoa(apiPort))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	cfgPath := filepath.Join(dir, "xray", "config.json")
	sup := &xray.Supervisor{Runner: xray.ExecRunner{}, Bin: bin, ConfigPath: cfgPath, Log: discard()}
	x := &Xray{Store: st, Runner: xray.ExecRunner{}, Sup: sup, API: client, Bin: bin, ConfigPath: cfgPath, Log: discard()}
	if err := x.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	go sup.Run(ctx)
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	if err := x.WaitAPI(wctx); err != nil {
		t.Fatal(err)
	}
	wcancel()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(vlessPort), 2*time.Second)
	if err != nil {
		t.Fatalf("VLESS Reality inbound is not listening: %v", err)
	}
	conn.Close()

	emails, err := client.Users(ctx, "p1")
	if err != nil || !slices.Equal(emails, []string{"u1"}) {
		t.Fatalf("users from DB seed: %v %v", emails, err)
	}

	// hot add without restart
	pid := sup.Status().PID
	u2, _ := st.CreateUser(ctx, store.User{Name: "Second", SubToken: "b", VlessUUID: "0a9c8b7d-5e4f-4321-8abc-def012345678", Hy2Password: "y", Enabled: true})
	if err := x.AddUser(ctx, u2); err != nil {
		t.Fatal(err)
	}
	emails, _ = client.Users(ctx, "p1")
	slices.Sort(emails)
	if !slices.Equal(emails, []string{"u1", "u2"}) {
		t.Fatalf("after hot add: %v", emails)
	}
	if err := st.DeleteUser(ctx, u1.ID); err != nil {
		t.Fatal(err)
	}
	if err := x.RemoveUser(ctx, u1); err != nil {
		t.Fatal(err)
	}
	emails, _ = client.Users(ctx, "p1")
	if !slices.Equal(emails, []string{"u2"}) {
		t.Fatalf("after hot remove: %v", emails)
	}
	if sup.Status().PID != pid || sup.Status().Restarts != 0 {
		t.Fatal("user changes restarted xray")
	}

	// kill -9 and measure recovery
	p, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	killed := time.Now()
	if err := p.Kill(); err != nil {
		t.Fatal(err)
	}
	for {
		st := sup.Status()
		if st.Status == xray.StatusRunning && st.PID != pid {
			break
		}
		if time.Since(killed) > 5*time.Second {
			t.Fatal("xray was not restarted")
		}
		time.Sleep(20 * time.Millisecond)
	}
	restarted := time.Since(killed)
	for {
		c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(vlessPort), 200*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		if time.Since(killed) > 5*time.Second {
			t.Fatal("inbound did not come back")
		}
		time.Sleep(10 * time.Millisecond)
	}
	listening := time.Since(killed)
	wctx, wcancel = context.WithTimeout(ctx, 5*time.Second)
	defer wcancel()
	if err := x.WaitAPI(wctx); err != nil {
		t.Fatal(err)
	}
	t.Logf("after kill: process restarted in %v, inbound listening in %v, api ready in %v", restarted, listening, time.Since(killed))
	if restarted > 2*time.Second {
		t.Fatalf("restart after kill took %v, want ≤ 2s", restarted)
	}
	// the restarted process still has the users from the regenerated config
	emails, _ = client.Users(ctx, "p1")
	if !slices.Equal(emails, []string{"u2"}) {
		t.Fatalf("users after restart: %v", emails)
	}
}
