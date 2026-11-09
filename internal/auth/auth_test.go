package auth

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vellnico/swift/internal/store"
)

var cheap = Params{Memory: 1024, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse", DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected PHC string %q", h)
	}
	if ok, err := VerifyPassword("correct horse", h); !ok || err != nil {
		t.Fatalf("verify: %v %v", ok, err)
	}
	if ok, _ := VerifyPassword("wrong", h); ok {
		t.Fatal("wrong password accepted")
	}
	if _, err := VerifyPassword("x", "$bcrypt$foo"); !errors.Is(err, ErrBadHash) {
		t.Fatalf("want ErrBadHash, got %v", err)
	}
}

func TestGeneratedCredentials(t *testing.T) {
	if u := NewUsername(); !regexp.MustCompile(`^[a-z][a-z0-9]{7}$`).MatchString(u) {
		t.Fatalf("username %q", u)
	}
	if p := NewPassword(); len(p) != 20 {
		t.Fatalf("password length %d", len(p))
	}
	if a, b := NewPassword(), NewPassword(); a == b {
		t.Fatal("passwords repeat")
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

func TestLimiterBlocksAfterFiveFailures(t *testing.T) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	l := NewLimiter()
	l.Now = c.now
	for i := 0; i < 4; i++ {
		if l.Fail("1.2.3.4") {
			t.Fatalf("blocked too early at %d", i)
		}
		c.add(time.Minute)
	}
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("blocked before the fifth failure")
	}
	if !l.Fail("1.2.3.4") {
		t.Fatal("fifth failure must block")
	}
	ok, wait := l.Allow("1.2.3.4")
	if ok || wait != 15*time.Minute {
		t.Fatalf("allow=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Fatal("other IPs must not be blocked")
	}
	c.add(15 * time.Minute)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("block must expire after 15 minutes")
	}
}

func TestLimiterWindowSlides(t *testing.T) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	l := NewLimiter()
	l.Now = c.now
	for i := 0; i < 10; i++ {
		if l.Fail("ip") {
			t.Fatalf("failures spread over >10 min must not block (i=%d)", i)
		}
		c.add(3 * time.Minute)
	}
	l.Fail("ip")
	l.Success("ip")
	if ok, _ := l.Allow("ip"); !ok {
		t.Fatal("success must reset")
	}
}

func newManager(t *testing.T) (*Manager, *clock) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "swift.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	m := NewManager(st)
	m.Params, m.Now = cheap, c.now
	return m, c
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	m, c := newManager(t)
	user, pass, err := m.ResetAdmin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Login(ctx, user, "nope", "ip", "ua"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := m.Login(ctx, "ghost", pass, "ip", "ua"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user: %v", err)
	}
	ses, err := m.Login(ctx, user, pass, "ip", "ua")
	if err != nil {
		t.Fatal(err)
	}
	if len(ses.ID) != 43 {
		t.Fatalf("session id should be 32 bytes base64url, got %q", ses.ID)
	}

	c.add(13 * 24 * time.Hour)
	got, admin, err := m.Validate(ctx, ses.ID)
	if err != nil || admin.Username != user {
		t.Fatalf("validate: %v %v", admin, err)
	}
	if !got.ExpiresAt.Equal(c.t.Add(14 * 24 * time.Hour)) {
		t.Fatalf("session not extended: %v", got.ExpiresAt)
	}

	c.add(14*24*time.Hour + time.Second)
	if _, _, err := m.Validate(ctx, ses.ID); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expired session accepted: %v", err)
	}

	ses2, _ := m.Login(ctx, user, pass, "ip", "ua")
	if err := m.Logout(ctx, ses2.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Validate(ctx, ses2.ID); !errors.Is(err, ErrNoSession) {
		t.Fatal("logged out session still valid")
	}
}

func TestResetDropsSessions(t *testing.T) {
	ctx := context.Background()
	m, _ := newManager(t)
	user, pass, _ := m.ResetAdmin(ctx)
	ses, err := m.Login(ctx, user, pass, "", "")
	if err != nil {
		t.Fatal(err)
	}
	user2, pass2, err := m.ResetAdmin(ctx)
	if err != nil || user2 != user || pass2 == pass {
		t.Fatalf("reset: %q %v", user2, err)
	}
	if _, _, err := m.Validate(ctx, ses.ID); !errors.Is(err, ErrNoSession) {
		t.Fatal("old session survived password reset")
	}
	if err := m.ChangePassword(ctx, 1, "bad", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("change with wrong current: %v", err)
	}
	if err := m.ChangePassword(ctx, 1, pass2, "new-password-123"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Login(ctx, user, "new-password-123", "", ""); err != nil {
		t.Fatal(err)
	}
}
