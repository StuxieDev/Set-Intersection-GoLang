// Command set-intersection compares the keys in two CSV files and reports
// how many keys each contains and how much they overlap.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"setintersection/keyset"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run does the actual work. It takes the writers explicitly (rather than
// using os.Stdout/os.Stderr directly) so tests can capture output.
func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("set-intersection", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		file1     string
		file2     string
		hasHeader bool
		column    string
		delimiter string
		asJSON    bool
	)

	fs.StringVar(&file1, "file1", "", "path to the first CSV file (required)")
	fs.StringVar(&file2, "file2", "", "path to the second CSV file (required)")
	fs.BoolVar(&hasHeader, "header", false, "set if the CSV files have a header row")
	fs.StringVar(&column, "column", "0", "key column: zero-based index, or header name when -header is set")
	fs.StringVar(&delimiter, "delimiter", ",", `field delimiter, e.g. ";" or "\t" for TSV`)
	fs.BoolVar(&asJSON, "json", false, "print the result as JSON instead of a table")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: set-intersection -file1 <path> -file2 <path> [-header] [-column <name-or-index>] [-delimiter <char>] [-json]")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if file1 == "" || file2 == "" {
		fs.Usage()
		return fmt.Errorf("-file1 and -file2 are required")
	}

	delim, err := parseDelimiter(delimiter)
	if err != nil {
		return err
	}

	// Both files are read with the same options. That's fine for the common
	// case of comparing two exports from the same pipeline; if the files
	// genuinely differ in shape, normalize one of them first.
	opts := keyset.Options{HasHeader: hasHeader, Column: column, Delimiter: delim}

	c1, c2, err := loadBoth(file1, file2, opts)
	if err != nil {
		return err
	}

	overlap := keyset.Compare(c1, c2)

	if asJSON {
		return printJSON(stdout, c1, c2, overlap)
	}
	printTable(stdout, c1, c2, overlap)
	return nil
}

// parseDelimiter turns a flag value into a single delimiter rune. It accepts
// a literal one-character string (e.g. ";") as well as the shell-friendly
// escape "\t" for tab-separated files, since typing an actual tab character
// on a command line is awkward.
func parseDelimiter(s string) (rune, error) {
	switch s {
	case "", ",":
		return ',', nil
	case `\t`, "\t":
		return '\t', nil
	}
	runes := []rune(s)
	if len(runes) != 1 {
		return 0, fmt.Errorf("-delimiter must be a single character, got %q", s)
	}
	return runes[0], nil
}

// loadBoth loads both files at the same time on separate goroutines, since
// each is an independent read-and-count pass over its own file. Wall-clock
// time ends up close to max(len(file1), len(file2)) instead of the sum.
func loadBoth(file1, file2 string, opts keyset.Options) (*keyset.Counts, *keyset.Counts, error) {
	type result struct {
		counts *keyset.Counts
		err    error
	}
	ch1 := make(chan result, 1)
	ch2 := make(chan result, 1)

	go func() {
		c, err := keyset.Load(file1, opts)
		ch1 <- result{c, err}
	}()
	go func() {
		c, err := keyset.Load(file2, opts)
		ch2 <- result{c, err}
	}()

	// Channels are buffered, so both goroutines can finish and send their
	// result even if we're slow to read it - reading both here always
	// completes without deadlocking, whichever one errors first.
	r1, r2 := <-ch1, <-ch2
	if r1.err != nil {
		return nil, nil, r1.err
	}
	if r2.err != nil {
		return nil, nil, r2.err
	}
	return r1.counts, r2.counts, nil
}

// printTable writes the human-readable report: per-file counts followed by
// the two overlap figures.
func printTable(w io.Writer, c1, c2 *keyset.Counts, o keyset.Overlap) {
	fmt.Fprintf(w, "%-40s %12s %12s\n", "File", "Keys", "Distinct")
	fmt.Fprintf(w, "%-40s %12d %12d\n", c1.Path, c1.Total, c1.Distinct())
	fmt.Fprintf(w, "%-40s %12d %12d\n", c2.Path, c2.Total, c2.Distinct())
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-40s %12d\n", "Distinct overlap", o.Distinct)
	fmt.Fprintf(w, "%-40s %12d\n", "Total overlap", o.Total)
}

// printJSON writes the same report as printTable, but as JSON for scripting.
func printJSON(w io.Writer, c1, c2 *keyset.Counts, o keyset.Overlap) error {
	out := struct {
		Files []struct {
			Path     string `json:"path"`
			Keys     int    `json:"keys"`
			Distinct int    `json:"distinct"`
		} `json:"files"`
		DistinctOverlap int `json:"distinct_overlap"`
		TotalOverlap    int `json:"total_overlap"`
	}{
		DistinctOverlap: o.Distinct,
		TotalOverlap:    o.Total,
	}
	for _, c := range []*keyset.Counts{c1, c2} {
		out.Files = append(out.Files, struct {
			Path     string `json:"path"`
			Keys     int    `json:"keys"`
			Distinct int    `json:"distinct"`
		}{Path: c.Path, Keys: c.Total, Distinct: c.Distinct()})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
