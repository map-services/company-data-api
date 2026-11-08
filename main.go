package main

import (
	"github.com/map-services/company-data-api/cmd"

	"github.com/spf13/cobra"
)

func main() {
	var err error
	var dbPath string
	var port int
	var debug bool
	var companiesHouseZipFile string
	var maxRecords int
	var codepointZipFile string
	var sicCodesCSVFile string

	rootCmd := &cobra.Command{
		Use:  "company-data",
		Long: `Company Data API & data importers`,
	}

	apiServerCmd := &cobra.Command{
		Use:   "api-server [--db <path>] [--port <port>] [--debug]",
		Short: "Start HTTP API server",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.ApiServer(dbPath, port, debug)
		},
	}
	apiServerCmd.Flags().IntVar(&port, "port", 8080, "Port to run HTTP server on")
	apiServerCmd.Flags().BoolVar(&debug, "debug", false, "Enable debugging (pprof) - WARING: do not enable in production")

	processCompaniesHouseZipCmd := &cobra.Command{
		Use:   "import-companies-house [--zip-file <path>] [--db <path>] [--max-records <n>]",
		Short: "Import Companies House ZIP file",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.ImportCompaniesHouseZipFile(companiesHouseZipFile, dbPath, maxRecords)
		},
	}
	processCompaniesHouseZipCmd.Flags().StringVar(&companiesHouseZipFile, "zip-file", "./data/BasicCompanyDataAsOneFile-2025-09-01.zip", "Path to Companies House .zip file")
	processCompaniesHouseZipCmd.Flags().IntVar(&maxRecords, "max-records", 0, "Maximum company records to import (0 means no limit)")

	processCodepointZipCmd := &cobra.Command{
		Use:   "import-code-point [--zip-file <path>] [--db <path>]",
		Short: "Import Codepoint ZIP file",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.ImportCodepointZipFile(codepointZipFile, dbPath)
		},
	}
	processCodepointZipCmd.Flags().StringVar(&codepointZipFile, "zip-file", "./data/codepo_gb.zip", "Path to Codepoint .zip file")

	importSICCodesCmd := &cobra.Command{
		Use:   "import-sic-codes [--csv-file <path>] [--db <path>]",
		Short: "Import SIC codes from CSV",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.ImportSICCodes(sicCodesCSVFile, dbPath)
		},
	}
	importSICCodesCmd.Flags().StringVar(&sicCodesCSVFile, "csv-file", "https://assets.publishing.service.gov.uk/media/5a7f8639e5274a2e87db65e1/SIC07_CH_condensed_list_en.csv", "Path or URL to SIC07 CSV file")

	rootCmd.AddCommand(apiServerCmd)
	rootCmd.AddCommand(processCompaniesHouseZipCmd)
	rootCmd.AddCommand(processCodepointZipCmd)
	rootCmd.AddCommand(importSICCodesCmd)
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "./data/companies_data.db", "Path to Companies data SQLite database")

	if err = rootCmd.Execute(); err != nil {
		panic(err)
	}
}
