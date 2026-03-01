// Package format provides output formatting for query results.
// It supports table (ASCII/Unicode), CSV, TSV, JSON, and vertical/expanded output.
package format

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// OutputFormat specifies the output format type.
type OutputFormat string

const (
	TableFormat    OutputFormat = "table"
	CSVFormat      OutputFormat = "csv"
	TSVFormat      OutputFormat = "tsv"
	JSONFormat     OutputFormat = "json"
	VerticalFormat OutputFormat = "vertical"
	AlignedFormat  OutputFormat = "aligned"
)

// TableStyle controls the table border style.
type TableStyle string

const (
	ASCIIStyle       TableStyle = "ascii"
	PsqlStyle        TableStyle = "psql"
	UnicodeStyle     TableStyle = "unicode"
	UnicodeFullStyle TableStyle = "unicode_full"
)

// Options configures the output formatter.
type Options struct {
	Format    OutputFormat
	Style     TableStyle
	Expanded  bool // \x expanded output
	MaxWidth  int  // terminal width for wrapping
	NullValue string
	FloatFmt  string
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		Format:    TableFormat,
		Style:     UnicodeStyle,
		MaxWidth:  0, // auto-detect
		NullValue: "NULL",
		FloatFmt:  "%.2f",
	}
}

// QueryResult holds the result of a query execution.
type QueryResult struct {
	Columns    []string
	Rows       [][]string
	StatusText string // e.g. "SELECT 5", "INSERT 0 1"
	RowCount   int
}

// Format writes the query result to w using the specified options.
func Format(w io.Writer, result *QueryResult, opts Options) error {
	if result == nil {
		return nil
	}
	if opts.Expanded || opts.Format == VerticalFormat {
		return formatVertical(w, result, opts)
	}
	switch opts.Format {
	case TableFormat, AlignedFormat:
		return formatTable(w, result, opts)
	case CSVFormat:
		return formatCSV(w, result, ',')
	case TSVFormat:
		return formatCSV(w, result, '\t')
	case JSONFormat:
		return formatJSON(w, result)
	default:
		return formatTable(w, result, opts)
	}
}

// borders holds the characters used for table borders.
type borders struct {
	TopLeft, TopMid, TopRight       string
	MidLeft, MidMid, MidRight       string
	BotLeft, BotMid, BotRight       string
	Horizontal, Vertical            string
	HeaderHorizontal                string
}

func asciiBorders() borders {
	return borders{
		TopLeft: "+", TopMid: "+", TopRight: "+",
		MidLeft: "+", MidMid: "+", MidRight: "+",
		BotLeft: "+", BotMid: "+", BotRight: "+",
		Horizontal: "-", Vertical: "|",
		HeaderHorizontal: "-",
	}
}

func unicodeBorders() borders {
	return borders{
		TopLeft: "┌", TopMid: "┬", TopRight: "┐",
		MidLeft: "├", MidMid: "┼", MidRight: "┤",
		BotLeft: "└", BotMid: "┴", BotRight: "┘",
		Horizontal: "─", Vertical: "│",
		HeaderHorizontal: "─",
	}
}

func unicodeFullBorders() borders {
	return borders{
		TopLeft: "╔", TopMid: "╦", TopRight: "╗",
		MidLeft: "╠", MidMid: "╬", MidRight: "╣",
		BotLeft: "╚", BotMid: "╩", BotRight: "╝",
		Horizontal: "═", Vertical: "║",
		HeaderHorizontal: "═",
	}
}

func psqlBorders() borders {
	return borders{
		TopLeft: "", TopMid: "", TopRight: "",
		MidLeft: "|", MidMid: "+", MidRight: "|",
		BotLeft: "", BotMid: "", BotRight: "",
		Horizontal: "-", Vertical: "|",
		HeaderHorizontal: "-",
	}
}

