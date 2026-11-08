package importer

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"

	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/map-services/company-data-api/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestFromCompanyDataCSV(t *testing.T) {
	headers := []string{
		"CompanyName", "CompanyNumber", "RegAddress.CareOf", "RegAddress.POBox",
		"RegAddress.AddressLine1", "RegAddress.AddressLine2", "RegAddress.PostTown",
		"RegAddress.County", "RegAddress.Country", "RegAddress.PostCode",
		"CompanyCategory", "CompanyStatus", "CountryOfOrigin", "DissolutionDate",
		"IncorporationDate", "Accounts.AccountRefDay", "Accounts.AccountRefMonth",
		"Accounts.NextDueDate", "Accounts.LastMadeUpDate", "Accounts.AccountCategory",
		"Returns.NextDueDate", "Returns.LastMadeUpDate", "Mortgages.NumCharges",
		"Mortgages.NumOutstanding", "Mortgages.NumPartSatisfied", "Mortgages.NumSatisfied",
		"SICCode.SicText_1", "SICCode.SicText_2", "SICCode.SicText_3", "SICCode.SicText_4",
		"LimitedPartnerships.NumGenPartners", "LimitedPartnerships.NumLimPartners", "URI",
		"ConfStmtNextDueDate", "ConfStmtLastMadeUpDate",
	}

	record := make([]string, 55)
	record[0] = "company"
	record[1] = "123456"
	record[4] = "address1"
	record[5] = "address2"
	record[6] = "posttown"
	record[7] = "county"
	record[8] = "country"
	record[9] = "postcode"
	record[10] = "category"
	record[11] = "status"
	record[12] = "origin"
	record[13] = "01/01/2025"
	record[14] = "01/01/2024"
	record[15] = "1"
	record[16] = "1"
	record[17] = "01/01/2025"
	record[18] = "01/01/2024"
	record[19] = "category"
	record[20] = "01/01/2025"
	record[21] = "01/01/2024"
	record[22] = "1"
	record[23] = "1"
	record[24] = "1"
	record[25] = "1"
	record[26] = "sic1"
	record[27] = "sic2"
	record[28] = "sic3"
	record[29] = "sic4"
	record[30] = "1"
	record[31] = "1"
	record[32] = "uri"
	record[53] = "01/01/2025"
	record[54] = "01/01/2024"

	dissolutionDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	incorporationDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	accountsNextDueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	accountsLastMadeUpDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	returnsNextDueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	returnsLastMadeUpDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	confStmtNextDueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	confStmtLastMadeUpDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	expected := &models.CompanyData{
		CompanyName:                       "company",
		CompanyNumber:                     "123456",
		RegAddressAddressLine1:            "address1",
		RegAddressAddressLine2:            "address2",
		RegAddressPostTown:                "posttown",
		RegAddressCounty:                  "county",
		RegAddressCountry:                 "country",
		RegAddressPostCode:                "postcode",
		CompanyCategory:                   "category",
		CompanyStatus:                     "status",
		CountryOfOrigin:                   "origin",
		DissolutionDate:                   &dissolutionDate,
		IncorporationDate:                 &incorporationDate,
		AccountsAccountRefDay:             1,
		AccountsAccountRefMonth:           1,
		AccountsNextDueDate:               &accountsNextDueDate,
		AccountsLastMadeUpDate:            &accountsLastMadeUpDate,
		AccountsAccountCategory:           "category",
		ReturnsNextDueDate:                &returnsNextDueDate,
		ReturnsLastMadeUpDate:             &returnsLastMadeUpDate,
		MortgagesNumCharges:               "1",
		MortgagesNumOutstanding:           "1",
		MortgagesNumPartSatisfied:         "1",
		MortgagesNumSatisfied:             "1",
		SICCode1:                          "sic1",
		SICCode2:                          "sic2",
		SICCode3:                          "sic3",
		SICCode4:                          "sic4",
		LimitedPartnershipsNumGenPartners: "1",
		LimitedPartnershipsNumLimPartners: "1",
		ConfStmtNextDueDate:               &confStmtNextDueDate,
		ConfStmtLastMadeUpDate:            &confStmtLastMadeUpDate,
	}

	actual, err := fromCompanyDataCSV(record, headers)

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestFromCompanyDataCSVShortRecord(t *testing.T) {
	headers := []string{"CompanyName", "CompanyNumber"}
	record := []string{"company"}

	_, err := fromCompanyDataCSV(record, headers)

	assert.Error(t, err)
}

func TestFromCompanyDataCSVInvalidDate(t *testing.T) {
	headers := []string{
		"CompanyName", "CompanyNumber", "RegAddress.CareOf", "RegAddress.POBox",
		"RegAddress.AddressLine1", "RegAddress.AddressLine2", "RegAddress.PostTown",
		"RegAddress.County", "RegAddress.Country", "RegAddress.PostCode",
		"CompanyCategory", "CompanyStatus", "CountryOfOrigin", "DissolutionDate",
	}
	record := make([]string, 55)
	record[13] = "invalid-date"

	_, err := fromCompanyDataCSV(record, headers)

	assert.Error(t, err)
}

func TestCompanyDataToTuple(t *testing.T) {
	dissolutionDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	incorporationDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	accountsNextDueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	accountsLastMadeUpDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	returnsNextDueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	returnsLastMadeUpDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	confStmtNextDueDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	confStmtLastMadeUpDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	companyData := models.CompanyData{
		CompanyName:               "company",
		CompanyNumber:             "123456",
		RegAddressAddressLine1:    "address1",
		RegAddressAddressLine2:    "address2",
		RegAddressPostTown:        "posttown",
		RegAddressCounty:          "county",
		RegAddressCountry:         "country",
		RegAddressPostCode:        "postcode",
		CompanyCategory:           "category",
		CompanyStatus:             "status",
		CountryOfOrigin:           "origin",
		DissolutionDate:           &dissolutionDate,
		IncorporationDate:         &incorporationDate,
		AccountsAccountRefDay:     1,
		AccountsAccountRefMonth:   1,
		AccountsNextDueDate:       &accountsNextDueDate,
		AccountsLastMadeUpDate:    &accountsLastMadeUpDate,
		AccountsAccountCategory:   "category",
		ReturnsNextDueDate:        &returnsNextDueDate,
		ReturnsLastMadeUpDate:     &returnsLastMadeUpDate,
		MortgagesNumCharges:       "1",
		MortgagesNumOutstanding:   "1",
		MortgagesNumPartSatisfied: "1",
		MortgagesNumSatisfied:     "1",
		SICCode1:                  "sic1",
		SICCode2:                  "sic2",
		SICCode3:                  "sic3", SICCode4: "sic4",
		LimitedPartnershipsNumGenPartners: "1",
		LimitedPartnershipsNumLimPartners: "1",
		ConfStmtNextDueDate:               &confStmtNextDueDate,
		ConfStmtLastMadeUpDate:            &confStmtLastMadeUpDate,
	}

	expected := []any{
		"company", "123456", "", "", "address1", "address2", "posttown", "county",
		"country", "postcode", "category", "status", "origin",
		&dissolutionDate,
		&incorporationDate,
		1, 1, &accountsNextDueDate,
		&accountsLastMadeUpDate,
		"category", &returnsNextDueDate,
		&returnsLastMadeUpDate,
		"1", "1", "1", "1", "sic1", "sic2", "sic3", "sic4", "1", "1",
		&confStmtNextDueDate,
		&confStmtLastMadeUpDate,
	}

	actual := companyDataToTuple(companyData)

	assert.Equal(t, expected, actual)
}

// createTestZip creates a temporary zip file with a single CSV file for testing
func createTestZip(t *testing.T, numRecords int) string {
	t.Helper()
	tempFile, err := os.CreateTemp("", "test-*.zip")
	assert.NoError(t, err)
	defer func() {
		if err := tempFile.Close(); err != nil {
			t.Fatalf("Failed to close temporary file: %v", err)
		}
	}()

	zipWriter := zip.NewWriter(tempFile)
	defer func() {
		assert.NoError(t, zipWriter.Close())
	}()

	f, err := zipWriter.Create("test.csv")
	assert.NoError(t, err)

	csvWriter := csv.NewWriter(f)
	defer csvWriter.Flush()

	headers := make([]string, 55)
	headers[0] = "CompanyName"
	assert.NoError(t, csvWriter.Write(headers))

	for i := range numRecords {
		record := make([]string, 55)
		record[0] = fmt.Sprintf("company%d", i)
		record[1] = fmt.Sprintf("123456%d", i)
		record[4] = "address1"
		record[5] = "address2"
		record[6] = "posttown"
		record[7] = "county"
		record[8] = "country"
		record[9] = "postcode"
		record[10] = "category"
		record[11] = "status"
		record[12] = "origin"
		record[13] = "01/01/2025"
		record[14] = "01/01/2024"
		record[15] = "1"
		record[16] = "1"
		record[17] = "01/01/2025"
		record[18] = "01/01/2024"
		record[19] = "category"
		record[20] = "01/01/2025"
		record[21] = "01/01/2024"
		record[22] = "1"
		record[23] = "1"
		record[24] = "1"
		record[25] = "1"
		record[26] = "sic1"
		record[27] = "sic2"
		record[28] = "sic3"
		record[29] = "sic4"
		record[30] = "1"
		record[31] = "1"
		record[32] = "uri"
		record[53] = "01/01/2025"
		record[54] = "01/01/2024"
		assert.NoError(t, csvWriter.Write(record))
	}

	return tempFile.Name()
}

func TestImportCompanyData(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db)

	zipPath := createTestZip(t, 1) // Create a zip with 1 record
	defer func() {
		assert.NoError(t, os.Remove(zipPath))
	}()

	// Pre-populate reference caches
	mock.ExpectQuery("SELECT id, code FROM company_categories").WillReturnRows(sqlmock.NewRows([]string{"id", "code"}))
	mock.ExpectQuery("SELECT id, code FROM company_statuses").WillReturnRows(sqlmock.NewRows([]string{"id", "code"}))
	mock.ExpectQuery("SELECT id, code FROM accounts_categories").WillReturnRows(sqlmock.NewRows([]string{"id", "code"}))
	mock.ExpectQuery("SELECT id, name FROM countries").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectQuery("SELECT id, name FROM country_origins").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	mock.ExpectBegin()

	// Prepared statement insert (Prepare happens before reference lookups)
	mock.ExpectPrepare("INSERT OR REPLACE INTO company_data")
	// Reference table lookups (not in cache, so INSERT)
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = \\?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories \\(code\\) VALUES \\(\\?\\)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = \\?").
		WithArgs("status").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses \\(code\\) VALUES \\(\\?\\)").
		WithArgs("status").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = \\?").
		WithArgs("origin").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins \\(name\\) VALUES \\(\\?\\)").
		WithArgs("origin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = \\?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories \\(code\\) VALUES \\(\\?\\)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = \\?").
		WithArgs("country").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries \\(name\\) VALUES \\(\\?\\)").
		WithArgs("country").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT OR REPLACE INTO company_data").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec("ANALYZE company_data").
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = companyData.Import(zipPath, http.Header{})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessCompanyDataCSV(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db)

	zipPath := createTestZip(t, 1) // Create a zip with 1 record
	defer func() {
		assert.NoError(t, os.Remove(zipPath))
	}()

	r, err := zip.OpenReader(zipPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, r.Close())
	}()

	// Pre-populate reference caches
	mock.ExpectQuery("SELECT id, code FROM company_categories").WillReturnRows(sqlmock.NewRows([]string{"id", "code"}))
	mock.ExpectQuery("SELECT id, code FROM company_statuses").WillReturnRows(sqlmock.NewRows([]string{"id", "code"}))
	mock.ExpectQuery("SELECT id, code FROM accounts_categories").WillReturnRows(sqlmock.NewRows([]string{"id", "code"}))
	mock.ExpectQuery("SELECT id, name FROM countries").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectQuery("SELECT id, name FROM country_origins").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	mock.ExpectBegin()
	// Prepared statement insert (Prepare happens before reference lookups)
	mock.ExpectPrepare("INSERT OR REPLACE INTO company_data")
	// Reference table lookups (not in cache, so INSERT)
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = \\?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories \\(code\\) VALUES \\(\\?\\)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = \\?").
		WithArgs("status").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses \\(code\\) VALUES \\(\\?\\)").
		WithArgs("status").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = \\?").
		WithArgs("origin").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins \\(name\\) VALUES \\(\\?\\)").
		WithArgs("origin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = \\?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories \\(code\\) VALUES \\(\\?\\)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = \\?").
		WithArgs("country").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries \\(name\\) VALUES \\(\\?\\)").
		WithArgs("country").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT OR REPLACE INTO company_data").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err = companyData.processCSV(r.File[0])
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessCompanyDataCSVBatching(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db)

	// Use a small batch size for testing
	companyData.batchSize = 2
	numRecords := companyData.batchSize*2 + 1 // More than two batches
	zipPath := createTestZip(t, numRecords)
	defer func() {
		assert.NoError(t, os.Remove(zipPath))
	}()

	r, err := zip.OpenReader(zipPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, r.Close())
	}()

	// Pre-populate reference caches with cached values so no INSERT needed
	rows := sqlmock.NewRows([]string{"id", "code"}).AddRow(1, "category")
	mock.ExpectQuery("SELECT id, code FROM company_categories").WillReturnRows(rows)
	rows = sqlmock.NewRows([]string{"id", "code"}).AddRow(1, "status")
	mock.ExpectQuery("SELECT id, code FROM company_statuses").WillReturnRows(rows)
	rows = sqlmock.NewRows([]string{"id", "code"}).AddRow(1, "category")
	mock.ExpectQuery("SELECT id, code FROM accounts_categories").WillReturnRows(rows)
	rows = sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "country")
	mock.ExpectQuery("SELECT id, name FROM countries").WillReturnRows(rows)
	rows = sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "origin")
	mock.ExpectQuery("SELECT id, name FROM country_origins").WillReturnRows(rows)

	// 3 batches: 2, 2, 1
	batchSizes := []int{2, 2, 1}
	for _, size := range batchSizes {
		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT OR REPLACE INTO company_data")
		for range size {
			mock.ExpectExec("INSERT OR REPLACE INTO company_data").WillReturnResult(sqlmock.NewResult(1, 1))
		}
		mock.ExpectCommit()
	}
	err = companyData.processCSV(r.File[0])
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertCompanyDataBatchRollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db)

	// Prepare a batch of csvRow with non-empty reference fields
	batch := []*csvRow{
		{companyNumber: "1", companyName: "Company One", companyCategory: "cat", companyStatus: "stat",
			countryOfOrigin: "UK", accountsAccCategory: "acc", regAddressCountry: "GB"},
		{companyNumber: "2", companyName: "Company Two", companyCategory: "cat", companyStatus: "stat",
			countryOfOrigin: "UK", accountsAccCategory: "acc", regAddressCountry: "GB"},
	}

	mock.ExpectBegin()

	// Prepared statement insert (Prepare happens before reference lookups)
	mock.ExpectPrepare("INSERT OR REPLACE INTO company_data")
	// Reference table lookups for first record (second record uses cache)
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = \\?").
		WithArgs("cat").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories \\(code\\) VALUES \\(\\?\\)").
		WithArgs("cat").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = \\?").
		WithArgs("stat").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses \\(code\\) VALUES \\(\\?\\)").
		WithArgs("stat").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = \\?").
		WithArgs("UK").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins \\(name\\) VALUES \\(\\?\\)").
		WithArgs("UK").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = \\?").
		WithArgs("acc").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories \\(code\\) VALUES \\(\\?\\)").
		WithArgs("acc").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = \\?").
		WithArgs("GB").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries \\(name\\) VALUES \\(\\?\\)").
		WithArgs("GB").WillReturnResult(sqlmock.NewResult(1, 1))
	// First row Exec fails — rollback expected
	mock.ExpectExec("INSERT OR REPLACE INTO company_data").
		WillReturnError(fmt.Errorf("mock insert error"))
	mock.ExpectRollback()

	err = companyData.insertBatch(batch, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute row insert: mock insert error")
}
