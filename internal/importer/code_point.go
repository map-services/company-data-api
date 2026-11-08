package importer

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/map-services/company-data-api/internal"
)

type CodePoint struct {
	PostCode string `json:"post_code"`
	Easting  int    `json:"easting"`
	Northing int    `json:"northing"`
}

func fromCodePointCSV(record []string, headers []string) (*CodePoint, error) {
	easting, err := parseInt(record[2])
	if err != nil {
		return nil, err
	}
	northing, err := parseInt(record[3])
	if err != nil {
		return nil, err
	}

	return &CodePoint{
		PostCode: record[0],
		Easting:  easting,
		Northing: northing,
	}, nil
}

func codePointToTuple(codePoint CodePoint) []any {
	return []any{
		codePoint.PostCode,
		codePoint.Easting,
		codePoint.Northing,
	}
}

type codePointImporter struct {
	batchSize int
	db        *sql.DB
}

func NewCodePointImporter(db *sql.DB) *codePointImporter {
	return &codePointImporter{
		batchSize: 5000,
		db:        db,
	}
}

func (importer *codePointImporter) Import(zipPath string, _ http.Header) error {
	startedAt := logImportStart("code_point")
	totalRecordsImported := 0
	defer func() {
		logImportCompletion("code_point", startedAt, totalRecordsImported)
	}()

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			slog.Error("error closing zip file", "error", err)
		}
	}()

	for _, f := range r.File {
		if f.FileInfo().IsDir() || !strings.HasPrefix(f.Name, "Data/CSV/") {
			continue
		}
		recordsInFile, err := importer.processCSV(f)
		totalRecordsImported += recordsInFile
		if err != nil {
			return fmt.Errorf("failed to process CSV data: %w", err)
		}
		slog.Info("Processed file", "filename", f.Name, "records", recordsInFile)
	}

	slog.Info("Completed successfully", "totalRecords", totalRecordsImported)
	slog.Info("Analyzing \"code_point\" table")
	if _, err = importer.db.Exec("ANALYZE code_point"); err != nil {
		return fmt.Errorf("failed to analyze \"code_point\" table: %w", err)
	}
	return nil
}

func (importer *codePointImporter) processCSV(f *zip.File) (int, error) {
	r, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("failed to open embedded file %s in zip: %w", f.Name, err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			slog.Error("error closing embedded zip file", "error", err)
		}
	}()

	batch := make([]CodePoint, 0, importer.batchSize)
	lineNum := 0
	recordsProcessed := 0

	for result := range internal.ParseCSV(r, false, fromCodePointCSV) {
		lineNum = result.LineNum
		if result.Error != nil {
			return recordsProcessed, fmt.Errorf("error parsing line %d: %w", lineNum, result.Error)
		}

		batch = append(batch, *result.Value)

		if len(batch) >= importer.batchSize {
			if err := importer.insertBatch(batch); err != nil {
				return recordsProcessed, fmt.Errorf("failed to insert batch: %w", err)
			}
			recordsProcessed += len(batch)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := importer.insertBatch(batch); err != nil {
			return recordsProcessed, fmt.Errorf("failed to insert batch: %w", err)
		}
		recordsProcessed += len(batch)
	}
	return recordsProcessed, nil
}

func (importer *codePointImporter) insertBatch(batch []CodePoint) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := importer.db.Begin()
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

	stmt, err := tx.Prepare(internal.InsertCodePointSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Error("failed to close statement", "error", err)
		}
	}()

	for _, codePoint := range batch {
		_, err = stmt.Exec(codePointToTuple(codePoint)...)
		if err != nil {
			return fmt.Errorf("failed to execute individual insert: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
