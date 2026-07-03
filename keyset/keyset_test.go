package keyset

import (
	"os"
	"path/filepath"
	"testing"
)

// writeCSV creates a temporary CSV file with the given lines and returns its path.
func writeCSV(t *testing.T, name string, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestLoad_NoHeader checks that Load counts every row as a key when there's no header.
func TestLoad_NoHeader(t *testing.T) {
	path := writeCSV(t, "data.csv", []string{"A", "B", "C", "D", "D", "E", "F", "F"})

	c, err := Load(path, Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Total != 8 {
		t.Errorf("Total = %d, want 8", c.Total)
	}
	if c.Distinct() != 6 {
		t.Errorf("Distinct = %d, want 6", c.Distinct())
	}
	if c.Freq["D"] != 2 || c.Freq["F"] != 2 {
		t.Errorf("Freq = %v, want D:2 F:2", c.Freq)
	}
}

// TestLoad_HeaderByName checks that Load resolves Column to an index using the header row.
func TestLoad_HeaderByName(t *testing.T) {
	path := writeCSV(t, "data.csv", []string{
		"id,udprn,name",
		"1,30433784,Alice",
		"2,08034283,Bob",
		"3,30433784,Carol",
	})

	c, err := Load(path, Options{HasHeader: true, Column: "udprn"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Total != 3 {
		t.Errorf("Total = %d, want 3", c.Total)
	}
	if c.Distinct() != 2 {
		t.Errorf("Distinct = %d, want 2", c.Distinct())
	}
	if c.Freq["30433784"] != 2 {
		t.Errorf("Freq[30433784] = %d, want 2", c.Freq["30433784"])
	}
}

// TestLoad_HeaderByIndex checks that a numeric Column still works when HasHeader is set,
// and that keys with leading zeros are preserved as strings.
func TestLoad_HeaderByIndex(t *testing.T) {
	path := writeCSV(t, "data.csv", []string{
		"id,udprn",
		"1,00012345",
		"2,00067890",
	})

	c, err := Load(path, Options{HasHeader: true, Column: "1"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Distinct() != 2 {
		t.Errorf("Distinct = %d, want 2", c.Distinct())
	}
	// Leading zeros must be preserved: keys are strings, not numbers.
	if _, ok := c.Freq["00012345"]; !ok {
		t.Errorf("expected leading-zero key %q to be preserved, got %v", "00012345", c.Freq)
	}
}

// TestLoad_UnknownColumnName checks that Load errors when Column names a header that doesn't exist.
func TestLoad_UnknownColumnName(t *testing.T) {
	path := writeCSV(t, "data.csv", []string{"id,udprn", "1,00012345"})

	_, err := Load(path, Options{HasHeader: true, Column: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown column name, got nil")
	}
}

// TestLoad_ColumnOutOfRange checks that Load errors when a row doesn't have the requested column.
func TestLoad_ColumnOutOfRange(t *testing.T) {
	path := writeCSV(t, "data.csv", []string{"A", "B"})

	_, err := Load(path, Options{Column: "5"})
	if err == nil {
		t.Fatal("expected error for out-of-range column index, got nil")
	}
}

// TestLoad_MissingFile checks that Load errors when the path doesn't exist.
func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.csv"), Options{})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestCompare_PDFExample checks Compare against the worked example from the task spec.
func TestCompare_PDFExample(t *testing.T) {
	// Example from the task spec:
	// Dataset 1: A B C D D E F F
	// Dataset 2: A C C D F F F X Y
	// Distinct overlap = 4 (A C D F)
	// Total overlap    = 11
	path1 := writeCSV(t, "d1.csv", []string{"A", "B", "C", "D", "D", "E", "F", "F"})
	path2 := writeCSV(t, "d2.csv", []string{"A", "C", "C", "D", "F", "F", "F", "X", "Y"})

	c1, err := Load(path1, Options{})
	if err != nil {
		t.Fatalf("Load d1: %v", err)
	}
	c2, err := Load(path2, Options{})
	if err != nil {
		t.Fatalf("Load d2: %v", err)
	}

	if c1.Total != 8 {
		t.Errorf("d1 Total = %d, want 8", c1.Total)
	}
	if c2.Total != 9 {
		t.Errorf("d2 Total = %d, want 9", c2.Total)
	}

	o := Compare(c1, c2)
	if o.Distinct != 4 {
		t.Errorf("Distinct overlap = %d, want 4", o.Distinct)
	}
	if o.Total != 11 {
		t.Errorf("Total overlap = %d, want 11", o.Total)
	}
}

// TestCompare_Symmetric checks that Compare(a, b) equals Compare(b, a).
func TestCompare_Symmetric(t *testing.T) {
	path1 := writeCSV(t, "d1.csv", []string{"A", "B", "C", "D", "D", "E", "F", "F"})
	path2 := writeCSV(t, "d2.csv", []string{"A", "C", "C", "D", "F", "F", "F", "X", "Y"})

	c1, _ := Load(path1, Options{})
	c2, _ := Load(path2, Options{})

	o1 := Compare(c1, c2)
	o2 := Compare(c2, c1)

	if o1 != o2 {
		t.Errorf("Compare not symmetric: %+v vs %+v", o1, o2)
	}
}

// TestCompare_NoOverlap checks that Compare returns a zero Overlap when the files share no keys.
func TestCompare_NoOverlap(t *testing.T) {
	path1 := writeCSV(t, "d1.csv", []string{"A", "B"})
	path2 := writeCSV(t, "d2.csv", []string{"C", "D"})

	c1, _ := Load(path1, Options{})
	c2, _ := Load(path2, Options{})

	o := Compare(c1, c2)
	if o.Distinct != 0 || o.Total != 0 {
		t.Errorf("Overlap = %+v, want zero overlap", o)
	}
}
