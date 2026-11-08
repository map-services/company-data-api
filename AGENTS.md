# Project Overview

This project is a Go-based REST API for querying UK company data. It provides a fast and efficient way to search for companies within a specific geographic bounding box. The API is built using the Gin framework and uses a SQLite database to store the company and postcode data.

The data is imported from three main sources:

*   **Companies House:** Provides basic company data for UK companies.
*   **Ordnance Survey CodePoint Open:** Provides postcode data for the UK.
*   **UK Government SIC07:** Provides the official Standard Industrial Classification codes (731 codes) as reference data.

The API exposes several endpoints for searching and retrieving company data, as well as for health checks and monitoring.

## Database Schema

The SQLite database uses reference tables to minimize storage size:

*   **`sic_codes`** — SIC code to description mapping (populated from official SIC07 CSV).
*   **`company_categories`**, **`company_statuses`**, **`accounts_categories`**, **`countries`**, **`country_origins`** — lookup tables for low-cardinality text fields.
*   **`company_data`** — main table storing integer foreign keys to reference tables, 5-digit SIC codes only (descriptions expanded at query time), and nullable numeric fields.

The `uri` field is derived at response time from `company_number` rather than stored.

## Data Import

Before running the API, import the data in the following order:

1.  **SIC codes** (reference data, ~731 codes):

    ```sh
    ./company-data import-sic-codes --csv-file https://assets.publishing.service.gov.uk/media/5a7f8639e5274a2e87db65e1/SIC07_CH_condensed_list_en.csv
    ```

2.  **Companies House data** (company records, ~5M+ rows):

    ```sh
    ./company-data import-companies-house --zip-file <path_or_url>
    ```

3.  **CodePoint Open data** (postcodes, ~2.9M rows):

    ```sh
    ./company-data import-code-point --zip-file https://api.os.uk/downloads/v1/products/CodePointOpen/downloads?area=GB&format=CSV&redirect
    ```

## Building and Running

The project can be built and run using either Go commands or Docker.

### Go

To build and run the project using Go, you will need to have Go installed on your system.

1.  **Build the application:**

    ```sh
    golangci-lint run
    go build -tags=jsoniter -o company-data .
    ```

2.  **Run the API server:**

    ```sh
    ./company-data api-server --db ./data/companies_data.db --port 8080
    ```

### Docker

To build and run the project using Docker, you will need to have Docker installed on your system.

1.  **Build the Docker image:**

    ```sh
    docker build -t company-data-api .
    ```

2.  **Run the Docker container:**

    ```sh
    docker run -p 8080:8080 -v $PWD/data:/app/data company-data-api api-server
    ```

## Development Conventions

*   **API:** The API is built using the Gin framework.
*   **Database:** The project uses a SQLite database to store the company and postcode data.
*   **Data Import:** The data is imported from CSV files using custom Go scripts.
*   **Dependencies:** The project uses Go modules to manage dependencies.
*   **Linting:** The project uses `golangci-lint` for linting.
*   **Testing:** The project uses `gotestsum` for testing and generates JUnit reports.
*   **CI/CD:** The project uses GitHub Actions for CI/CD. The workflow builds and tests the application, and then publishes a Docker image to GitHub Container Registry.
