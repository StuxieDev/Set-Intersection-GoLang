// Package keyset reads keys from CSV files and compares them.
package keyset

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Options controls how a CSV file is parsed into keys.
type Options struct {
	// HasHeader indicates the CSV's first row is a header, not data.
	HasHeader bool
	// Column selects the key column. It may be a zero-based numeric index
	// (e.g. "0") or, when HasHeader is true, a header name (e.g. "udprn").
	// An empty value defaults to column 0.
	Column string
	// Delimiter is the field separator. The zero value defaults to ','.
	// Set it to handle other dialects (e.g. ';' or '\t') so the two files
	// being compared don't have to be identically formatted.
	Delimiter rune
}

// Counts holds per-file key statistics: the total number of keys read
// (including duplicates) and how many times each distinct key occurred.
type Counts struct {
	// Path is the file the counts were loaded from.
	Path string
	// Total is the number of keys read, including duplicates.
	Total int
	// Freq maps each distinct key to the number of times it appeared in the file.
	Freq map[string]int
}

// Distinct returns the number of distinct keys.
func (c *Counts) Distinct() int {
	return len(c.Freq)
}

// Load reads path as CSV and returns key frequency counts for it.
//
// The file is read one row at a time instead of being loaded in full, so
// memory use is driven by the number of distinct keys (one map entry each),
// not by the number of rows in the file.
func Load(path string, opts Options) (*Counts, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1 // rows are allowed to have different column counts
	r.ReuseRecord = true   // reuse the field slice between rows to cut down on allocations
	if opts.Delimiter != 0 {
		r.Comma = opts.Delimiter
	}

	// Work out which column holds the key. If it's numeric we already know
	// the index; if it's a header name we can't resolve it until we've read
	// the header row below.
	colIndex := -1
	if opts.Column == "" {
		colIndex = 0
	} else if idx, err := strconv.Atoi(opts.Column); err == nil {
		colIndex = idx
	}

	counts := &Counts{Path: path, Freq: make(map[string]int)}

	rowNum := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: line %d: %w", path, rowNum+1, err)
		}
		rowNum++

		// First row: if it's a header, resolve the column name to an index
		// (if needed) and skip it rather than counting it as a key.
		if rowNum == 1 && opts.HasHeader {
			if colIndex == -1 {
				colIndex = indexOf(record, opts.Column)
				if colIndex == -1 {
					return nil, fmt.Errorf("%s: column %q not found in header %v", path, opts.Column, record)
				}
			}
			continue
		}

		if colIndex < 0 || colIndex >= len(record) {
			return nil, fmt.Errorf("%s: line %d: no column %d in row %v", path, rowNum, colIndex, record)
		}

		counts.Total++
		counts.Freq[record[colIndex]]++
	}

	return counts, nil
}

// indexOf returns the position of name in header, or -1 if it isn't there.
func indexOf(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

// Overlap holds the result of comparing two Counts.
type Overlap struct {
	// Distinct is the number of distinct keys present in both files.
	Distinct int
	// Total is the "maximum possible overlap" as defined by the task spec.
	// For each key present in both files, every occurrence in file a could
	// potentially pair with every occurrence in file b, so the key
	// contributes count(a) * count(b) to the total. This is the same number
	// you'd get from a SQL inner join on the key with no deduplication
	// (verified against the worked example in the task PDF - see README).
	Total int
}

// Compare computes the distinct and total overlap between a and b.
//
// It loops over whichever of the two has fewer distinct keys and does a map
// lookup into the other side, so the cost is O(min(distinct(a), distinct(b)))
// rather than a full nested loop over both key sets.
func Compare(a, b *Counts) Overlap {
	small, big := a, b
	if len(b.Freq) < len(a.Freq) {
		small, big = b, a
	}

	var o Overlap
	for key, n := range small.Freq {
		m, ok := big.Freq[key]
		if !ok {
			continue // key only appears in one of the two files
		}
		o.Distinct++
		o.Total += n * m
	}
	return o
}
