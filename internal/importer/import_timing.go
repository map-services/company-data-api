package importer

import (
	"log/slog"
	"time"
)

func logImportStart(importer string) time.Time {
	startedAt := time.Now()
	slog.Info("Import started", "importer", importer, "startTime", startedAt)
	return startedAt
}

func logImportCompletion(importer string, startedAt time.Time, recordsProcessed int) {
	endedAt := time.Now()
	elapsed := endedAt.Sub(startedAt)
	recordsPerSecond := 0.0
	if elapsed > 0 {
		recordsPerSecond = float64(recordsProcessed) / elapsed.Seconds()
	}

	slog.Info("Import finished",
		"importer", importer,
		"startTime", startedAt,
		"endTime", endedAt,
		"elapsed", elapsed.String(),
		"recordsProcessed", recordsProcessed,
		"recordsPerSecond", recordsPerSecond,
	)
}
