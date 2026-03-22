package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type sharedDB struct {
	db    *sql.DB
	count int
}

var (
	dbPoolMu sync.Mutex
	dbPool   = map[string]*sharedDB{}
)

// acquireDB returns the shared *sql.DB for filePath, opening it and running migrations
// on first call. Each call increments an internal reference count; call releaseDB when done.
func acquireDB(filePath string) (*sql.DB, error) {
	dbPoolMu.Lock()
	defer dbPoolMu.Unlock()

	if s, ok := dbPool[filePath]; ok {
		s.count++
		return s.db, nil
	}

	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %q: %w", filePath, err)
	}
	// SQLite performs best with a single writer connection.
	db.SetMaxOpenConns(1)

	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	dbPool[filePath] = &sharedDB{db: db, count: 1}
	return db, nil
}

// releaseDB decrements the reference count for filePath and closes the DB when it reaches zero.
func releaseDB(filePath string) {
	dbPoolMu.Lock()
	defer dbPoolMu.Unlock()

	s, ok := dbPool[filePath]
	if !ok {
		return
	}
	s.count--
	if s.count <= 0 {
		s.db.Close()
		delete(dbPool, filePath)
	}
}

func applyMigrations(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for i, entry := range entries {
		version := i + 1

		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if count > 0 {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		statements := strings.Split(string(data), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
			}
		}

		if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}

	return nil
}
