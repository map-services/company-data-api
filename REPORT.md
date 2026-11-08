# Companies House Import Optimization Report

## Summary

- **+ Kept:** synthetic benchmark harness, deferred postcode index, import-only SQLite tuning, parser/writer pipeline, and empty-reference NULL correction.
- **- Dropped:** multi-row SQL inserts and allocation-only CSV/numeric parsing changes. They did not improve the agreed throughput gate.
- **- Rejected:** `WITHOUT ROWID`; the candidate took about twice as long to import.
- **+ Preserved:** API connection settings are unchanged. Search returns an empty registration-country string when the stored value is NULL.

All imports used fresh databases under `/tmp`; `data/companies_data.db` was not modified. The supplied 495 MB archive was imported with `--max-records 200000`. Synthetic measurements used three 500,000-row runs unless noted. Import times below are the importer's elapsed-time log; peak RSS is from macOS `/usr/bin/time -l` around the benchmark command.

## Synthetic Benchmark

| Phase / decision                                 | Import time | Rows/s | Peak RSS |
| ------------------------------------------------ | ----------: | -----: | -------: |
| 0. Baseline, 500k rows                           |     14.18 s |  35.3k |  158 MiB |
| 1. Defer postcode index                          |      9.19 s |  54.4k |  164 MiB |
| 2. Import-only PRAGMAs                           |      8.13 s |  61.5k |  180 MiB |
| 3. Single-row statements, control                |      8.29 s |  60.3k |  178 MiB |
| 3. 500-row multi-value statements, rejected      |      8.58 s |  58.3k |  188 MiB |
| 4. Parse/write pipeline                          |      6.78 s |  73.8k |  211 MiB |
| 5. CSV reuse + parsed nullable numbers, rejected |      6.78 s |  73.7k |  217 MiB |
| 6. NULL correction, primitive SQL values         |      6.94 s |  72.0k |  212 MiB |
| 7. `WITHOUT ROWID` candidate, one run            |      4.76 s |  42.1k |  205 MiB |

Phase 1 improved the baseline by about 35%. Phase 2 added about 11.5% over Phase 1. Phase 4 added about 16.7% over Phase 2. The accepted phases together improved synthetic throughput by about 52% relative to the baseline.

Phase 3 compared 1-row statements (8.29 s average) with 500-row statements (8.58 s average); 250- and 900-row samples were slower too. The multi-row version also used more peak RSS, so it was removed. A duplicate-key test remains to lock down existing last-row-wins behavior.

Phase 5 reduced benchmark allocations from about 15.0M to 13.0M and bytes allocated from about 1.96 GB to 1.65 GB, but throughput was unchanged and peak RSS rose about 3%. The CPU profile attributed 56% of samples to SQLite/CGo, about 20% to syscalls, and about 1% flat time to CSV record reading. The allocation-only changes were removed because parsing was not a material CPU hotspot and did not improve elapsed time.

Phase 6's small synthetic slowdown (~2.4% versus Phase 4) is accepted for the deliberate correctness fix. Returning primitive `nil`/`int` values keeps allocation counts at the Phase 4 level.

Phase 7 failed its first gate: the candidate was about 2x slower than the accepted rowid schema on the same 200k archive sample. The optional schema change was discarded, so no search endpoint comparison was needed.

## Archive Import

| Phase                        | Records | Import elapsed | Rows/s | Peak RSS |
| ---------------------------- | ------: | -------------: | -----: | -------: |
| 0. Baseline                  |    200k |  4.68 s median |  42.8k |   68 MiB |
| 1. Deferred postcode index   |    200k |         3.50 s |  57.2k |   69 MiB |
| 2. Import-only PRAGMAs       |    200k |         3.26 s |  61.4k |  151 MiB |
| 4. Pipeline                  |    200k |         2.34 s |  85.5k |  193 MiB |
| 6. NULL correction           |    200k |         2.36 s |  84.8k |  196 MiB |
| 7. `WITHOUT ROWID` candidate |    200k |         4.76 s |  42.1k |  205 MiB |

The baseline `company_data` dump hash was `aaadba91b9c78c9a1c97e5c71a4ad32825071620ed2c98101d96a0cc624f6d8a`. Phases 1, 2, and 4 reproduced that hash exactly. Phase 6 reproduced it after normalizing the three nullable reference IDs (`NULL` to prior zero), proving no other company-data values changed.

Phase 6 stored 37,567 empty `reg_address_country` values as NULL. The sample had no empty `country_of_origin` or `accounts_account_category` values. Required empty company category and status values now fail their database `NOT NULL` constraints. All reference table counts remained unchanged: 22 categories, 9 statuses, 16 accounts categories, 79 countries, and 83 origins.

## Phase Commits

| Commit    | Change                                       |
| --------- | -------------------------------------------- |
| `b508a48` | Add synthetic import benchmark harness       |
| `618d328` | Defer postcode index during import           |
| `e164bf0` | Tune SQLite for import commands only         |
| `bd7c18c` | Test duplicate company replacement semantics |
| `6ed0de2` | Pipeline CSV parsing and database writes     |
| `f630531` | Store empty optional references as NULL      |

The dropped Phase 3 and Phase 5 implementations and Phase 7 candidate were not committed. Full tests, `golangci-lint run`, and `go build -tags=jsoniter` passed for each retained implementation phase. The pipeline also passed `go test -race ./internal/importer`.
