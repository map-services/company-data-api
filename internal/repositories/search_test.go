package repositories

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/map-services/company-data-api/internal"
	"github.com/map-services/company-data-api/internal/models"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestFindHandlesNullRegistrationCountry(t *testing.T) {
	db, err := internal.ConnectForImport(filepath.Join(t.TempDir(), "search.db"))
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	_, err = db.Exec("INSERT INTO company_categories (id, code) VALUES (1, 'private')")
	assert.NoError(t, err)
	_, err = db.Exec("INSERT INTO company_statuses (id, code) VALUES (1, 'active')")
	assert.NoError(t, err)
	_, err = db.Exec(`INSERT INTO company_data (
		company_name, company_number, reg_address_care_of, reg_address_po_box,
		reg_address_address_line_1, reg_address_address_line_2, reg_address_post_town,
		reg_address_county, reg_address_post_code, company_category, company_status,
		incorporation_date, accounts_account_ref_day, accounts_account_ref_month,
		sic_code_1, sic_code_2, sic_code_3, sic_code_4
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Company", "00000001", "", "", "1 High Street", "", "London", "", "AB1 2CD", 1, 1,
		time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), 1, 1, "01110", "", "", "")
	assert.NoError(t, err)
	_, err = db.Exec("INSERT INTO code_point (post_code, easting, northing) VALUES (?, ?, ?)", "AB1 2CD", 100, 200)
	assert.NoError(t, err)

	repository, err := NewSqliteDbRepository(db)
	assert.NoError(t, err)
	var result *models.CompanyDataWithLocation
	err = repository.Find([]float64{0, 0, 300, 300}, func(company *models.CompanyDataWithLocation) {
		result = company
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected a search result")
	}
	assert.Empty(t, result.RegAddressCountry)
}
