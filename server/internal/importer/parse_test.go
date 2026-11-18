package importer

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseCSVWithAliasesAndBOM(t *testing.T) {
	csv := "\xef\xbb\xbfEmployee No.,Name,Designation,Department,Employment Type\r\n" +
		"e-001,Maria Santos,Clerk I,Cash Division,Contract of Service\r\n" +
		"e-002,Juan Dela Cruz,,Records,JO\r\n" +
		",,,,\r\n"
	rows, err := Parse([]byte(csv), "staff.csv")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	if r := rows[0]; r.EmployeeNo != "E-001" || r.Employment != "contract_of_service" || r.Position != "Clerk I" || r.Status != "ok" || r.Line != 2 {
		t.Errorf("row 0: %+v", r)
	}
	if rows[1].Employment != "job_order" {
		t.Errorf("JO should map to job_order, got %q", rows[1].Employment)
	}
}

func TestParseXLSX(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetSheetRow("Sheet1", "A1", &[]string{"Employee Number", "Full Name", "Role"})
	_ = f.SetSheetRow("Sheet1", "A2", &[]string{"X-10", "Ana Reyes", "Supervisor"})
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	rows, err := Parse(buf.Bytes(), "staff.xlsx")
	if err != nil || len(rows) != 1 || rows[0].Role != "supervisor" || rows[0].Status != "ok" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}

func TestParseFlagsBadRows(t *testing.T) {
	csv := "Employee No,Full Name,Role,Employment\n" +
		"A1,Ok Person,,\n" +
		"A1,Same Number,,\n" +
		"A2,,,\n" +
		"A3,Wants Admin,hr_admin,\n" +
		"A4,Odd Type,,Volunteer\n"
	rows, err := Parse([]byte(csv), "x.csv")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ok", "duplicate", "invalid", "invalid", "invalid"}
	for i, w := range want {
		if rows[i].Status != w {
			t.Errorf("line %d: status %q, want %q (%s)", rows[i].Line, rows[i].Status, w, rows[i].Message)
		}
	}
}

func TestParseRejectsMissingColumnsAndOldFormats(t *testing.T) {
	if _, err := Parse([]byte("Name,Office\nA,B\n"), "x.csv"); err == nil {
		t.Error("missing employee number column accepted")
	}
	if _, err := Parse([]byte("Employee No\nA1\n"), "x.csv"); err == nil {
		t.Error("missing full name column accepted")
	}
	if _, err := Parse([]byte("\xd0\xcf\x11\xe0 legacy"), "staff.xls"); err == nil {
		t.Error(".xls accepted")
	}
	if _, err := Parse([]byte(""), "x.csv"); err == nil {
		t.Error("empty file accepted")
	}
}

func TestParseCapsRowCount(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("Employee No,Full Name\n")
	for i := 0; i < MaxRows+1; i++ {
		b.WriteString("N" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + string(rune('A'+(i/676)%26)) + ",Person\n")
	}
	if _, err := Parse(b.Bytes(), "big.csv"); err == nil {
		t.Error("oversized file accepted")
	}
}
