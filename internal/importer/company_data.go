package importer

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/map-services/company-data-api/internal"
	"github.com/map-services/company-data-api/internal/models"
)

// stripSICDescription keeps only the first 5 characters of a SIC code, removing the " - description" suffix.
func stripSICDescription(s string) string {
	if idx := strings.Index(s, " - "); idx > 0 {
		return s[:idx]
	}
	// Return first 5 chars if no separator
	if len(s) > 5 {
		return s[:5]
	}
	return s
}

// parseNullableInt returns a pointer to int for non-empty strings, or nil for empty strings.
func parseNullableInt(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

type companyDataImporter struct {
	batchSize int
	db        *sql.DB

	// Reference data caches: value -> ID
	companyCategories  map[string]int
	companyStatuses    map[string]int
	accountsCategories map[string]int
	countries          map[string]int
	countryOrigins     map[string]int
}

func NewCompanyDataImporter(db *sql.DB) *companyDataImporter {
	return &companyDataImporter{
		batchSize:          5000,
		db:                 db,
		companyCategories:  make(map[string]int),
		companyStatuses:    make(map[string]int),
		accountsCategories: make(map[string]int),
		countries:          make(map[string]int),
		countryOrigins:     make(map[string]int),
	}
}

// getOrCreateRefID looks up or inserts a reference value and returns its ID.
func (imp *companyDataImporter) getOrCreateRefID(tx *sql.Tx, cache map[string]int, table, column, value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	if id, ok := cache[value]; ok {
		return id, nil
	}

	var id int
	err := tx.QueryRow(fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", table, column), value).Scan(&id)
	if err == sql.ErrNoRows {
		result, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", table, column), value)
		if err != nil {
			return 0, fmt.Errorf("failed to insert into %s: %w", table, err)
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get last insert ID from %s: %w", table, err)
		}
		id = int(lastID)
	} else if err != nil {
		return 0, fmt.Errorf("failed to query %s: %w", table, err)
	}
	cache[value] = id
	return id, nil
}

func fromCompanyDataCSV(record []string, headers []string) (*models.CompanyData, error) {
	if len(record) < 33 {
		return nil, fmt.Errorf("record has fewer fields than expected: %d", len(record))
	}

	var err error
	parseDateField := func(index int, fieldName string) *time.Time {
		if err != nil {
			return nil
		}
		var date *time.Time
		date, err = parseDate(record[index])
		if err != nil {
			err = fmt.Errorf("invalid %s: %w", fieldName, err)
		}
		return date
	}

	parseIntField := func(index int, fieldName string) int {
		if err != nil {
			return 0
		}
		var val int
		val, err = parseInt(record[index])
		if err != nil {
			err = fmt.Errorf("invalid %s: %w", fieldName, err)
		}
		return val
	}

	company := models.CompanyData{
		CompanyName:                       record[0],
		CompanyNumber:                     record[1],
		RegAddressCareOf:                  record[2],
		RegAddressPOBox:                   record[3],
		RegAddressAddressLine1:            record[4],
		RegAddressAddressLine2:            record[5],
		RegAddressPostTown:                record[6],
		RegAddressCounty:                  record[7],
		RegAddressCountry:                 record[8],
		RegAddressPostCode:                record[9],
		CompanyCategory:                   record[10],
		CompanyStatus:                     record[11],
		CountryOfOrigin:                   record[12],
		DissolutionDate:                   parseDateField(13, "DissolutionDate"),
		IncorporationDate:                 parseDateField(14, "IncorporationDate"),
		AccountsAccountRefDay:             parseIntField(15, "AccountsAccountRefDay"),
		AccountsAccountRefMonth:           parseIntField(16, "AccountsAccountRefMonth"),
		AccountsNextDueDate:               parseDateField(17, "AccountsNextDueDate"),
		AccountsLastMadeUpDate:            parseDateField(18, "AccountsLastMadeUpDate"),
		AccountsAccountCategory:           record[19],
		ReturnsNextDueDate:                parseDateField(20, "ReturnsNextDueDate"),
		ReturnsLastMadeUpDate:             parseDateField(21, "ReturnsLastMadeUpDate"),
		MortgagesNumCharges:               record[22],
		MortgagesNumOutstanding:           record[23],
		MortgagesNumPartSatisfied:         record[24],
		MortgagesNumSatisfied:             record[25],
		SICCode1:                          stripSICDescription(record[26]),
		SICCode2:                          stripSICDescription(record[27]),
		SICCode3:                          stripSICDescription(record[28]),
		SICCode4:                          stripSICDescription(record[29]),
		LimitedPartnershipsNumGenPartners: record[30],
		LimitedPartnershipsNumLimPartners: record[31],
		ConfStmtNextDueDate:               parseDateField(53, "ConfStmtNextDueDate"),
		ConfStmtLastMadeUpDate:            parseDateField(54, "ConfStmtLastMadeUpDate"),
	}

	if err != nil {
		return nil, err
	}

	return &company, nil
}

func companyDataToTuple(companyData models.CompanyData) []any {
	return []any{
		companyData.CompanyName,
		companyData.CompanyNumber,
		companyData.RegAddressCareOf,
		companyData.RegAddressPOBox,
		companyData.RegAddressAddressLine1,
		companyData.RegAddressAddressLine2,
		companyData.RegAddressPostTown,
		companyData.RegAddressCounty,
		companyData.RegAddressCountry,
		companyData.RegAddressPostCode,
		companyData.CompanyCategory,
		companyData.CompanyStatus,
		companyData.CountryOfOrigin,
		companyData.DissolutionDate,
		companyData.IncorporationDate,
		companyData.AccountsAccountRefDay,
		companyData.AccountsAccountRefMonth,
		companyData.AccountsNextDueDate,
		companyData.AccountsLastMadeUpDate,
		companyData.AccountsAccountCategory,
		companyData.ReturnsNextDueDate,
		companyData.ReturnsLastMadeUpDate,
		companyData.MortgagesNumCharges,
		companyData.MortgagesNumOutstanding,
		companyData.MortgagesNumPartSatisfied,
		companyData.MortgagesNumSatisfied,
		companyData.SICCode1,
		companyData.SICCode2,
		companyData.SICCode3,
		companyData.SICCode4,
		companyData.LimitedPartnershipsNumGenPartners,
		companyData.LimitedPartnershipsNumLimPartners,
		companyData.ConfStmtNextDueDate,
		companyData.ConfStmtLastMadeUpDate,
	}
}

func (importer *companyDataImporter) Import(zipPath string, _ http.Header) error {
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
		if err := importer.processCSV(f); err != nil {
			return fmt.Errorf("failed to process CSV data: %w", err)
		}
	}

	slog.Info("Import completed successfully!")
	slog.Info("Analyzing \"company_data\" table")
	if _, err = importer.db.Exec("ANALYZE company_data"); err != nil {
		return fmt.Errorf("failed to analyze \"company_data\" table: %w", err)
	}
	return nil
}

func (importer *companyDataImporter) processCSV(f *zip.File) error {
	r, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open embedded file %s in zip: %w", f.Name, err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			slog.Error("error closing embedded zip file", "error", err)
		}
	}()

	batch := make([]models.CompanyData, 0, importer.batchSize)
	lineNum := 0

	for result := range internal.ParseCSV(r, true, fromCompanyDataCSV) {
		lineNum = result.LineNum
		if result.Error != nil {
			return fmt.Errorf("error parsing line %d: %w", lineNum, result.Error)
		}

		batch = append(batch, *result.Value)

		if len(batch) >= importer.batchSize {
			if err := importer.insertBatch(batch, lineNum); err != nil {
				return fmt.Errorf("failed to insert company data batch at line %d: %w", lineNum, err)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := importer.insertBatch(batch, lineNum); err != nil {
			return fmt.Errorf("failed to insert final company data batch at line %d: %w", lineNum, err)
		}
	}

	return nil
}

func (importer *companyDataImporter) insertBatch(batch []models.CompanyData, lastLineNum int) error {
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

	stmt, err := tx.Prepare(internal.InsertCompanyDataSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Error("failed to close statement", "error", err)
		}
	}()

	for _, cd := range batch {
		// Resolve reference IDs
		catID, err := importer.getOrCreateRefID(tx, importer.companyCategories, "company_categories", "code", cd.CompanyCategory)
		if err != nil {
			return err
		}
		statID, err := importer.getOrCreateRefID(tx, importer.companyStatuses, "company_statuses", "code", cd.CompanyStatus)
		if err != nil {
			return err
		}
		countryOriginID, err := importer.getOrCreateRefID(tx, importer.countryOrigins, "country_origins", "name", cd.CountryOfOrigin)
		if err != nil {
			return err
		}
		accCatID, err := importer.getOrCreateRefID(tx, importer.accountsCategories, "accounts_categories", "code", cd.AccountsAccountCategory)
		if err != nil {
			return err
		}
		countryID, err := importer.getOrCreateRefID(tx, importer.countries, "countries", "name", cd.RegAddressCountry)
		if err != nil {
			return err
		}

		// Parse nullable numeric fields
		mortgagesCharges := parseNullableInt(cd.MortgagesNumCharges)
		mortgagesOutstanding := parseNullableInt(cd.MortgagesNumOutstanding)
		mortgagesPartSatisfied := parseNullableInt(cd.MortgagesNumPartSatisfied)
		mortgagesSatisfied := parseNullableInt(cd.MortgagesNumSatisfied)
		lpGenPartners := parseNullableInt(cd.LimitedPartnershipsNumGenPartners)
		lpLimPartners := parseNullableInt(cd.LimitedPartnershipsNumLimPartners)

		tuple := []any{
			cd.CompanyName,
			cd.CompanyNumber,
			cd.RegAddressCareOf,
			cd.RegAddressPOBox,
			cd.RegAddressAddressLine1,
			cd.RegAddressAddressLine2,
			cd.RegAddressPostTown,
			cd.RegAddressCounty,
			countryID,
			cd.RegAddressPostCode,
			catID,
			statID,
			countryOriginID,
			cd.DissolutionDate,
			cd.IncorporationDate,
			cd.AccountsAccountRefDay,
			cd.AccountsAccountRefMonth,
			cd.AccountsNextDueDate,
			cd.AccountsLastMadeUpDate,
			accCatID,
			cd.ReturnsNextDueDate,
			cd.ReturnsLastMadeUpDate,
			mortgagesCharges,
			mortgagesOutstanding,
			mortgagesPartSatisfied,
			mortgagesSatisfied,
			cd.SICCode1,
			cd.SICCode2,
			cd.SICCode3,
			cd.SICCode4,
			lpGenPartners,
			lpLimPartners,
			cd.ConfStmtNextDueDate,
			cd.ConfStmtLastMadeUpDate,
		}

		_, err = stmt.Exec(tuple...)
		if err != nil {
			return fmt.Errorf("failed to execute individual insert: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("Inserted records", "lastLineNum", lastLineNum)
	return nil
}
