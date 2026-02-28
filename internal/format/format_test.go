package format

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatTable_BasicASCII(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows: [][]string{
			{"1", "Alice", "alice@example.com"},
			{"2", "Bob", "bob@example.com"},
			{"3", "Charlie", "charlie@example.com"},
		},
	}

	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Style = ASCIIStyle
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check header
	if !strings.Contains(output, "id") {
		t.Error("output should contain column 'id'")
	}
	if !strings.Contains(output, "name") {
		t.Error("output should contain column 'name'")
	}
	if !strings.Contains(output, "email") {
		t.Error("output should contain column 'email'")
	}

	// Check data
	if !strings.Contains(output, "Alice") {
		t.Error("output should contain 'Alice'")
	}
	if !strings.Contains(output, "Bob") {
		t.Error("output should contain 'Bob'")
	}
	if !strings.Contains(output, "Charlie") {
		t.Error("output should contain 'Charlie'")
	}

	// Check ASCII borders
	if !strings.Contains(output, "+") {
		t.Error("ASCII style should use + for borders")
	}
	if !strings.Contains(output, "|") {
		t.Error("ASCII style should use | for vertical borders")
	}
	if !strings.Contains(output, "-") {
		t.Error("ASCII style should use - for horizontal borders")
	}
}

func TestFormatTable_Unicode(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"col1", "col2"},
		Rows:    [][]string{{"a", "b"}},
	}

	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Style = UnicodeStyle
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "┌") {
		t.Error("unicode style should use ┌ for top-left corner")
	}
	if !strings.Contains(output, "│") {
		t.Error("unicode style should use │ for vertical borders")
	}
	if !strings.Contains(output, "─") {
		t.Error("unicode style should use ─ for horizontal borders")
	}
}

func TestFormatTable_UnicodeFull(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"x"},
		Rows:    [][]string{{"1"}},
	}

	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Style = UnicodeFullStyle
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "╔") {
		t.Error("unicode_full style should use ╔ for top-left corner")
	}
}

func TestFormatCSV(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "name", "value"},
		Rows: [][]string{
			{"1", "test", "hello world"},
			{"2", "quoted", `has "quotes"`},
			{"3", "comma", "a,b,c"},
		},
	}

	var buf bytes.Buffer
	opts := Options{Format: CSVFormat}
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 { // header + 3 rows
		t.Errorf("expected 4 lines, got %d", len(lines))
	}

	// Header
	if lines[0] != "id,name,value" {
		t.Errorf("unexpected header: %s", lines[0])
	}

	// Data with quotes should be properly escaped
	if !strings.Contains(output, `"has ""quotes"""`) {
		t.Errorf("CSV should properly escape quotes, got: %s", output)
	}

	// Data with commas should be quoted
	if !strings.Contains(output, `"a,b,c"`) {
		t.Errorf("CSV should quote fields with commas, got: %s", output)
	}
}

func TestFormatTSV(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "hello"}, {"2", "world"}},
	}

	var buf bytes.Buffer
	opts := Options{Format: TSVFormat}
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a\tb" {
		t.Errorf("unexpected header: %q", lines[0])
	}
	if lines[1] != "1\thello" {
		t.Errorf("unexpected row 1: %q", lines[1])
	}
}

func TestFormatJSON(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "name"},
		Rows: [][]string{
			{"1", "Alice"},
			{"2", "Bob"},
		},
	}

	var buf bytes.Buffer
	opts := Options{Format: JSONFormat}
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var data []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if len(data) != 2 {
		t.Errorf("expected 2 rows, got %d", len(data))
	}
	if data[0]["id"] != "1" {
		t.Errorf("expected id=1, got %v", data[0]["id"])
	}
	if data[0]["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", data[0]["name"])
	}
	if data[1]["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", data[1]["name"])
	}
}

func TestFormatVertical(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows: [][]string{
			{"1", "Alice", "alice@test.com"},
			{"2", "Bob", "bob@test.com"},
		},
	}

	var buf bytes.Buffer
	opts := Options{Format: VerticalFormat}
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "RECORD 1") {
		t.Error("vertical output should contain 'RECORD 1'")
	}
	if !strings.Contains(output, "RECORD 2") {
		t.Error("vertical output should contain 'RECORD 2'")
	}
	if !strings.Contains(output, "id    | 1") {
		t.Error("vertical output should contain 'id    | 1'")
	}
	if !strings.Contains(output, "Alice") {
		t.Error("vertical output should contain 'Alice'")
	}
}

func TestFormatExpanded(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"col"},
		Rows:    [][]string{{"val"}},
	}

	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.Expanded = true
	err := Format(&buf, result, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "RECORD") {
		t.Error("expanded output should use vertical format")
	}
}

