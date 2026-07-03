# set-intersection

> Written by Leo Ridgwell as a technical test submission for InfoSum.  
> Time spent: ~3.5 hours - a bit over the suggested 2-3, partly ramp-up  
> time since Go is newer to me than other languages I've worked in.

Compares the keys in two CSV files and reports:

- the count of keys in each file
- the count of distinct keys in each file
- the distinct overlap (how many distinct keys appear in both files)
- the total overlap (see [How overlap is defined](#how-overlap-is-defined))

Written for the InfoSum set-intersection exercise. `examples/A_f.csv` and
`examples/B_f.csv` are the sample datasets from the task - single-column
CSVs of UDPRN keys with a header row. The brief also asked for handling
different CSV shapes (see [Flags](#flags)), thinking through larger files
(see [Scaling](#scaling)), and being honest about accuracy if
approximations are used - which they aren't here, everything is exact.

## Layout

```
cmd/          CLI entrypoint (flag parsing, output formatting)
keyset/       CSV reading, counting, and comparison logic
examples/     sample UDPRN datasets from the task
```

The comparison logic lives in its own package (`keyset`) independent of the
CLI, and `main` is under `cmd/` so another entrypoint (a small HTTP service
wrapping the same package, say) could sit alongside it without reshuffling
anything.

## Build & run

Requires Go 1.21+, no external dependencies.

```sh
go build -o set-intersection.exe ./cmd
./set-intersection -file1 examples/A_f.csv -file2 examples/B_f.csv -header -column udprn
```

or just `go run ./cmd -file1 examples/A_f.csv -file2 examples/B_f.csv -header -column udprn`.

`make build`, `make test`, `make run` do the same via the Makefile.

### Flags

| Flag         | Default      | Description                                                             |
|--------------|--------------|-------------------------------------------------------------------------|
| `-file1`     | *(required)* | Path to the first CSV file                                              |
| `-file2`     | *(required)* | Path to the second CSV file                                             |
| `-header`    | `false`      | Set if the CSV files have a header row                                  |
| `-column`    | `0`          | Key column - a zero-based index, or a header name when `-header` is set |
| `-delimiter` | `,`          | Field delimiter, e.g. `;` or `\t` for TSV                               |
| `-json`      | `false`      | Print the result as JSON instead of a table                             |

`-header`/`-column`/`-delimiter` apply to both files - fine for comparing two
exports from the same pipeline. If the two files are shaped differently,
normalize one first (`cut`, `awk`, whatever's convenient).

### Example

```
$ go run ./cmd -file1 examples/A_f.csv -file2 examples/B_f.csv -header -column udprn
File                                             Keys     Distinct
examples/A_f.csv                                95000        72799
examples/B_f.csv                                80000        72815

Distinct overlap                                58222
Total overlap                                60627882
```

## How overlap is defined

The task PDF gives this example:

```
Dataset 1: A B C D D E F F
Dataset 2: A C C D F F F X Y
Distinct Overlap = A C D F = 4
Total Overlap = A C C D D F F F F F F = 11
```

Distinct overlap is just the size of the set intersection - easy.

Total overlap took a second look though. A plain multiset intersection
(`min` of the two counts per key) gives `1+1+1+2 = 5` for this example, not
11. Working backwards from 11 instead: `C` contributes 2 (1x2), `D`
contributes 2 (2x1), `F` contributes 6 (2x3), `A` contributes 1 (1x1) - so
each shared key contributes `count_in_file1 * count_in_file2`, not
`min(count_in_file1, count_in_file2)`. That lines up with how the task
describes it: "the maximum possible overlap" - the number of matching row
pairs you'd get from a SQL join on the key with no dedup, since any
occurrence in file A could line up with any occurrence in file B. `Compare`
in `keyset/keyset.go` implements it that way, and `TestCompare_PDFExample`
pins the numbers down against the PDF's own example.

Keys are treated as strings, not numbers, so `08034283` keeps its leading
zero.

## Design notes

- CSV files are streamed row by row rather than loaded whole, so memory is
  driven by the number of distinct keys, not the number of rows.
- The two files are read concurrently on separate goroutines, so wall-clock
  time is roughly `max(file1, file2)` instead of the sum.
- `Compare` walks whichever file has fewer distinct keys and looks each one
  up in the other, so it's `O(min(distinct1, distinct2))`.
- Standard library only - `go build` works with nothing else installed.

## Testing

```sh
go test ./...
```

`keyset/keyset_test.go` covers the PDF's worked example, header/no-header
parsing, selecting the key column by name or index, leading zeros, and a
few error cases (missing file, unknown column, out-of-range column).
`cmd/main_test.go` drives the CLI end to end, including
`-json` and a custom delimiter. Test fixtures are small CSVs written to a
temp dir at test time rather than checked-in files - the `examples/` CSVs
are for manual runs, not assertions, since there's no independently-known-
correct overlap number for them to check against.

The two input files are read concurrently (`loadBoth` in `cmd/main.go`), so
it's also worth running with the race detector: `go test -race ./...`
(needs cgo, so a C compiler - already present on most Linux/macOS setups,
MinGW-w64 on Windows).

## Scaling

Memory is one `map[string]int` per file, sized to the number of distinct
keys - not the row count. I checked this holds up by running it against two
synthetic files of 10M and 8M rows (~7.9M and ~6.6M distinct keys, ~180MB
combined): about 6.7s wall clock, ~950MB peak memory, which tracks - a Go
`map[string]int` costs roughly 60-100 bytes per entry once you include the
string data and bucket overhead.

UDPRN specifically also has a natural ceiling: it's a Royal Mail identifier
for UK delivery points, and there are only tens of millions of those in
total, so the distinct key space (and this tool's memory use) is bounded
regardless of row count. If the key type were something much larger or
unbounded, the next steps would be an external sort-merge join or sharding
by `hash(key) % N` to get past the memory limit, and HyperLogLog/MinHash for
approximate counts if exact ones stopped being feasible - reporting an error
bound alongside the estimate, per the task's own note on that.

## If I had more time

- CI (GitHub Actions running build/vet/test/gofmt on push) and a benchmark
  suite, so both correctness and performance regressions get caught
  automatically rather than by hand.
- Per-file `-column`/`-delimiter` overrides for comparing two files that
  are shaped differently, plus reading from stdin, gzip input, and
  composite (multi-column) keys.
- The bigger-than-memory approaches from [Scaling](#scaling) - sort-merge
  join, sharding, or approximate counting.
- More use of goroutines: right now concurrency is just the two files
  loading in parallel. A single large file could be split into byte-range
  chunks, parsed on separate goroutines, and the per-chunk frequency maps
  merged afterward - worth doing if profiling showed parsing, not I/O, was
  the actual bottleneck.
- Fuzz testing the CSV parsing path, and packaging (Dockerfile, prebuilt
  binaries) so it doesn't need a Go toolchain to run.
