package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"test/storage"

	_ "github.com/mattn/go-sqlite3"
	//"read-adviser-bot/storage"
)

type Storage struct {
	db *sql.DB
}

// ListPrepared retrieves all pages for a specific user from the database.
func (s *Storage) ListPrepared(ctx context.Context, userName string) (*[]storage.Page, error) {
	q := `SELECT url FROM pages WHERE user_name = ?`

	rows, err := s.db.QueryContext(ctx, q, userName)
	if err != nil {
		return nil, fmt.Errorf("can't retrieve pages: %w", err)
	}
	defer rows.Close()

	var pages []storage.Page
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("can't scan page: %w", err)
		}
		pages = append(pages, storage.Page{
			URL:      url,
			Username: userName,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred during rows iteration: %w", err)
	}
	if len(pages) == 0 {
		return nil, storage.ErrNoSavedPages
	}
	return &pages, nil
}

// New creates new SQLite storage.
func New(path string) (*Storage, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("can't open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("can't connect to database: %w", err)
	}
	return &Storage{db: db}, nil
}

// Save saves page to storage.
func (s *Storage) Save(ctx context.Context, p *storage.Page) error {
	q := `INSERT INTO pages (url, user_name) VALUES (?, ?)`
	if _, err := s.db.ExecContext(ctx, q, p.URL, p.Username); err != nil {
		return fmt.Errorf("can't save page: %w", err)
	}
	return nil
}

// PickRandom picks random page from storage.
func (s *Storage) PickRandom(ctx context.Context, userName string) (*storage.Page, error) {
	q := `SELECT url FROM pages WHERE user_name = ? ORDER BY RANDOM() LIMIT 1`
	var url string
	err := s.db.QueryRowContext(ctx, q, userName).Scan(&url)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNoSavedPages
	}
	if err != nil {
		return nil, fmt.Errorf("can't pick random page: %w", err)
	}
	return &storage.Page{
		URL:      url,
		Username: userName,
	}, nil
}

// Remove removes page from storage.
func (s *Storage) Remove(ctx context.Context, page *storage.Page) error {
	q := `DELETE FROM pages WHERE url = ? AND user_name = ?`
	if _, err := s.db.ExecContext(ctx, q, page.URL, page.Username); err != nil {
		return fmt.Errorf("can't remove page: %w", err)
	}
	return nil
}

// IsExists checks if page exists in storage.
func (s *Storage) IsExists(ctx context.Context, page *storage.Page) (bool, error) {
	q := `SELECT COUNT(*) FROM pages WHERE url = ? AND user_name = ?`
	var count int
	if err := s.db.QueryRowContext(ctx, q, page.URL, page.Username).Scan(&count); err != nil {
		return false, fmt.Errorf("can't check if page exists: %w", err)
	}
	return count > 0, nil
}
func (s *Storage) Init(ctx context.Context) error {
	q := `CREATE TABLE IF NOT EXISTS pages (url TEXT, user_name TEXT)`
	_, err := s.db.ExecContext(ctx, q)
	if err != nil {
		return fmt.Errorf("can't create table: %w", err)
	}
	return nil
}
