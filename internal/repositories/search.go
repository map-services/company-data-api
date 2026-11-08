package repositories

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/map-services/company-data-api/internal"
	"github.com/map-services/company-data-api/internal/models"
)

const (
	LEFT = iota
	BOTTOM
	RIGHT
	TOP
)

type SearchRepository interface {
	Find(bbox []float64, processRow func(cd *models.CompanyDataWithLocation)) error
	LastUpdated() *time.Time
}

type SqliteDbRepository struct {
	findStmt    *sql.Stmt
	lastUpdated atomic.Value

	// Reference data maps for expanding integer keys to text
	sicCodes           map[string]string // code -> description
	companyCategories  map[int]string    // id -> code
	companyStatuses    map[int]string    // id -> code
	accountsCategories map[int]string    // id -> code
	countries          map[int]string    // id -> name
	countryOrigins     map[int]string    // id -> name
}

func NewSqliteDbRepository(db *sql.DB) (SearchRepository, error) {
	findStmt, err := prepareStatement(db)
	if err != nil {
		return nil, fmt.Errorf("error preparing statement: %w", err)
	}

	repo := &SqliteDbRepository{findStmt: findStmt}

	// Load reference data maps
	if err := loadReferenceData(db, repo); err != nil {
		return nil, fmt.Errorf("failed to load reference data: %w", err)
	}

	go func() {
		lastUpdated, err := getLastUpdated(db)
		if err != nil {
			slog.Error("failed to obtain last updated date", "error", err)
		}
		repo.lastUpdated.Store(lastUpdated)
		slog.Info("Company data last updated", "lastUpdated", lastUpdated)
	}()

	return repo, nil
}

func loadReferenceData(db *sql.DB, repo *SqliteDbRepository) error {
	repo.sicCodes = make(map[string]string)
	repo.companyCategories = make(map[int]string)
	repo.companyStatuses = make(map[int]string)
	repo.accountsCategories = make(map[int]string)
	repo.countries = make(map[int]string)
	repo.countryOrigins = make(map[int]string)

	// Load SIC codes
	rows, err := db.Query("SELECT code, description FROM sic_codes")
	if err != nil {
		return fmt.Errorf("failed to query sic_codes: %w", err)
	}
	for rows.Next() {
		var code, desc string
		if err := rows.Scan(&code, &desc); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan sic_codes: %w", err)
		}
		repo.sicCodes[code] = desc
	}
	_ = rows.Close()
	slog.Info("Loaded SIC codes", "count", len(repo.sicCodes))

	// Load company categories
	rows, err = db.Query("SELECT id, code FROM company_categories")
	if err != nil {
		return fmt.Errorf("failed to query company_categories: %w", err)
	}
	for rows.Next() {
		var id int
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan company_categories: %w", err)
		}
		repo.companyCategories[id] = code
	}
	_ = rows.Close()
	slog.Info("Loaded company categories", "count", len(repo.companyCategories))

	// Load company statuses
	rows, err = db.Query("SELECT id, code FROM company_statuses")
	if err != nil {
		return fmt.Errorf("failed to query company_statuses: %w", err)
	}
	for rows.Next() {
		var id int
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan company_statuses: %w", err)
		}
		repo.companyStatuses[id] = code
	}
	_ = rows.Close()
	slog.Info("Loaded company statuses", "count", len(repo.companyStatuses))

	// Load accounts categories
	rows, err = db.Query("SELECT id, code FROM accounts_categories")
	if err != nil {
		return fmt.Errorf("failed to query accounts_categories: %w", err)
	}
	for rows.Next() {
		var id int
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan accounts_categories: %w", err)
		}
		repo.accountsCategories[id] = code
	}
	_ = rows.Close()
	slog.Info("Loaded accounts categories", "count", len(repo.accountsCategories))

	// Load countries
	rows, err = db.Query("SELECT id, name FROM countries")
	if err != nil {
		return fmt.Errorf("failed to query countries: %w", err)
	}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan countries: %w", err)
		}
		repo.countries[id] = name
	}
	_ = rows.Close()
	slog.Info("Loaded countries", "count", len(repo.countries))

	// Load country origins
	rows, err = db.Query("SELECT id, name FROM country_origins")
	if err != nil {
		return fmt.Errorf("failed to query country_origins: %w", err)
	}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan country_origins: %w", err)
		}
		repo.countryOrigins[id] = name
	}
	_ = rows.Close()
	slog.Info("Loaded country origins", "count", len(repo.countryOrigins))

	return nil
}

