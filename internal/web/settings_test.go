package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vellnico/swift/internal/service"
	"github.com/vellnico/swift/internal/store"
)

func loggedIn(t *testing.T, e *env) *client {
	t.Helper()
	c := newClient(t, e)
	c.do("GET", "/api/auth/session", "", false)
	creds := `{"username":"` + e.user + `","password":"` + e.pass + `"}`
	if code, body := c.do("POST", "/api/auth/login", creds, true); code != 200 {
		t.Fatalf("login: %d %v", code, body)
	}
	return c
}

func TestPanelSettingsMovePath(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_ = e.st.SetSetting(ctx, "panel_port", "24000")
	_ = e.st.SetSetting(ctx, "sub_port", "24001")
	var moved []int
	e.srv.Panel = &service.Panel{Store: e.st, Ports: &service.Ports{Store: e.st}, Move: func(port int, path string) {
		moved = append(moved, port)
		e.srv.SetPanelPath(path)
	}}
	e.srv.PanelPort = func() int { return 24000 }
	e.srv.registerSettings()
	c := loggedIn(t, e)

	if code, body := c.do("GET", "/api/settings/panel", "", false); code != 200 || body["path"] != "abc123" || body["port"] != 24000.0 || body["subPort"] != 24001.0 {
		t.Fatalf("get: %d %v", code, body)
	}
	code, body := c.do("PUT", "/api/settings/panel", `{"port":24001,"path":"ab"}`, true)
	fields, _ := body["fields"].(map[string]any)
	if code != 422 || fields["port"] != "busy" || fields["path"] != "invalid" {
		t.Fatalf("validation: %d %v", code, body)
	}
	if code, body := c.do("PUT", "/api/settings/panel", `{"port":80,"path":"newpath99"}`, true); code != 422 {
		t.Fatalf("privileged port accepted: %d %v", code, body)
	}
	if code, body := c.do("PUT", "/api/settings/panel", `{"port":24000,"path":"newpath99"}`, true); code != 200 || body["path"] != "newpath99" {
		t.Fatalf("move: %d %v", code, body)
	}
	if len(moved) != 1 {
		t.Fatalf("Move calls: %v", moved)
	}
	if v, _ := e.st.Setting(ctx, "panel_path"); v != "newpath99" {
		t.Fatalf("stored path %q", v)
	}
	h := e.srv.Handler()
	if rec := get(t, h, "/abc123/"); rec.Code != http.StatusNotFound {
		t.Fatalf("old path still served: %d", rec.Code)
	}
	if rec := get(t, h, "/newpath99/"); rec.Code != http.StatusOK {
		t.Fatalf("new path: %d", rec.Code)
	}
}

func TestPanelPinnedAndAddress(t *testing.T) {
	e := newEnv(t)
	e.srv.Panel = &service.Panel{Store: e.st, Ports: &service.Ports{Store: e.st}, Pinned: true}
	e.srv.registerSettings()
	c := loggedIn(t, e)
	if code, body := c.do("PUT", "/api/settings/panel", `{"port":25000,"path":"newpath99"}`, true); code != 422 || body["fields"].(map[string]any)["port"] != "pinned" {
		t.Fatalf("pinned: %d %v", code, body)
	}
	for _, bad := range []string{"", "127.0.0.1", "example.com", "0.0.0.0"} {
		if code, _ := c.do("PUT", "/api/settings/address", `{"address":"`+bad+`"}`, true); code != 422 {
			t.Fatalf("address %q accepted", bad)
		}
	}
	if code, body := c.do("PUT", "/api/settings/address", `{"address":"203.0.113.9"}`, true); code != 200 || body["address"] != "203.0.113.9" {
		t.Fatalf("address: %d %v", code, body)
	}
}

func TestJournalFilterAndPaging(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = e.st.AddEvent(ctx, "info", "test", "test.info", nil)
	}
	_, _ = e.st.AddEvent(ctx, "error", "xray", "xray.crashed", map[string]any{"code": 1})
	c := loggedIn(t, e)
	list := func(q string) []store.Event {
		t.Helper()
		resp, err := c.hc.Get(c.base + "/api/journal" + q)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var evs []store.Event
		if err := json.NewDecoder(resp.Body).Decode(&evs); err != nil {
			t.Fatalf("%s: %d %v", q, resp.StatusCode, err)
		}
		return evs
	}
	if evs := list("?level=error"); len(evs) != 1 || evs[0].Key != "xray.crashed" {
		t.Fatalf("level filter: %+v", evs)
	}
	first := list("?limit=3")
	if len(first) != 3 || first[0].Key != "auth.login" || first[1].Key != "xray.crashed" {
		t.Fatalf("first page: %+v", first)
	}
	// the login is the newest entry; four test entries remain after the first page
	if rest := list("?limit=10&before=" + jsonID(first[2].ID)); len(rest) != 4 || rest[0].ID != first[2].ID-1 {
		t.Fatalf("second page: %d", len(rest))
	}
	if resp, _ := c.hc.Get(c.base + "/api/journal?level=debug"); resp.StatusCode != 400 {
		t.Fatalf("bad level: %d", resp.StatusCode)
	}
}

func jsonID(id int64) string { b, _ := json.Marshal(id); return string(b) }
