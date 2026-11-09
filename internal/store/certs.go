package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Certificate struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"` // acme | self_signed
	Domain    string    `json:"domain"`
	CertPath  string    `json:"-"`
	KeyPath   string    `json:"-"`
	SHA256Hex string    `json:"sha256"`
	NotAfter  time.Time `json:"notAfter"`
	LastError string    `json:"lastError"`
}

// Certificate returns the active certificate (the newest row).
func (s *Store) Certificate(ctx context.Context) (Certificate, error) {
	var c Certificate
	var domain, lastErr sql.NullString
	var notAfter int64
	err := s.DB.QueryRowContext(ctx, `SELECT id, kind, domain, cert_path, key_path, sha256_hex, not_after, last_error FROM certificates ORDER BY id DESC LIMIT 1`).
		Scan(&c.ID, &c.Kind, &domain, &c.CertPath, &c.KeyPath, &c.SHA256Hex, &notAfter, &lastErr)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("read certificate: %w", err)
	}
	c.Domain, c.LastError, c.NotAfter = domain.String, lastErr.String, time.Unix(notAfter, 0)
	return c, nil
}

// SaveCertificate makes c the only active certificate.
func (s *Store) SaveCertificate(ctx context.Context, c Certificate) (Certificate, error) {
	var domain any
	if c.Domain != "" {
		domain = c.Domain
	}
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM certificates`); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO certificates(kind, domain, cert_path, key_path, sha256_hex, not_after, last_error) VALUES (?, ?, ?, ?, ?, ?, NULL)`,
			c.Kind, domain, c.CertPath, c.KeyPath, c.SHA256Hex, c.NotAfter.Unix())
		if err != nil {
			return err
		}
		c.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return c, fmt.Errorf("save certificate: %w", err)
	}
	c.LastError = ""
	return c, nil
}

func (s *Store) SetCertificateError(ctx context.Context, msg string) error {
	var v any
	if msg != "" {
		v = msg
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE certificates SET last_error = ?`, v)
	if err != nil {
		return fmt.Errorf("certificate error: %w", err)
	}
	return nil
}
