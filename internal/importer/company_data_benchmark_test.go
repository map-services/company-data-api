package importer

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/map-services/company-data-api/internal"
	_ "github.com/mattn/go-sqlite3"
)

func writeCompanyDataBenchmarkZip(b *testing.B, zipPath string, rowCount int) {
	b.Helper()

	file, err := os.Create(zipPath)
	if err != nil {
		b.Fatal(err)
	}
	zipWriter := zip.NewWriter(file)
	csvFile, err := zipWriter.Create("BasicCompanyData.csv")
	if err != nil {
		b.Fatal(err)
	}
	csvWriter := csv.NewWriter(csvFile)

	headers := make([]string, 55)
	copy(headers, []string{
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
	})
	headers[53] = "ConfStmtNextDueDate"
	headers[54] = "ConfStmtLastMadeUpDate"
	if err := csvWriter.Write(headers); err != nil {
		b.Fatal(err)
	}

	rng := rand.New(rand.NewSource(1))
	multiplier := 104729
	for greatestCommonDivisor(multiplier, rowCount) != 1 {
		multiplier++
	}
	offset := rng.Intn(rowCount)
	postTowns := []string{"LONDON", "BIRMINGHAM", "LEEDS", "GLASGOW", "BRISTOL", "CARDIFF", "MANCHESTER", "LIVERPOOL"}
	counties := []string{"GREATER LONDON", "WEST MIDLANDS", "WEST YORKSHIRE", "KENT", "ESSEX", "SURREY"}
	countries := []string{"England", "Wales", "Scotland", "Northern Ireland", "United Kingdom", "Jersey"}
	origins := []string{"United Kingdom", "England", "Scotland", "Wales", "Ireland", "France", "United States"}
	accountCategories := []string{"micro entity", "dormant", "total exemption full", "small", "full"}
	sicCodes := []string{"01110", "62020", "68100", "70229", "82990", "47190"}
	row := make([]string, 55)
	for index := 0; index < rowCount; index++ {
		companyID := (int64(index)*int64(multiplier) + int64(offset)) % int64(rowCount)
		row[0] = fmt.Sprintf("SYNTHETIC COMPANY %08d", companyID)
		row[1] = fmt.Sprintf("%08d", companyID)
		row[2] = ""
		row[3] = ""
		row[4] = fmt.Sprintf("%d HIGH STREET", rng.Intn(999)+1)
		row[5] = ""
		row[6] = postTowns[rng.Intn(len(postTowns))]
		row[7] = counties[rng.Intn(len(counties))]
		row[8] = weightedCountry(rng, countries)
		row[9] = fmt.Sprintf("AB%d %dCD", rng.Intn(25000), rng.Intn(10))
		row[10] = weightedCategory(rng)
		row[11] = weightedStatus(rng)
		row[12] = origins[rng.Intn(len(origins))]
		row[13] = ""
		row[14] = "01/01/2020"
		row[15] = "1"
		row[16] = "1"
		row[17] = ""
		row[18] = ""
		row[19] = accountCategories[rng.Intn(len(accountCategories))]
		row[20] = ""
		row[21] = ""
		for field := 22; field <= 25; field++ {
			row[field] = nullableBenchmarkNumber(rng)
		}
		for field := 26; field <= 29; field++ {
			row[field] = sicCodes[rng.Intn(len(sicCodes))]
		}
		row[30] = nullableBenchmarkNumber(rng)
		row[31] = nullableBenchmarkNumber(rng)
		row[32] = "https://example.test/company/" + row[1]
		if err := csvWriter.Write(row); err != nil {
			b.Fatal(err)
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		b.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}
}

func greatestCommonDivisor(left, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	return left
}

func nullableBenchmarkNumber(rng *rand.Rand) string {
	if rng.Intn(4) == 0 {
		return ""
	}
	return strconv.Itoa(rng.Intn(100))
}

func weightedCategory(rng *rand.Rand) string {
	if rng.Intn(100) < 83 {
		return "Private limited by shares"
	}
	return []string{"Private limited by guarantee", "Public limited company", "Limited partnership", "Private unlimited company"}[rng.Intn(4)]
}

func weightedStatus(rng *rand.Rand) string {
	if rng.Intn(100) < 82 {
		return "Active"
	}
	return []string{"Dissolved", "Liquidation", "Administration", "Receivership"}[rng.Intn(4)]
}

func weightedCountry(rng *rand.Rand, countries []string) string {
	if rng.Intn(100) < 88 {
		return "England"
	}
	return countries[rng.Intn(len(countries))]
}

func BenchmarkCompanyDataImport(b *testing.B) {
	rowCount := 500_000
	if value := os.Getenv("COMPANY_IMPORT_BENCH_ROWS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			b.Fatalf("invalid COMPANY_IMPORT_BENCH_ROWS %q", value)
		}
		rowCount = parsed
	}

	zipPath := filepath.Join(b.TempDir(), "companies.zip")
	writeCompanyDataBenchmarkZip(b, zipPath, rowCount)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		dbPath := filepath.Join(b.TempDir(), fmt.Sprintf("run-%d.db", iteration))
		db, err := internal.Connect(dbPath)
		if err != nil {
			b.Fatal(err)
		}
		if err := NewCompanyDataImporter(db, 0).Import(zipPath, http.Header{}); err != nil {
			b.Fatal(err)
		}
		if err := db.Close(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(rowCount)/b.Elapsed().Seconds(), "rows/s")
}
