package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/pg"
)

func TestRenderCSV(t *testing.T) {
	body, err := renderCSV(sampleExportRows())
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "id,category,empty") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "1,books,") {
		t.Fatalf("missing row/null handling: %q", got)
	}
}

func TestRenderJSON(t *testing.T) {
	body, err := renderJSON(sampleExportRows())
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["category"] != "books" || rows[0]["empty"] != nil {
		t.Fatalf("rows: %#v", rows)
	}
}

func TestRenderXLSXProducesZipWorkbook(t *testing.T) {
	body, err := renderXLSX(sampleExportRows())
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	foundSheet := false
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			foundSheet = true
			break
		}
	}
	if !foundSheet {
		t.Fatalf("sheet not found in xlsx files: %+v", zr.File)
	}
}

func sampleExportRows() *pg.ExportRowsResult {
	return &pg.ExportRowsResult{
		Columns: []pg.ColumnRef{
			{Name: "id", TypeName: "int8"},
			{Name: "category", TypeName: "text"},
			{Name: "empty", TypeName: "text"},
		},
		Rows: [][]any{{int64(1), "books", nil}},
	}
}
