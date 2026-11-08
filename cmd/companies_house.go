package cmd

import (
	"log/slog"
	"os"

	"github.com/map-services/company-data-api/internal"
	"github.com/map-services/company-data-api/internal/importer"
	"github.com/rm-hull/godx"
)

func ImportCompaniesHouseZipFile(zipFile string, dbPath string, maxRecords int) {
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

	err = internal.TransientDownload(zipFile, importer.NewCompanyDataImporter(db, maxRecords).Import)
	if err != nil {
		slog.Error("failed to import company data", "error", err)
		os.Exit(1)
	}
}
