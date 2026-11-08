# PLAN: Speed up the Companies House import

Repo: `github.com/map-services/company-data-api` (Go, SQLite, Gin)
Scope: `import-companies-house` only (`internal/importer/company_data.go` and supporting files). Do not change API behaviour.

## Goal

Reduce wall-clock time of `./company-data import-companies-house --zip-file <file>` (~5M+ rows) without changing the data that ends up in the database, apart from the deliberate fix listed in Phase 6.

## Ground rules

- **Measure before and after every phase.** Do not keep a change that does not show a measurable gain, and record the numbers in the final summary.
- One phase = one commit, so any phase can be reverted independently.
- Run `golangci-lint run` and the test suite (`go test ./...`, including `internal/importer/company_data_test.go`) after each phase. Existing tests must keep passing; update them only where a phase intentionally changes behaviour, and say so.
- Build with `-tags=jsoniter` as the repo does.
- Durability settings (Phase 2) must apply to the import commands only, never to `api-server`.

## Background (from reading the code; verify before relying on it)

- `internal.Connect` opens SQLite with `_busy_timeout=5000&_journal_mode=WAL` and runs `migration.sql` on every connect. The driver name `sqlite3` suggests `mattn/go-sqlite3`; confirm in `go.mod`, since DSN options differ between drivers.
- `processCSV` is a single-goroutine loop: parse a row (via `internal.ParseCSV`), append to a batch of 5000, then `insertBatch` runs synchronously. Parsing and writing never overlap.
- `insertBatch` opens a transaction, calls `tx.Prepare` for every batch, and does one `stmt.Exec` per row (34 parameters each) using `INSERT OR REPLACE`.
- `company_data` has `company_number TEXT PRIMARY KEY` on a normal rowid table (so a table B-tree plus a separate unique index), and a secondary index `idx_company_data_reg_address_post_code`. Both are updated on every insert in effectively random key order.
- `getOrCreateRefID` caches reference lookups in maps, so it is cheap on cache hits. It returns `0` for empty strings.
- `companyDataToTuple` appears unused (the tuple is built inline in `insertBatch`).

## Phase 0: Baseline and harness

1. Create a small generator (a `_test.go` benchmark or a script under `cmd/` or `scripts/`, not shipped in the main binary) that writes a zip containing a CSV with the real Companies House layout: the same header and column order, at least 55 columns (the importer reads indexes 0–31 and 53–54), realistic distributions for the low-cardinality fields, **company numbers not in sorted order**, and postcodes drawn from a few hundred thousand distinct values. Generate roughly 500k rows, and make the size a parameter.
2. Record the baseline: total time, rows/second, and peak RSS, from at least 3 runs into a fresh DB file each time.
3. After the baseline import, save a canonical dump for later comparison, e.g. `sqlite3 db "SELECT * FROM company_data ORDER BY company_number"` hashed, plus row counts of each reference table. Every later phase must reproduce an identical result (except Phase 6, where only the empty-value rows differ).
4. Capture a CPU profile of the baseline (`pprof` is already wired via `--debug` for the server; for the importer, add a temporary `runtime/pprof` hook) to confirm where time goes before optimising.

## Phase 1: Defer secondary index creation

