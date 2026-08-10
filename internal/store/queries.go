package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

func newID() string { return uuid.NewString() }

// ---- Users ----

// UpsertUser returns the user for email, creating one if absent. Email is
// lower-cased by the caller.
func (s *Store) UpsertUser(email string) (*User, error) {
	if u, err := s.UserByEmail(email); err == nil {
		return u, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	u := &User{ID: newID(), Email: email, CreatedAt: time.Now().UTC()}
	_, err := s.db.Exec(s.rebind(
		`INSERT INTO users (id, email, created_at) VALUES (?, ?, ?)`),
		u.ID, u.Email, nowStr())
	if err != nil {
		// Lost a create race; re-read.
		if u2, e2 := s.UserByEmail(email); e2 == nil {
			return u2, nil
		}
		return nil, err
	}
	return u, nil
}

func (s *Store) UserByEmail(email string) (*User, error) {
	var row struct {
		ID        string `db:"id"`
		Email     string `db:"email"`
		CreatedAt string `db:"created_at"`
	}
	err := s.db.Get(&row, s.rebind(`SELECT id, email, created_at FROM users WHERE email = ?`), email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &User{ID: row.ID, Email: row.Email, CreatedAt: parseTime(row.CreatedAt)}, nil
}

func (s *Store) UserByID(id string) (*User, error) {
	var row struct {
		ID        string `db:"id"`
		Email     string `db:"email"`
		CreatedAt string `db:"created_at"`
	}
	err := s.db.Get(&row, s.rebind(`SELECT id, email, created_at FROM users WHERE id = ?`), id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &User{ID: row.ID, Email: row.Email, CreatedAt: parseTime(row.CreatedAt)}, nil
}

// ---- Login tokens (magic link) ----

func (s *Store) CreateLoginToken(tokenHash, email string, ttl time.Duration) error {
	_, err := s.db.Exec(s.rebind(
		`INSERT INTO login_tokens (id, token_hash, email, expires_at, used_at, created_at)
		 VALUES (?, ?, ?, ?, NULL, ?)`),
		newID(), tokenHash, email, time.Now().UTC().Add(ttl).Format(time.RFC3339Nano), nowStr())
	return err
}

// ConsumeLoginToken atomically validates and marks a token used, returning the
// email it was issued for. Rejects unknown, expired, or already-used tokens.
func (s *Store) ConsumeLoginToken(tokenHash string) (string, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var row struct {
		Email     string         `db:"email"`
		ExpiresAt string         `db:"expires_at"`
		UsedAt    sql.NullString `db:"used_at"`
	}
	err = tx.Get(&row, tx.Rebind(
		`SELECT email, expires_at, used_at FROM login_tokens WHERE token_hash = ?`), tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if row.UsedAt.Valid {
		return "", errors.New("token already used")
	}
	if parseTime(row.ExpiresAt).Before(time.Now().UTC()) {
		return "", errors.New("token expired")
	}
	if _, err := tx.Exec(tx.Rebind(
		`UPDATE login_tokens SET used_at = ? WHERE token_hash = ? AND used_at IS NULL`),
		nowStr(), tokenHash); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return row.Email, nil
}

// ---- Sessions ----

func (s *Store) CreateSession(id, userID string, ttl time.Duration) error {
	_, err := s.db.Exec(s.rebind(
		`INSERT INTO sessions (id, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`),
		id, userID, time.Now().UTC().Add(ttl).Format(time.RFC3339Nano), nowStr())
	return err
}

// SessionUser returns the user for a live (non-expired) session id.
func (s *Store) SessionUser(sessionID string) (*User, error) {
	var row struct {
		UserID    string `db:"user_id"`
		ExpiresAt string `db:"expires_at"`
	}
	err := s.db.Get(&row, s.rebind(`SELECT user_id, expires_at FROM sessions WHERE id = ?`), sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parseTime(row.ExpiresAt).Before(time.Now().UTC()) {
		_ = s.DeleteSession(sessionID)
		return nil, ErrNotFound
	}
	return s.UserByID(row.UserID)
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(s.rebind(`DELETE FROM sessions WHERE id = ?`), id)
	return err
}

// ---- Tags ----

func (s *Store) CreateTag(userID, guid, name string, showEmail, showPhone bool, phone string) (*Tag, error) {
	t := &Tag{ID: newID(), GUID: guid, UserID: userID, Name: name,
		ShowEmail: showEmail, ShowPhone: showPhone, Phone: phone, CreatedAt: time.Now().UTC()}
	_, err := s.db.Exec(s.rebind(
		`INSERT INTO tags (id, guid, user_id, name, show_email, show_phone, phone, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
		t.ID, t.GUID, t.UserID, t.Name, b2i(showEmail), b2i(showPhone), t.Phone, nowStr())
	if err != nil {
		return nil, err
	}
	return t, nil
}

type tagRow struct {
	ID        string `db:"id"`
	GUID      string `db:"guid"`
	UserID    string `db:"user_id"`
	Name      string `db:"name"`
	ShowEmail int    `db:"show_email"`
	ShowPhone int    `db:"show_phone"`
	Phone     string `db:"phone"`
	CreatedAt string `db:"created_at"`
}

func (r tagRow) toTag() *Tag {
	return &Tag{ID: r.ID, GUID: r.GUID, UserID: r.UserID, Name: r.Name,
		ShowEmail: r.ShowEmail != 0, ShowPhone: r.ShowPhone != 0, Phone: r.Phone,
		CreatedAt: parseTime(r.CreatedAt)}
}

const tagCols = `id, guid, user_id, name, show_email, show_phone, phone, created_at`

func (s *Store) TagsByUser(userID string) ([]*Tag, error) {
	var rows []tagRow
	err := s.db.Select(&rows, s.rebind(
		`SELECT `+tagCols+` FROM tags WHERE user_id = ? ORDER BY created_at DESC`), userID)
	if err != nil {
		return nil, err
	}
	out := make([]*Tag, len(rows))
	for i, r := range rows {
		out[i] = r.toTag()
	}
	return out, nil
}

func (s *Store) TagByGUID(guid string) (*Tag, error) {
	var r tagRow
	err := s.db.Get(&r, s.rebind(`SELECT `+tagCols+` FROM tags WHERE guid = ?`), guid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.toTag(), nil
}

func (s *Store) UpdateTag(guid, userID, name string, showEmail, showPhone bool, phone string) (*Tag, error) {
	res, err := s.db.Exec(s.rebind(
		`UPDATE tags SET name = ?, show_email = ?, show_phone = ?, phone = ?
		 WHERE guid = ? AND user_id = ?`),
		name, b2i(showEmail), b2i(showPhone), phone, guid, userID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.TagByGUID(guid)
}

func (s *Store) DeleteTag(guid, userID string) error {
	res, err := s.db.Exec(s.rebind(`DELETE FROM tags WHERE guid = ? AND user_id = ?`), guid, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Found reports ----

func (s *Store) CreateFoundReport(r *FoundReport) error {
	r.ID = newID()
	r.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(s.rebind(
		`INSERT INTO found_reports (id, tag_id, finder_name, finder_email, finder_phone, message, remote_ip, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
		r.ID, r.TagID, r.FinderName, r.FinderEmail, r.FinderPhone, r.Message, r.RemoteIP, nowStr())
	return err
}
