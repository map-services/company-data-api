package internal

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"
)

//go:embed sql/migration.sql
var migrationSQL string

//go:embed sql/company_data_postcode_index.sql
var CompanyDataPostcodeIndexSQL string

const DropCompanyDataPostcodeIndexSQL = "DROP INDEX IF EXISTS idx_company_data_reg_address_post_code"

//go:embed sql/insert_code_point.sql
var InsertCodePointSQL string

//go:embed sql/insert_company_data.sql
var InsertCompanyDataSQL string

//go:embed sql/insert_sic_code.sql
var InsertSICCodeSQL string

//go:embed sql/search.sql
var SearchSQL string

func CreateDB(db *sql.DB) error {
	if _, err := db.Exec(migrationSQL); err != nil {
		return err
	}
	_, err := db.Exec(CompanyDataPostcodeIndexSQL)
	return err
}

func Connect(dbPath string) (*sql.DB, error) {
	return connect(dbPath, false)
}

func ConnectForImport(dbPath string) (*sql.DB, error) {
	return connect(dbPath, true)
}

func connect(dbPath string, forImport bool) (*sql.DB, error) {
	dsn := dbPath
	if strings.Contains(dsn, "?") {
		dsn += "&"
	} else {
		dsn += "?"
	}
	queryParams := []string{"_busy_timeout=5000", "_journal_mode=WAL"}
	if forImport {
		queryParams = append(queryParams, "_synchronous=NORMAL", "_cache_size=-262144", "_temp_store=MEMORY")
	}
	dsn += strings.Join(queryParams, "&")
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if forImport {
		db.SetMaxOpenConns(1)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if forImport {
		if _, err = db.Exec("PRAGMA temp_store = MEMORY"); err != nil {
			return nil, fmt.Errorf("failed to configure import temp storage: %w", err)
		}
	}
	slog.Info("connected to database", "dsn", dsn)

	if err = CreateDB(db); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	return db, nil
}
