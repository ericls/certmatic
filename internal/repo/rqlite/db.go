package rqlite

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	gorqlite "github.com/rqlite/gorqlite"

	// Reuse the same migration SQL files as the sqlite backend.
	"github.com/ericls/certmatic/internal/repo/sqlite"
)

type sharedConn struct {
	conn  *gorqlite.Connection
	mu    sync.Mutex // gorqlite connections are not thread-safe
	count int
}

var (
	connPoolMu sync.Mutex
	connPool   = map[string]*sharedConn{}
)

// acquireConn returns the shared *gorqlite.Connection for httpAddr, opening it and
// running migrations on first call. Each call increments an internal reference count;
// call releaseConn when done.
func acquireConn(httpAddr string) (*sharedConn, error) {
	connPoolMu.Lock()
	defer connPoolMu.Unlock()

	if s, ok := connPool[httpAddr]; ok {
		s.count++
		return s, nil
	}

	conn, err := gorqlite.Open(httpAddr)
	if err != nil {
		return nil, fmt.Errorf("open rqlite connection %q: %w", httpAddr, err)
	}

	sc := &sharedConn{conn: conn, count: 1}

	if err := applyMigrations(sc); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	connPool[httpAddr] = sc
	return sc, nil
}

// releaseConn decrements the reference count for httpAddr and closes the connection when it reaches zero.
func releaseConn(httpAddr string) {
	connPoolMu.Lock()
	defer connPoolMu.Unlock()

	s, ok := connPool[httpAddr]
	if !ok {
		return
	}
	s.count--
	if s.count <= 0 {
		s.conn.Close()
		delete(connPool, httpAddr)
	}
}

func applyMigrations(sc *sharedConn) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Create schema_migrations table if it doesn't exist.
	_, err := sc.conn.WriteOne(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := sqlite.MigrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for i, entry := range entries {
		version := i + 1

		// Check if migration is already applied.
		qr, err := sc.conn.QueryOneParameterized(gorqlite.ParameterizedStatement{
			Query:     `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`,
			Arguments: []interface{}{version},
		})
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if qr.Next() {
			var count int
			if err := qr.Scan(&count); err != nil {
				return fmt.Errorf("scan migration count %d: %w", version, err)
			}
			if count > 0 {
				continue
			}
		}

		data, err := sqlite.MigrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		// Batch all DDL statements + version record in a single atomic write.
		var stmts []string
		for _, stmt := range strings.Split(string(data), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
		}
		stmts = append(stmts, fmt.Sprintf(`INSERT INTO schema_migrations (version) VALUES (%d)`, version))

		results, err := sc.conn.Write(stmts)
		if err != nil {
			return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
		}
		for _, wr := range results {
			if wr.Err != nil {
				return fmt.Errorf("execute migration %s: %w", entry.Name(), wr.Err)
			}
		}
	}

	return nil
}
