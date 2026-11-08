package importer

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/map-services/company-data-api/internal"
)

type sicCodesImporter struct {
	batchSize int
	db        *sql.DB
}

func NewSICCodesImporter(db *sql.DB) *sicCodesImporter {
	return &sicCodesImporter{
		batchSize: 100,
		db:        db,
	}
}

func (imp *sicCodesImporter) Import(csvPath string, _ http.Header) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("error closing CSV file", "error", err)
		}
	}()

	batch := make([][2]string, 0, imp.batchSize)

	for result := range internal.ParseCSV(file, true, func(record []string, headers []string) ([2]string, error) {
		if len(record) < 2 {
			return [2]string{}, fmt.Errorf("expected 2 columns, got %d", len(record))
		}
		code := record[0]
		description := record[1]
		if code == "" {
			return [2]string{}, fmt.Errorf("empty SIC code")
		}
		return [2]string{code, description}, nil
	}) {
		if result.Error != nil {
			return fmt.Errorf("error parsing line %d: %w", result.LineNum, result.Error)
		}

		batch = append(batch, result.Value)

		if len(batch) >= imp.batchSize {
			if err := imp.insertBatch(batch); err != nil {
				return fmt.Errorf("failed to insert SIC codes batch at line %d: %w", result.LineNum, err)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := imp.insertBatch(batch); err != nil {
			return fmt.Errorf("failed to insert final SIC codes batch: %w", err)
		}
	}

	slog.Info("SIC codes import completed successfully")
	if _, err = imp.db.Exec("ANALYZE sic_codes"); err != nil {
		return fmt.Errorf("failed to analyze sic_codes table: %w", err)
	}
	return nil
}

func (imp *sicCodesImporter) insertBatch(batch [][2]string) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := imp.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Error("error rolling back transaction", "error", rbErr)
			}
		}
	}()

	stmt, err := tx.Prepare(internal.InsertSICCodeSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Error("failed to close statement", "error", err)
		}
	}()

	for _, item := range batch {
		_, err = stmt.Exec(item[0], item[1])
		if err != nil {
			return fmt.Errorf("failed to execute insert: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("Inserted SIC codes", "count", len(batch))
	return nil
}
