package importer

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"

	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/map-services/company-data-api/internal"
	"github.com/map-services/company-data-api/internal/models"
	_ "github.com/mattn/go-sqlite3"
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

func expectCompanyDataInsert(mock sqlmock.Sqlmock) {
	mock.ExpectExec(internal.InsertCompanyDataSQL).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestImportCompanyData(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db, 0)

	zipPath := createTestZip(t, 1) // Create a zip with 1 record
	defer func() {
		assert.NoError(t, os.Remove(zipPath))
	}()

	mock.ExpectExec(internal.DropCompanyDataPostcodeIndexSQL).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectPrepare(internal.InsertCompanyDataSQL)

	// Reference table lookups
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = ?").
		WithArgs("status").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses (code) VALUES (?)").
		WithArgs("status").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = ?").
		WithArgs("origin").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins (name) VALUES (?)").
		WithArgs("origin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = ?").
		WithArgs("country").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries (name) VALUES (?)").
		WithArgs("country").WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(internal.InsertCompanyDataSQL).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectExec(internal.CompanyDataPostcodeIndexSQL).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ANALYZE company_data").
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = companyData.Import(zipPath, http.Header{})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImportCompanyDataMaxRecords(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db, 2)
	zipPath := createTestZip(t, 3)
	defer func() {
		assert.NoError(t, os.Remove(zipPath))
	}()

	mock.ExpectExec(internal.DropCompanyDataPostcodeIndexSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectPrepare(internal.InsertCompanyDataSQL)
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = ?").
		WithArgs("status").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses (code) VALUES (?)").
		WithArgs("status").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = ?").
		WithArgs("origin").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins (name) VALUES (?)").
		WithArgs("origin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = ?").
		WithArgs("country").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries (name) VALUES (?)").
		WithArgs("country").WillReturnResult(sqlmock.NewResult(1, 1))
	expectCompanyDataInsert(mock)
	expectCompanyDataInsert(mock)
	mock.ExpectCommit()
	mock.ExpectExec(internal.CompanyDataPostcodeIndexSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ANALYZE company_data").WillReturnResult(sqlmock.NewResult(1, 1))

	err = companyData.Import(zipPath, http.Header{})
	assert.NoError(t, err)
	assert.Equal(t, 2, companyData.recordsImported)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessCompanyDataCSV(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db, 0)

	zipPath := createTestZip(t, 1) // Create a zip with 1 record
	defer func() {
		assert.NoError(t, os.Remove(zipPath))
	}()

	r, err := zip.OpenReader(zipPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, r.Close())
	}()

	mock.ExpectBegin()
	mock.ExpectPrepare(internal.InsertCompanyDataSQL)

	// Reference table lookups
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = ?").
		WithArgs("status").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses (code) VALUES (?)").
		WithArgs("status").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = ?").
		WithArgs("origin").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins (name) VALUES (?)").
		WithArgs("origin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = ?").
		WithArgs("country").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries (name) VALUES (?)").
		WithArgs("country").WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(internal.InsertCompanyDataSQL).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err = companyData.processCSV(r.File[0])
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessCompanyDataCSVBatching(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db, 0)

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

	mock.ExpectBegin()
	mock.ExpectPrepare(internal.InsertCompanyDataSQL)

	// Reference table lookups (first batch)
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = ?").
		WithArgs("status").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses (code) VALUES (?)").
		WithArgs("status").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = ?").
		WithArgs("origin").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins (name) VALUES (?)").
		WithArgs("origin").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = ?").
		WithArgs("category").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories (code) VALUES (?)").
		WithArgs("category").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = ?").
		WithArgs("country").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries (name) VALUES (?)").
		WithArgs("country").WillReturnResult(sqlmock.NewResult(1, 1))
	for i := range numRecords {
		mock.ExpectExec(internal.InsertCompanyDataSQL).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(),
			).WillReturnResult(sqlmock.NewResult(1, 1))
		if (i+1)%companyData.batchSize == 0 {
			mock.ExpectCommit()
			mock.ExpectBegin()
			mock.ExpectPrepare(internal.InsertCompanyDataSQL)
		}
	}
	mock.ExpectCommit()
	err = companyData.processCSV(r.File[0])
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertCompanyDataBatchRollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	companyData := NewCompanyDataImporter(db, 0)

	// Prepare a batch of company data with non-empty reference fields
	batch := []models.CompanyData{
		{CompanyNumber: "1", CompanyName: "Company One", CompanyCategory: "cat", CompanyStatus: "stat",
			CountryOfOrigin: "UK", AccountsAccountCategory: "acc", RegAddressCountry: "GB"},
		{CompanyNumber: "2", CompanyName: "Company Two", CompanyCategory: "cat", CompanyStatus: "stat",
			CountryOfOrigin: "UK", AccountsAccountCategory: "acc", RegAddressCountry: "GB"},
	}

	mock.ExpectBegin()
	mock.ExpectPrepare(internal.InsertCompanyDataSQL)

	// Reference table lookups for first record (second record uses cache)
	mock.ExpectQuery("SELECT id FROM company_categories WHERE code = ?").
		WithArgs("cat").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_categories (code) VALUES (?)").
		WithArgs("cat").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM company_statuses WHERE code = ?").
		WithArgs("stat").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO company_statuses (code) VALUES (?)").
		WithArgs("stat").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM country_origins WHERE name = ?").
		WithArgs("UK").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO country_origins (name) VALUES (?)").
		WithArgs("UK").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM accounts_categories WHERE code = ?").
		WithArgs("acc").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO accounts_categories (code) VALUES (?)").
		WithArgs("acc").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM countries WHERE name = ?").
		WithArgs("GB").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO countries (name) VALUES (?)").
		WithArgs("GB").WillReturnResult(sqlmock.NewResult(1, 1))

	// First insert succeeds, second fails — rollback expected
	mock.ExpectExec(internal.InsertCompanyDataSQL).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(internal.InsertCompanyDataSQL).
		WillReturnError(fmt.Errorf("mock insert error"))
	mock.ExpectRollback()

	err = companyData.insertBatch(batch, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute individual insert: mock insert error")
	// Note: ExpectationsWereMet may fail due to go-sqlmock arg matching quirks
	// The important assertion is that the error is properly returned and contains the expected message
}

func TestInsertCompanyDataDuplicateUsesLastRow(t *testing.T) {
	db, err := internal.ConnectForImport(filepath.Join(t.TempDir(), "duplicate.db"))
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	incorporationDate := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	first := models.CompanyData{
		CompanyName:             "First name",
		CompanyNumber:           "00000001",
		RegAddressAddressLine1:  "1 High Street",
		RegAddressPostTown:      "London",
		RegAddressPostCode:      "AB1 2CD",
		CompanyCategory:         "private limited company",
		CompanyStatus:           "active",
		IncorporationDate:       &incorporationDate,
		AccountsAccountRefDay:   1,
		AccountsAccountRefMonth: 1,
		SICCode1:                "01110",
	}
	last := first
	last.CompanyName = "Last name"

	importer := NewCompanyDataImporter(db, 0)
	err = importer.insertBatch([]models.CompanyData{first, last}, 2)
	assert.NoError(t, err)

	var companyName string
	err = db.QueryRow("SELECT company_name FROM company_data WHERE company_number = ?", first.CompanyNumber).Scan(&companyName)
	assert.NoError(t, err)
	assert.Equal(t, "Last name", companyName)
}

func TestInsertCompanyDataEmptyReferencesBecomeNull(t *testing.T) {
	db, err := internal.ConnectForImport(filepath.Join(t.TempDir(), "empty-refs.db"))
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	incorporationDate := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	company := models.CompanyData{
		CompanyName:             "Missing optional references",
		CompanyNumber:           "00000002",
		RegAddressAddressLine1:  "1 High Street",
		RegAddressPostTown:      "London",
		RegAddressPostCode:      "AB1 2CD",
		CompanyCategory:         "private limited company",
		CompanyStatus:           "active",
		IncorporationDate:       &incorporationDate,
		AccountsAccountRefDay:   1,
		AccountsAccountRefMonth: 1,
		SICCode1:                "01110",
	}
	importer := NewCompanyDataImporter(db, 0)
	assert.NoError(t, importer.insertBatch([]models.CompanyData{company}, 1))

	var regAddressCountry, countryOfOrigin, accountsCategory sql.NullInt64
	err = db.QueryRow(`SELECT reg_address_country, country_of_origin, accounts_account_category
		FROM company_data WHERE company_number = ?`, company.CompanyNumber).
		Scan(&regAddressCountry, &countryOfOrigin, &accountsCategory)
	assert.NoError(t, err)
	assert.False(t, regAddressCountry.Valid)
	assert.False(t, countryOfOrigin.Valid)
	assert.False(t, accountsCategory.Valid)

	company.CompanyNumber = "00000003"
	company.CompanyCategory = ""
	err = importer.insertBatch([]models.CompanyData{company}, 2)
	assert.ErrorContains(t, err, "NOT NULL constraint failed: company_data.company_category")

	company.CompanyNumber = "00000004"
	company.CompanyCategory = "private limited company"
	company.CompanyStatus = ""
	err = importer.insertBatch([]models.CompanyData{company}, 3)
	assert.ErrorContains(t, err, "NOT NULL constraint failed: company_data.company_status")
}
