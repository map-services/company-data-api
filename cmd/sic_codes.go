package cmd

import (
	"log/slog"
	"os"

	"github.com/map-services/company-data-api/internal"
	"github.com/map-services/company-data-api/internal/importer"
	"github.com/rm-hull/godx"
)

func ImportSICCodes(csvFile string, dbPath string) {
	logger := internal.SetupLogger()
	godx.Diagnostics(logger)

	db, err := internal.Connect(dbPath)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("error closing database", "error", err)
		}
	}()

	err = internal.TransientDownload(csvFile, importer.NewSICCodesImporter(db).Import)
	if err != nil {
		slog.Error("failed to import SIC codes", "error", err)
		os.Exit(1)
	}
}