func TestFormatNil(t *testing.T) {
	var buf bytes.Buffer
	err := Format(&buf, nil, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error for nil result: %v", err)
	}
	if buf.Len() != 0 {
		t.Error("nil result should produce no output")
	}
}

func TestFormatEmptyColumns(t *testing.T) {
	result := &QueryResult{
		Columns: []string{},
		Rows:    [][]string{},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Error("empty columns should produce no output")
	}
}

func TestFormatSingleRow(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"count"},
		Rows:    [][]string{{"42"}},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "42") {
		t.Error("output should contain '42'")
	}
}

func TestFormatWideColumns(t *testing.T) {
	longValue := strings.Repeat("x", 200)
	result := &QueryResult{
		Columns: []string{"short", "long_column_name"},
		Rows:    [][]string{{"a", longValue}},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), longValue) {
		t.Error("output should contain the long value")
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"", 0},
		{"abc", 3},
		{"日本語", 6},  // CJK characters are double-width
		{"a日b", 4},   // mixed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			w := displayWidth(tt.input)
			if w != tt.expected {
				t.Errorf("displayWidth(%q) = %d, want %d", tt.input, w, tt.expected)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"long", 2, "long"}, // no truncation
		{"", 3, "   "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := padRight(tt.input, tt.width)
			if result != tt.expected {
				t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.Format != TableFormat {
		t.Errorf("default format should be TableFormat, got %v", opts.Format)
	}
	if opts.Style != UnicodeStyle {
		t.Errorf("default style should be UnicodeStyle, got %v", opts.Style)
	}
	if opts.NullValue != "NULL" {
		t.Errorf("default null value should be 'NULL', got %v", opts.NullValue)
	}
}

func TestFormatMultipleRows(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"num"},
		Rows:    make([][]string, 100),
	}
	for i := 0; i < 100; i++ {
		result.Rows[i] = []string{strings.Repeat("x", 10)}
	}

	var buf bytes.Buffer
	err := Format(&buf, result, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have header, separator, 100 data rows, plus borders
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// top border + header + separator + 100 rows + bottom border = 104
	if len(lines) != 104 {
		t.Errorf("expected 104 lines, got %d", len(lines))
	}
}

func TestFormatCSV_EmptyResult(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, Options{Format: CSVFormat})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still have header
	output := strings.TrimSpace(buf.String())
	if output != "a,b" {
		t.Errorf("empty CSV should still have header, got: %q", output)
	}
}

func TestFormatJSON_EmptyResult(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"x"},
		Rows:    [][]string{},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, Options{Format: JSONFormat})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var data []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty array, got %d elements", len(data))
	}
}

func TestFormatVertical_SingleRecord(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"a", "longer_name"},
		Rows:    [][]string{{"1", "value"}},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, Options{Format: VerticalFormat})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "RECORD 1") {
		t.Error("should contain RECORD 1")
	}
	if strings.Contains(output, "RECORD 2") {
		t.Error("should not contain RECORD 2")
	}
}

func TestFormatTable_SpecialCharacters(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"data"},
		Rows: [][]string{
			{"hello\nworld"},
			{"tab\there"},
			{"null\x00byte"},
		},
	}

	var buf bytes.Buffer
	err := Format(&buf, result, DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not panic and should produce output
	if buf.Len() == 0 {
		t.Error("should produce output even with special characters")
	}
}

func BenchmarkFormatTable(b *testing.B) {
	result := &QueryResult{
		Columns: []string{"id", "name", "email", "created_at", "status"},
		Rows:    make([][]string, 1000),
	}
	for i := range result.Rows {
		result.Rows[i] = []string{"12345", "John Doe", "john@example.com", "2024-01-15 10:30:00", "active"}
	}

	opts := DefaultOptions()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		Format(&buf, result, opts)
	}
}

func BenchmarkFormatCSV(b *testing.B) {
	result := &QueryResult{
		Columns: []string{"id", "name", "value"},
		Rows:    make([][]string, 1000),
	}
	for i := range result.Rows {
		result.Rows[i] = []string{"123", "test data", "some value here"}
	}

	opts := Options{Format: CSVFormat}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		Format(&buf, result, opts)
	}
}

func BenchmarkFormatJSON(b *testing.B) {
	result := &QueryResult{
		Columns: []string{"id", "name"},
		Rows:    make([][]string, 1000),
	}
	for i := range result.Rows {
		result.Rows[i] = []string{"123", "test data"}
	}

	opts := Options{Format: JSONFormat}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		Format(&buf, result, opts)
	}
}
