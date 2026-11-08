package internal

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestConnectForImportPragmas(t *testing.T) {
	db, err := ConnectForImport(filepath.Join(t.TempDir(), "import.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close import database: %v", err)
		}
	})

	assertPragma(t, db, "synchronous", 1)
	assertPragma(t, db, "cache_size", -262144)
	assertPragma(t, db, "temp_store", 2)
}

func TestConnectKeepsDefaultPragmas(t *testing.T) {
	db, err := Connect(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close API database: %v", err)
		}
	})

	assertPragma(t, db, "synchronous", 1)
	assertPragma(t, db, "cache_size", -2000)
	assertPragma(t, db, "temp_store", 0)
}

func assertPragma(t *testing.T, db *sql.DB, pragma string, expected int) {
	t.Helper()

	var actual int
	if err := db.QueryRow("PRAGMA " + pragma).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("PRAGMA %s = %d, want %d", pragma, actual, expected)
	}
}
