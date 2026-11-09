package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vellnico/swift/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNoSession          = errors.New("no valid session")
)

// Manager issues and validates admin sessions stored in the database.
type Manager struct {
	Store  *store.Store
	TTL    time.Duration
	Params Params
	Now    func() time.Time

	dummyOnce sync.Once
	dummyHash string
}

func NewManager(st *store.Store) *Manager {
	return &Manager{Store: st, TTL: 14 * 24 * time.Hour, Params: DefaultParams, Now: time.Now}
}

// Login checks the credentials and creates a session. Unknown users cost the
// same argon2 work as known ones so timing does not reveal valid logins.
func (m *Manager) Login(ctx context.Context, username, password, ip, ua string) (store.Session, error) {
	admin, err := m.Store.AdminByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		m.dummyOnce.Do(func() { m.dummyHash, _ = HashPassword("swift-dummy", m.Params) })
		_, _ = VerifyPassword(password, m.dummyHash)
		return store.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return store.Session{}, err
	}
	ok, err := VerifyPassword(password, admin.PasswordHash)
	if err != nil {
		return store.Session{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return store.Session{}, ErrInvalidCredentials
	}
	if len(ua) > 256 {
		ua = ua[:256]
	}
	ses := store.Session{ID: RandomToken(32), AdminID: admin.ID, ExpiresAt: m.Now().Add(m.TTL), IP: ip, UserAgent: ua}
	if err := m.Store.CreateSession(ctx, ses); err != nil {
		return store.Session{}, err
	}
	return ses, nil
}

// Validate returns the session if it exists and has not expired, extending it
// when less than TTL-1h remains (sliding expiration without a write per request).
func (m *Manager) Validate(ctx context.Context, id string) (store.Session, store.Admin, error) {
	if id == "" {
		return store.Session{}, store.Admin{}, ErrNoSession
	}
	ses, err := m.Store.SessionByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return ses, store.Admin{}, ErrNoSession
	}
	if err != nil {
		return ses, store.Admin{}, err
	}
	now := m.Now()
	if !now.Before(ses.ExpiresAt) {
		_ = m.Store.DeleteSession(ctx, id)
		return ses, store.Admin{}, ErrNoSession
	}
	if ses.ExpiresAt.Sub(now) < m.TTL-time.Hour {
		ses.ExpiresAt = now.Add(m.TTL)
		if err := m.Store.ExtendSession(ctx, id, ses.ExpiresAt); err != nil {
			return ses, store.Admin{}, err
		}
	}
	admin, err := m.Store.AdminByID(ctx, ses.AdminID)
	if err != nil {
		return ses, admin, ErrNoSession
	}
	return ses, admin, nil
}

func (m *Manager) Logout(ctx context.Context, id string) error {
	return m.Store.DeleteSession(ctx, id)
}

// ResetAdmin sets a fresh password for the admin, creating one if none exists.
// It returns the username and the plain password, which the caller shows once.
func (m *Manager) ResetAdmin(ctx context.Context) (string, string, error) {
	password := NewPassword()
	hash, err := HashPassword(password, m.Params)
	if err != nil {
		return "", "", err
	}
	admin, err := m.Store.FirstAdmin(ctx)
	switch {
	case errors.Is(err, store.ErrNotFound):
		admin, err = m.Store.CreateAdmin(ctx, NewUsername(), hash)
		if err != nil {
			return "", "", err
		}
	case err != nil:
		return "", "", err
	default:
		if err := m.Store.SetAdminPassword(ctx, admin.ID, hash); err != nil {
			return "", "", err
		}
	}
	return admin.Username, password, nil
}

// ChangePassword verifies the current password and sets a new one.
func (m *Manager) ChangePassword(ctx context.Context, adminID int64, current, next string) error {
	admin, err := m.Store.AdminByID(ctx, adminID)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(current, admin.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(next, m.Params)
	if err != nil {
		return err
	}
	return m.Store.SetAdminPassword(ctx, adminID, hash)
}