- After `internal.Connect` (which recreates the index via `IF NOT EXISTS`), `DROP INDEX IF EXISTS idx_company_data_reg_address_post_code` before importing, and `CREATE INDEX` again after the last batch and before `ANALYZE`.
- Make sure the index is restored even if the import fails partway (use a deferred recreate, or rely on the next `Connect`'s migration and document that). Do not leave a DB that silently lacks the index after a successful run.
- Keep the SQL for the index defined in one place (reuse the statement from `migration.sql` rather than duplicating it).

## Phase 2: Import-only SQLite tuning

- Add an import-specific connection option (e.g. `ConnectForImport`, or a parameter on `Connect`) that sets `synchronous=NORMAL` (or `OFF`, but justify it: the DB can be rebuilt by re-running the import), a larger `cache_size` (e.g. 256 MB expressed in pages or KiB), and `temp_store=MEMORY`.
- PRAGMAs that are per-connection must apply to the connection actually doing the writes. Either use DSN params the driver supports, or call `db.SetMaxOpenConns(1)` and `Exec` the PRAGMAs. Verify with `PRAGMA synchronous;` that the setting took effect.
- Only wire this into `cmd/companies_house.go` (and optionally `code_point.go` and `sic_codes.go` if the same benefit shows). `api-server` must keep its current settings.

## Phase 3: One prepared statement, multi-row inserts

- Replace per-row `stmt.Exec` with multi-row `INSERT OR REPLACE ... VALUES (...),(...),...`. 34 columns per row; stay well under SQLite's variable limit (32766 on modern builds, 999 on old ones, so check the bundled SQLite version). About 500 rows per statement is a reasonable starting point; benchmark a few sizes.
- Prepare the full-size statement once for the whole import (or per transaction) and a separate remainder statement for the final partial chunk, instead of preparing per batch.
- Keep the transaction batching, but consider a larger transaction size (e.g. 20k–50k rows) now that fsync cost is reduced; benchmark.
- Preserve `INSERT OR REPLACE` semantics (re-importing over an existing DB must still replace by `company_number`). Note: duplicate `company_number` values within a single multi-row statement still resolve with last-wins semantics, so add a test for this.

## Phase 4: Pipeline parsing and writing

- Run CSV parsing in one goroutine and the DB writer in another, connected by a buffered channel of batches (a small buffer, e.g. 2–4 batches). The writer must stay single, because SQLite allows one writer.
- Handle errors and cancellation properly: a parse error or write error must stop the other goroutine and return the first error with its line number; no goroutine leaks; the zip reader must close on every path. Use `context.Context` or `errgroup`.
- Batches handed over the channel must not be reused or mutated by the producer while the writer holds them (allocate per batch, or use a pool with explicit hand-back).
- Preserve the progress log (`"Inserted records"` with line number).

## Phase 5: Cut per-row allocation and conversion work

- Set `csvReader.ReuseRecord = true` in `internal/csv.go`. This is safe only if no consumer retains the `[]string` slice itself (strings are fine). Check `code_point.go` and `sic_codes.go`, which share `ParseCSV`, and `csv_test.go`.
- Parse the six nullable numeric fields (`mortgages_*`, `limited_partnerships_*`) once in `fromCompanyDataCSV` rather than keeping strings in the model and re-parsing in `insertBatch`. Check the `models.CompanyData` field types and any API code that uses them before changing the model; if the model is shared with the API, introduce an import-specific row struct instead.
- Evaluate passing dates as pre-formatted strings or Unix integers rather than `*time.Time`, **only if** the stored representation stays identical to what the driver writes today (compare the dump from Phase 0). If it cannot be made identical, skip this item.
- Delete `companyDataToTuple` if confirmed unused (`grep` first).
- Keep gains only if the profile shows these costs matter.

## Phase 6: Fix empty reference values (behaviour change, separate commit)

- `getOrCreateRefID` returns `0` for empty strings, so nullable FK columns (`country_of_origin`, `reg_address_country`, `accounts_account_category`) get `0` instead of `NULL`. Return a nullable value for empty strings instead, and keep `NOT NULL` columns (`company_category`, `company_status`) failing loudly rather than silently getting `0`.
- Check `internal/repositories/search.go` and `internal/sql/search.sql` for how these columns are joined, to make sure `NULL` is handled the same as the current `0` (e.g. LEFT JOINs, `COALESCE`).
- Add or update tests. Call this out explicitly in the commit message because the resulting DB differs from before.

## Phase 7 (optional, gate on measurements): `WITHOUT ROWID` for `company_data`

- Changing the table to `WITHOUT ROWID` gives one B-tree keyed on `company_number` instead of a table plus a unique index, which should reduce both import time and file size.
- `CREATE TABLE IF NOT EXISTS` will not alter existing DBs, so this needs a decision: either document that a fresh DB / re-import is required, or write an explicit migration. Do not change it silently.
- Confirm API search queries (the join between `company_data` and `code_point` on postcode) are not slower. Benchmark `GET /v1/company-data/search` and `/by-postcode` on a populated DB before and after, and only keep this phase if the import gain is clear and search latency is not worse.

## Acceptance criteria

- The Phase 0 canonical dump is identical after Phases 1–5 and 7; after Phase 6 only the intended `NULL` vs `0` rows differ.
- The test suite and `golangci-lint` pass.
- A summary table reports the time, rows/sec and peak RSS for the baseline and after each phase, plus which phases were kept or dropped and why.
- `api-server` behaviour and its SQLite settings are unchanged.

## Out of scope

- The API server, routes and Swagger docs.
- The CodePoint and SIC importers (except where a shared helper change in Phase 2 or 5 touches them, in which case their tests must still pass).
- Changing the data source or download logic (`internal/downloader.go`). It was not reviewed here; if profiling shows the download dominates, report it rather than changing it.