// expandSICCode expands a 5-digit SIC code to "code - description" format.
func (repo *SqliteDbRepository) expandSICCode(code string) string {
	if code == "" {
		return ""
	}
	if desc, ok := repo.sicCodes[code]; ok {
		return code + " - " + desc
	}
	return code
}

func prepareStatement(db *sql.DB) (*sql.Stmt, error) {
	stmt, err := db.Prepare(internal.SearchSQL)
	if err != nil {
		return nil, fmt.Errorf("error preparing SQL statement: %w", err)
	}
	return stmt, nil
}

func (repo *SqliteDbRepository) Find(bbox []float64, rowProcessor func(companyData *models.CompanyDataWithLocation)) error {

	// In bbox: [LEFT, BOTTOM, RIGHT, TOP]
	rows, err := repo.findStmt.Query(
		bbox[LEFT],   // = min easting
		bbox[RIGHT],  // = max easting
		bbox[BOTTOM], // = min northing
		bbox[TOP],    // = max northing
	)
	if err != nil {
		return fmt.Errorf("error querying database: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("error closing rows", "error", err)
		}
	}()

	for rows.Next() {
		// Scan into raw variables for integer-key fields
		var companyName, companyNumber, regAddressCareOf, regAddressPOBox string
		var regAddressAddressLine1, regAddressAddressLine2, regAddressPostTown string
		var regAddressCounty, regAddressPostCode string
		var regAddressCountryID, companyCategoryID, companyStatusID int
		var countryOriginID sql.NullInt64
		var dissolutionDate, incorporationDate sql.NullTime
		var accountsAccountRefDay, accountsAccountRefMonth int
		var accountsNextDueDate, accountsLastMadeUpDate sql.NullTime
		var accountsAccountCategoryID sql.NullInt64
		var returnsNextDueDate, returnsLastMadeUpDate sql.NullTime
		var mortgagesNumCharges, mortgagesNumOutstanding sql.NullInt64
		var mortgagesNumPartSatisfied, mortgagesNumSatisfied sql.NullInt64
		var sicCode1, sicCode2, sicCode3, sicCode4 string
		var limitedPartnershipsNumGenPartners, limitedPartnershipsNumLimPartners sql.NullInt64
		var confStmtNextDueDate, confStmtLastMadeUpDate sql.NullTime
		var easting, northing int

		if err := rows.Scan(
			&companyName, &companyNumber, &regAddressCareOf, &regAddressPOBox,
			&regAddressAddressLine1, &regAddressAddressLine2, &regAddressPostTown,
			&regAddressCounty, &regAddressCountryID, &regAddressPostCode,
			&companyCategoryID, &companyStatusID, &countryOriginID,
			&dissolutionDate, &incorporationDate,
			&accountsAccountRefDay, &accountsAccountRefMonth,
			&accountsNextDueDate, &accountsLastMadeUpDate, &accountsAccountCategoryID,
			&returnsNextDueDate, &returnsLastMadeUpDate,
			&mortgagesNumCharges, &mortgagesNumOutstanding,
			&mortgagesNumPartSatisfied, &mortgagesNumSatisfied,
			&sicCode1, &sicCode2, &sicCode3, &sicCode4,
			&limitedPartnershipsNumGenPartners, &limitedPartnershipsNumLimPartners,
			&confStmtNextDueDate, &confStmtLastMadeUpDate,
			&easting, &northing,
		); err != nil {
			return fmt.Errorf("error scanning row: %w", err)
		}

		// Build CompanyData with expanded text values
		cd := models.CompanyDataWithLocation{}
		cd.CompanyName = companyName
		cd.CompanyNumber = companyNumber
		cd.RegAddressCareOf = regAddressCareOf
		cd.RegAddressPOBox = regAddressPOBox
		cd.RegAddressAddressLine1 = regAddressAddressLine1
		cd.RegAddressAddressLine2 = regAddressAddressLine2
		cd.RegAddressPostTown = regAddressPostTown
		cd.RegAddressCounty = regAddressCounty
		cd.RegAddressCountry = repo.countries[regAddressCountryID]
		cd.RegAddressPostCode = regAddressPostCode
		cd.CompanyCategory = repo.companyCategories[companyCategoryID]
		cd.CompanyStatus = repo.companyStatuses[companyStatusID]
		if countryOriginID.Valid {
			cd.CountryOfOrigin = repo.countryOrigins[int(countryOriginID.Int64)]
		}
		if dissolutionDate.Valid {
			cd.DissolutionDate = &dissolutionDate.Time
		}
		if incorporationDate.Valid {
			cd.IncorporationDate = &incorporationDate.Time
		}
		cd.AccountsAccountRefDay = accountsAccountRefDay
		cd.AccountsAccountRefMonth = accountsAccountRefMonth
		if accountsNextDueDate.Valid {
			cd.AccountsNextDueDate = &accountsNextDueDate.Time
		}
		if accountsLastMadeUpDate.Valid {
			cd.AccountsLastMadeUpDate = &accountsLastMadeUpDate.Time
		}
		if accountsAccountCategoryID.Valid {
			cd.AccountsAccountCategory = repo.accountsCategories[int(accountsAccountCategoryID.Int64)]
		}
		if returnsNextDueDate.Valid {
			cd.ReturnsNextDueDate = &returnsNextDueDate.Time
		}
		if returnsLastMadeUpDate.Valid {
			cd.ReturnsLastMadeUpDate = &returnsLastMadeUpDate.Time
		}
		if mortgagesNumCharges.Valid {
			cd.MortgagesNumCharges = fmt.Sprintf("%d", mortgagesNumCharges.Int64)
		}
		if mortgagesNumOutstanding.Valid {
			cd.MortgagesNumOutstanding = fmt.Sprintf("%d", mortgagesNumOutstanding.Int64)
		}
		if mortgagesNumPartSatisfied.Valid {
			cd.MortgagesNumPartSatisfied = fmt.Sprintf("%d", mortgagesNumPartSatisfied.Int64)
		}
		if mortgagesNumSatisfied.Valid {
			cd.MortgagesNumSatisfied = fmt.Sprintf("%d", mortgagesNumSatisfied.Int64)
		}
		cd.SICCode1 = repo.expandSICCode(sicCode1)
		cd.SICCode2 = repo.expandSICCode(sicCode2)
		cd.SICCode3 = repo.expandSICCode(sicCode3)
		cd.SICCode4 = repo.expandSICCode(sicCode4)
		if limitedPartnershipsNumGenPartners.Valid {
			cd.LimitedPartnershipsNumGenPartners = fmt.Sprintf("%d", limitedPartnershipsNumGenPartners.Int64)
		}
		if limitedPartnershipsNumLimPartners.Valid {
			cd.LimitedPartnershipsNumLimPartners = fmt.Sprintf("%d", limitedPartnershipsNumLimPartners.Int64)
		}
		cd.URI = models.URIFromCompanyNumber(companyNumber)
		if confStmtNextDueDate.Valid {
			cd.ConfStmtNextDueDate = &confStmtNextDueDate.Time
		}
		if confStmtLastMadeUpDate.Valid {
			cd.ConfStmtLastMadeUpDate = &confStmtLastMadeUpDate.Time
		}
		cd.Easting = easting
		cd.Northing = northing

		rowProcessor(&cd)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("error during rows iteration: %w", err)
	}

	return nil
}

func (repo *SqliteDbRepository) LastUpdated() *time.Time {
	return repo.lastUpdated.Load().(*time.Time)
}

func getLastUpdated(db *sql.DB) (*time.Time, error) {
	var lastUpdateStr sql.NullString
	row := db.QueryRow(`SELECT MAX(incorporation_date) FROM company_data`)
	if err := row.Scan(&lastUpdateStr); err != nil {
		return nil, fmt.Errorf("failed to determine last update: %w", err)
	}

	if !lastUpdateStr.Valid {
		// Table is empty or all incorporation_date values are NULL.
		return nil, nil
	}

	rfc3339str := strings.Replace(lastUpdateStr.String, " ", "T", 1)
	lastUpdate, err := time.Parse(time.RFC3339, rfc3339str)
	if err != nil {
		return nil, fmt.Errorf("failed to parse last update string '%s': %w", lastUpdateStr.String, err)
	}

	return &lastUpdate, nil
}