func getBorders(style TableStyle) borders {
	switch style {
	case ASCIIStyle:
		return asciiBorders()
	case PsqlStyle:
		return psqlBorders()
	case UnicodeFullStyle:
		return unicodeFullBorders()
	default:
		return unicodeBorders()
	}
}

// displayWidth returns the display width of a string, accounting for wide characters.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0x303e) || (r >= 0x3040 && r <= 0x33bf) ||
			(r >= 0x3400 && r <= 0x4dbf) || (r >= 0x4e00 && r <= 0xa4cf) ||
			(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
			(r >= 0xfe10 && r <= 0xfe6f) || (r >= 0xff01 && r <= 0xff60) ||
			(r >= 0xffe0 && r <= 0xffe6) ||
			(r >= 0x20000 && r <= 0x2fffd) || (r >= 0x30000 && r <= 0x3fffd)) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// padRight pads a string to the given display width.
func padRight(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

func formatTable(w io.Writer, result *QueryResult, opts Options) error {
	if len(result.Columns) == 0 {
		return nil
	}
	b := getBorders(opts.Style)

	// Calculate column widths
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = displayWidth(col)
	}
	for _, row := range result.Rows {
		for i, cell := range row {
			if i < len(widths) {
				cw := displayWidth(cell)
				if cw > widths[i] {
					widths[i] = cw
				}
			}
		}
	}

	// Helper to write a border line
	writeBorderLine := func(left, mid, right, horiz string) {
		fmt.Fprint(w, left)
		for i, width := range widths {
			fmt.Fprint(w, strings.Repeat(horiz, width+2))
			if i < len(widths)-1 {
				fmt.Fprint(w, mid)
			}
		}
		fmt.Fprintln(w, right)
	}

	// Top border (skip if empty, e.g. psql style)
	if b.TopLeft != "" {
		writeBorderLine(b.TopLeft, b.TopMid, b.TopRight, b.Horizontal)
	}

	// Header
	fmt.Fprint(w, b.Vertical)
	for i, col := range result.Columns {
		fmt.Fprintf(w, " %s ", padRight(col, widths[i]))
		fmt.Fprint(w, b.Vertical)
	}
	fmt.Fprintln(w)

	// Header separator
	writeBorderLine(b.MidLeft, b.MidMid, b.MidRight, b.HeaderHorizontal)

	// Data rows
	for _, row := range result.Rows {
		fmt.Fprint(w, b.Vertical)
		for i := range result.Columns {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			fmt.Fprintf(w, " %s ", padRight(cell, widths[i]))
			fmt.Fprint(w, b.Vertical)
		}
		fmt.Fprintln(w)
	}

	// Bottom border (skip if empty, e.g. psql style)
	if b.BotLeft != "" {
		writeBorderLine(b.BotLeft, b.BotMid, b.BotRight, b.Horizontal)
	}

	return nil
}

func formatVertical(w io.Writer, result *QueryResult, opts Options) error {
	if len(result.Columns) == 0 {
		return nil
	}

	// Find max column name width
	maxWidth := 0
	for _, col := range result.Columns {
		if cw := utf8.RuneCountInString(col); cw > maxWidth {
			maxWidth = cw
		}
	}

	for i, row := range result.Rows {
		fmt.Fprintf(w, "-[ RECORD %d ]%s\n", i+1, strings.Repeat("-", 40))
		for j, col := range result.Columns {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			fmt.Fprintf(w, "%-*s | %s\n", maxWidth, col, cell)
		}
	}

	return nil
}

func formatCSV(w io.Writer, result *QueryResult, delimiter rune) error {
	cw := csv.NewWriter(w)
	cw.Comma = delimiter

	if err := cw.Write(result.Columns); err != nil {
		return err
	}
	for _, row := range result.Rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func formatJSON(w io.Writer, result *QueryResult) error {
	rows := make([]map[string]interface{}, len(result.Rows))
	for i, row := range result.Rows {
		m := make(map[string]interface{})
		for j, col := range result.Columns {
			if j < len(row) {
				m[col] = row[j]
			}
		}
		rows[i] = m
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
