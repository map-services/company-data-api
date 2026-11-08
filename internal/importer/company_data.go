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
		batchSize:          20000,
		db:                 db,
		companyCategories:  make(map[string]int),
		companyStatuses:    make(map[string]int),
		accountsCategories: make(map[string]int),
		countries:          make(map[string]int),
		countryOrigins:     make(map[string]int),
	}
}

// prepopulateRefCaches loads all existing reference table IDs into memory before import.
func (imp *companyDataImporter) prepopulateRefCaches() error {
	tables := []struct {
		cache  map[string]int
		table  string
		column string
	}{
		{imp.companyCategories, "company_categories", "code"},
		{imp.companyStatuses, "company_statuses", "code"},
		{imp.accountsCategories, "accounts_categories", "code"},
		{imp.countries, "countries", "name"},
		{imp.countryOrigins, "country_origins", "name"},
	}

	for _, t := range tables {
		rows, err := imp.db.Query(fmt.Sprintf("SELECT id, %s FROM %s", t.column, t.table))
		if err != nil {
			return fmt.Errorf("failed to query %s: %w", t.table, err)
		}
		for rows.Next() {
			var id int
			var value string
			if err := rows.Scan(&id, &value); err != nil {
				_ = rows.Close()
				return fmt.Errorf("failed to scan %s: %w", t.table, err)
			}
			t.cache[value] = id
		}
		_ = rows.Close()
	}
	slog.Info("Pre-populated reference caches")
	return nil
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

// csvRow holds the raw CSV fields needed for import, avoiding the full CompanyData struct.
type csvRow struct {
	// Text fields (indices 0-12)
	companyName        string // 0
	companyNumber      string // 1
	regAddressCareOf   string // 2
	regAddressPOBox    string // 3
	regAddressLine1    string // 4
	regAddressLine2    string // 5
	regAddressPostTown string // 6
	regAddressCounty   string // 7
	regAddressCountry  string // 8
	regAddressPostCode string // 9
	companyCategory       string // 10
	companyStatus         string // 11
	countryOfOrigin       string // 12
	accountsAccCategory   string // 19
	// Dates (indices 13-14, 17-18, 20-21, 53-54)
	dissolutionDate        *time.Time // 13
	incorporationDate      *time.Time // 14
	accountsNextDueDate    *time.Time // 17
	accountsLastMadeUpDate *time.Time // 18
	returnsNextDueDate     *time.Time // 20
	returnsLastMadeUpDate  *time.Time // 21
	confStmtNextDueDate    *time.Time // 53
	confStmtLastMadeUpDate *time.Time // 54
	// Int fields (indices 15-16)
	accountsRefDay   int // 15
	accountsRefMonth int // 16
	// Numeric string fields (indices 22-25, 30-31)
	mortgagesCharges     string // 22
	mortgagesOutstanding string // 23
	mortgagesPartSatisfied string // 24
	mortgagesSatisfied   string // 25
	lpGenPartners        string // 30
	lpLimPartners        string // 31
	// SIC codes (indices 26-29, stripped)
	sicCode1 string // 26
	sicCode2 string // 27
	sicCode3 string // 28
	sicCode4 string // 29
}

func parseCSVRow(record []string) (*csvRow, error) {
	if len(record) < 33 {
		return nil, fmt.Errorf("record has fewer fields than expected: %d", len(record))
	}

	var err error
	parseDateField := func(index int, fieldName string) *time.Time {
		if err != nil {
			return nil
		}
		date, parseErr := parseDate(record[index])
		if parseErr != nil {
			err = fmt.Errorf("invalid %s: %w", fieldName, parseErr)
		}
		return date
	}

	parseIntField := func(index int, fieldName string) int {
		if err != nil {
			return 0
		}
		val, parseErr := parseInt(record[index])
		if parseErr != nil {
			err = fmt.Errorf("invalid %s: %w", fieldName, parseErr)
		}
		return val
	}

	row := &csvRow{
		companyName:          record[0],
		companyNumber:        record[1],
		regAddressCareOf:     record[2],
		regAddressPOBox:      record[3],
		regAddressLine1:      record[4],
		regAddressLine2:      record[5],
		regAddressPostTown:   record[6],
		regAddressCounty:     record[7],
		regAddressCountry:    record[8],
		regAddressPostCode:   record[9],
		companyCategory:       record[10],
		companyStatus:         record[11],
		countryOfOrigin:       record[12],
		accountsAccCategory:   record[19],
		dissolutionDate:      parseDateField(13, "DissolutionDate"),
		incorporationDate:    parseDateField(14, "IncorporationDate"),
		accountsRefDay:       parseIntField(15, "AccountsAccountRefDay"),
		accountsRefMonth:     parseIntField(16, "AccountsAccountRefMonth"),
		accountsNextDueDate:  parseDateField(17, "AccountsNextDueDate"),
		accountsLastMadeUpDate: parseDateField(18, "AccountsLastMadeUpDate"),
		returnsNextDueDate:   parseDateField(20, "ReturnsNextDueDate"),
		returnsLastMadeUpDate: parseDateField(21, "ReturnsLastMadeUpDate"),
		mortgagesCharges:     record[22],
		mortgagesOutstanding: record[23],
		mortgagesPartSatisfied: record[24],
		mortgagesSatisfied:   record[25],
		sicCode1:             stripSICDescription(record[26]),
		sicCode2:             stripSICDescription(record[27]),
		sicCode3:             stripSICDescription(record[28]),
		sicCode4:             stripSICDescription(record[29]),
		lpGenPartners:        record[30],
		lpLimPartners:        record[31],
		confStmtNextDueDate:  parseDateField(53, "ConfStmtNextDueDate"),
		confStmtLastMadeUpDate: parseDateField(54, "ConfStmtLastMadeUpDate"),
	}

	if err != nil {
		return nil, err
	}

	return row, nil
}

func fromCompanyDataCSV(record []string, headers []string) (*models.CompanyData, error) {
	row, err := parseCSVRow(record)
	if err != nil {
		return nil, err
	}

	company := &models.CompanyData{
		CompanyName:                       row.companyName,
		CompanyNumber:                     row.companyNumber,
		RegAddressCareOf:                  row.regAddressCareOf,
		RegAddressPOBox:                   row.regAddressPOBox,
		RegAddressAddressLine1:            row.regAddressLine1,
		RegAddressAddressLine2:            row.regAddressLine2,
		RegAddressPostTown:                row.regAddressPostTown,
		RegAddressCounty:                  row.regAddressCounty,
		RegAddressCountry:                 row.regAddressCountry,
		RegAddressPostCode:                row.regAddressPostCode,
		CompanyCategory:                   row.companyCategory,
		CompanyStatus:                     row.companyStatus,
		CountryOfOrigin:                   row.countryOfOrigin,
		DissolutionDate:                   row.dissolutionDate,
		IncorporationDate:                 row.incorporationDate,
		AccountsAccountRefDay:             row.accountsRefDay,
		AccountsAccountRefMonth:           row.accountsRefMonth,
		AccountsNextDueDate:               row.accountsNextDueDate,
		AccountsLastMadeUpDate:            row.accountsLastMadeUpDate,
		AccountsAccountCategory:           row.accountsAccCategory,
		ReturnsNextDueDate:                row.returnsNextDueDate,
		ReturnsLastMadeUpDate:             row.returnsLastMadeUpDate,
		MortgagesNumCharges:               row.mortgagesCharges,
		MortgagesNumOutstanding:           row.mortgagesOutstanding,
		MortgagesNumPartSatisfied:         row.mortgagesPartSatisfied,
		MortgagesNumSatisfied:             row.mortgagesSatisfied,
		SICCode1:                          row.sicCode1,
		SICCode2:                          row.sicCode2,
		SICCode3:                          row.sicCode3,
		SICCode4:                          row.sicCode4,
		LimitedPartnershipsNumGenPartners: row.lpGenPartners,
		LimitedPartnershipsNumLimPartners: row.lpLimPartners,
		ConfStmtNextDueDate:               row.confStmtNextDueDate,
		ConfStmtLastMadeUpDate:            row.confStmtLastMadeUpDate,
	}
	return company, nil
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
	startTime := time.Now()

	r, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open embedded file %s in zip: %w", f.Name, err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			slog.Error("error closing embedded zip file", "error", err)
		}
	}()

	// Pre-populate reference caches to avoid per-row DB lookups
	if err := importer.prepopulateRefCaches(); err != nil {
		return fmt.Errorf("failed to pre-populate reference caches: %w", err)
	}

	// Parse CSV directly to csvRow, skipping the models.CompanyData struct
	batch := make([]*csvRow, 0, importer.batchSize)
	lineNum := 0

	for result := range internal.ParseCSV(r, true, func(record []string, headers []string) (*csvRow, error) {
		return parseCSVRow(record)
	}) {
		lineNum = result.LineNum
		if result.Error != nil {
			return fmt.Errorf("error parsing line %d: %w", lineNum, result.Error)
		}

		batch = append(batch, result.Value)

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

	importer.logElapsed(startTime, lineNum)
	return nil
}

