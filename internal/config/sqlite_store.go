package config

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Values struct {
	ChromaURL       string
	CollectionName  string
	BackendHTTPPort int
	MCPTransport    string
}

const (
	defaultChromaURL      = "http://localhost:8000"
	defaultCollectionName = "default"
	defaultHTTPPort       = 8080
	defaultMCPTransport   = "stdio"
)

func Ensure(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	st := &Store{db: db}
	if err := st.migrate(); err != nil {
		return nil, err
	}
	if err := st.seedDefaults(); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (s *Store) seedDefaults() error {
	// insert if not exists
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	ins := `INSERT OR IGNORE INTO config(key,value) VALUES(?,?)`
	pairs := [][2]string{
		{"chroma_url", defaultChromaURL},
		{"collection_name", defaultCollectionName},
		{"backend_http_port", fmt.Sprintf("%d", defaultHTTPPort)},
		{"mcp_transport", defaultMCPTransport},
	}
	for _, p := range pairs {
		if _, err := tx.Exec(ins, p[0], p[1]); err != nil {
			return fmt.Errorf("seed %s: %w", p[0], err)
		}
	}
	return tx.Commit()
}

func (s *Store) GetAll() (Values, error) {
	rows, err := s.db.Query(`SELECT key, value FROM config`)
	if err != nil {
		return Values{}, err
	}
	defer rows.Close()
	vals := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return Values{}, err
		}
		vals[k] = v
	}
	if err := rows.Err(); err != nil {
		return Values{}, err
	}
	v := Values{
		ChromaURL:       pick(vals, "chroma_url", defaultChromaURL),
		CollectionName:  pick(vals, "collection_name", defaultCollectionName),
		BackendHTTPPort: atoi(pick(vals, "backend_http_port", fmt.Sprintf("%d", defaultHTTPPort))),
		MCPTransport:    pick(vals, "mcp_transport", defaultMCPTransport),
	}
	return v, nil
}

func (s *Store) Get(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO config(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// helpers
func pick(m map[string]string, k, d string) string {
	if v, ok := m[k]; ok && v != "" {
		return v
	}
	return d
}

func atoi(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}
