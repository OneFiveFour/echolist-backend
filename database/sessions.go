package database

import (
	"context"
	"database/sql"
	"errors"
)

// DeviceSession contains metadata and token identifiers, never bearer tokens.
// All timestamps are Unix milliseconds. Revoked/expired rows remain as tombstones.
type DeviceSession struct {
	ID, Username, DeviceName         string
	CreatedAt, LastUsedAt, ExpiresAt int64
	RefreshID, PreviousRefreshID     string
	PreviousValidUntil, RevokedAt    int64
}

var ErrInvalidSession = errors.New("invalid or expired session")

const sessionColumns = `id, username, device_name, created_at, last_used_at, expires_at,
 refresh_id, previous_refresh_id, previous_valid_until, revoked_at`

func scanSession(row interface{ Scan(...any) error }) (*DeviceSession, error) {
	s := &DeviceSession{}
	err := row.Scan(&s.ID, &s.Username, &s.DeviceName, &s.CreatedAt, &s.LastUsedAt,
		&s.ExpiresAt, &s.RefreshID, &s.PreviousRefreshID, &s.PreviousValidUntil, &s.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidSession
	}
	return s, err
}

func (d *Database) CreateSession(ctx context.Context, s *DeviceSession) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO auth_sessions (`+sessionColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.Username, s.DeviceName, s.CreatedAt, s.LastUsedAt, s.ExpiresAt,
		s.RefreshID, s.PreviousRefreshID, s.PreviousValidUntil, s.RevokedAt)
	return err
}

func (d *Database) GetActiveSession(ctx context.Context, id, username string, now int64) (*DeviceSession, error) {
	return scanSession(d.db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM auth_sessions
 WHERE id=? AND username=? AND revoked_at=0 AND expires_at>?`, id, username, now))
}

// RotateSession uses one atomic conditional UPDATE, safe across backend instances.
// The immediately preceding refresh token may recover the current token for 60s
// after rotation (lost responses / concurrent tabs). It cannot extend the expiry.
// A replay outside this window revokes the entire session.
func (d *Database) RotateSession(ctx context.Context, id, username, refreshID, nextID string, now, expires int64) (*DeviceSession, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE auth_sessions SET
 previous_refresh_id=refresh_id, previous_valid_until=?, refresh_id=?, last_used_at=?, expires_at=?
 WHERE id=? AND username=? AND revoked_at=0 AND expires_at>? AND refresh_id=?
 AND previous_valid_until<=?`, now+60000, nextID, now, expires, id, username, now, refreshID, now)
	if err != nil {
		return nil, err
	}
	s, err := scanSession(tx.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM auth_sessions
 WHERE id=? AND username=? AND revoked_at=0 AND expires_at>?`, id, username, now))
	if err != nil {
		return nil, err
	}
	// Current token is also returned unchanged during the grace period. This
	// prevents rapid rotations from invalidating concurrent responses.
	if refreshID != s.RefreshID && !(refreshID == s.PreviousRefreshID && now < s.PreviousValidUntil) {
		_, err = tx.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=? WHERE id=?`, now, id)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, ErrInvalidSession
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s, nil
}

func (d *Database) ListSessions(ctx context.Context, username string, now int64) ([]*DeviceSession, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT `+sessionColumns+` FROM auth_sessions
 WHERE username=? AND revoked_at=0 AND expires_at>? ORDER BY last_used_at DESC, id`, username, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []*DeviceSession{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// RevokeSession is idempotent and deliberately does not reveal other users' IDs.
func (d *Database) RevokeSession(ctx context.Context, id, username string, now int64) error {
	_, err := d.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=?
 WHERE id=? AND username=? AND revoked_at=0`, now, id, username)
	return err
}
