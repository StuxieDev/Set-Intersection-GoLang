package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCSV creates a temporary CSV file with the given lines and returns its path.
func writeCSV(t *testing.T, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// pdfExampleFiles recreates the worked example from set_intersection.pdf:
//
//	Dataset 1: A B C D D E F F
//	Dataset 2: A C C D F F F X Y
//	Distinct Overlap = 4, Total Overlap = 11
func pdfExampleFiles(t *testing.T) (file1, file2 string) {
	t.Helper()
	file1 = writeCSV(t, "d1.csv", []string{"A", "B", "C", "D", "D", "E", "F", "F"})
	file2 = writeCSV(t, "d2.csv", []string{"A", "C", "C", "D", "F", "F", "F", "X", "Y"})
	return file1, file2
}

// TestRun_PDFExample_Table checks that run's table output reports the worked example's overlap figures.
func TestRun_PDFExample_Table(t *testing.T) {
	file1, file2 := pdfExampleFiles(t)
	var stdout, stderr bytes.Buffer

	err := run([]string{"-file1", file1, "-file2", file2}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v (stderr: %s)", err, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{"Distinct overlap", "4", "Total overlap", "11"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
}

// TestRun_PDFExample_JSON checks that run's -json output matches the worked example's figures.
func TestRun_PDFExample_JSON(t *testing.T) {
	file1, file2 := pdfExampleFiles(t)
	var stdout, stderr bytes.Buffer

	err := run([]string{"-file1", file1, "-file2", file2, "-json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v (stderr: %s)", err, stderr.String())
	}

	var result struct {
		Files []struct {
			Path     string `json:"path"`
			Keys     int    `json:"keys"`
			Distinct int    `json:"distinct"`
		} `json:"files"`
		DistinctOverlap int `json:"distinct_overlap"`
		TotalOverlap    int `json:"total_overlap"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, stdout.String())
	}

	if result.DistinctOverlap != 4 {
		t.Errorf("DistinctOverlap = %d, want 4", result.DistinctOverlap)
	}
	if result.TotalOverlap != 11 {
		t.Errorf("TotalOverlap = %d, want 11", result.TotalOverlap)
	}
	if len(result.Files) != 2 || result.Files[0].Keys != 8 || result.Files[1].Keys != 9 {
		t.Errorf("Files = %+v, want keys 8 and 9", result.Files)
	}
}

// TestRun_CustomDelimiter checks that run works with a header row and a non-comma delimiter (TSV).
func TestRun_CustomDelimiter(t *testing.T) {
	file1 := writeCSV(t, "d1.tsv", []string{"id\tudprn", "1\t00012345", "2\t00067890"})
	file2 := writeCSV(t, "d2.tsv", []string{"id\tudprn", "1\t00012345", "2\t00099999"})
	var stdout, stderr bytes.Buffer

	err := run([]string{
		"-file1", file1, "-file2", file2,
		"-header", "-column", "udprn", "-delimiter", "\t",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v (stderr: %s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Distinct overlap") {
		t.Errorf("output missing expected content:\n%s", stdout.String())
	}
}

// TestRun_MissingArgs checks that run errors when a required flag is omitted.
func TestRun_MissingArgs(t *testing.T) {
	file1, _ := pdfExampleFiles(t)
	var stdout, stderr bytes.Buffer

	err := run([]string{"-file1", file1}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when -file2 is missing")
	}
}