const insertCompanyDataSQL = `INSERT OR REPLACE INTO company_data (
	company_name, company_number, reg_address_care_of, reg_address_po_box,
	reg_address_address_line_1, reg_address_address_line_2, reg_address_post_town,
	reg_address_county, reg_address_country, reg_address_post_code,
	company_category, company_status, country_of_origin, dissolution_date, incorporation_date,
	accounts_account_ref_day, accounts_account_ref_month, accounts_next_due_date,
	accounts_last_made_up_date, accounts_account_category, returns_next_due_date, returns_last_made_up_date,
	mortgages_num_charges, mortgages_num_outstanding, mortgages_num_part_satisfied,
	mortgages_num_satisfied, sic_code_1, sic_code_2, sic_code_3, sic_code_4,
	limited_partnerships_num_gen_partners, limited_partnerships_num_lim_partners,
	conf_stmt_next_due_date, conf_stmt_last_made_up_date
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

func (importer *companyDataImporter) insertBatch(batch []*csvRow, lastLineNum int) error {
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

	stmt, err := tx.Prepare(insertCompanyDataSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, row := range batch {
		catID, err := importer.getOrCreateRefID(tx, importer.companyCategories, "company_categories", "code", row.companyCategory)
		if err != nil {
			return err
		}
		statID, err := importer.getOrCreateRefID(tx, importer.companyStatuses, "company_statuses", "code", row.companyStatus)
		if err != nil {
			return err
		}
		countryOriginID, err := importer.getOrCreateRefID(tx, importer.countryOrigins, "country_origins", "name", row.countryOfOrigin)
		if err != nil {
			return err
		}
		accCatID, err := importer.getOrCreateRefID(tx, importer.accountsCategories, "accounts_categories", "code", row.accountsAccCategory)
		if err != nil {
			return err
		}
		countryID, err := importer.getOrCreateRefID(tx, importer.countries, "countries", "name", row.regAddressCountry)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			row.companyName, row.companyNumber, row.regAddressCareOf, row.regAddressPOBox,
			row.regAddressLine1, row.regAddressLine2, row.regAddressPostTown,
			row.regAddressCounty, countryID, row.regAddressPostCode,
			catID, statID, countryOriginID,
			row.dissolutionDate, row.incorporationDate,
			row.accountsRefDay, row.accountsRefMonth,
			row.accountsNextDueDate, row.accountsLastMadeUpDate,
			accCatID,
			row.returnsNextDueDate, row.returnsLastMadeUpDate,
			parseNullableInt(row.mortgagesCharges), parseNullableInt(row.mortgagesOutstanding),
			parseNullableInt(row.mortgagesPartSatisfied), parseNullableInt(row.mortgagesSatisfied),
			row.sicCode1, row.sicCode2, row.sicCode3, row.sicCode4,
			parseNullableInt(row.lpGenPartners), parseNullableInt(row.lpLimPartners),
			row.confStmtNextDueDate, row.confStmtLastMadeUpDate,
		)
		if err != nil {
			return fmt.Errorf("failed to execute row insert: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("Inserted records", "lastLineNum", lastLineNum)
	return nil
}

// parseNullableInt converts a string to *int, returning nil for empty strings.
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

func (importer *companyDataImporter) logElapsed(startTime time.Time, totalRows int) {
	elapsed := time.Since(startTime)
	slog.Info("Import completed",
		"totalRows", totalRows,
		"elapsed", elapsed.String(),
		"rowsPerSecond", float64(totalRows)/elapsed.Seconds(),
	)
}
