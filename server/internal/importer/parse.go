// Package importer reads employee spreadsheets (.xlsx or .csv) and creates accounts.
package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

const MaxRows = 500

type Row struct {
	Line       int    `json:"line"`
	EmployeeNo string `json:"employee_no"`
	FullName   string `json:"full_name"`
	Position   string `json:"position"`
	Office     string `json:"office"`
	Role       string `json:"role"`
	Employment string `json:"employment"`
	Status     string `json:"status"` // ok, duplicate, invalid
	Message    string `json:"message"`
}

var ErrFile = errors.New("file")

func fileErr(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrFile, fmt.Sprintf(format, a...))
}

// Message strips the sentinel prefix so the text can be shown to a person.
func Message(err error) string { return strings.TrimPrefix(err.Error(), "file: ") }

// Imports can only create ordinary accounts. HR and super admin accounts are
// made one at a time by a super admin.
var allowedRoles = map[string]bool{"employee": true, "supervisor": true, "timekeeper": true, "auditor": true}

var employmentNames = map[string]string{
	"plantilla": "plantilla", "regular": "plantilla", "permanent": "plantilla",
	"casual":            "casual",
	"contractofservice": "contract_of_service", "cos": "contract_of_service", "contractual": "contract_of_service",
	"joborder": "job_order", "jo": "job_order",
}

var headerAliases = map[string]string{
	"employeeno": "employee_no", "employeenumber": "employee_no", "empno": "employee_no", "id": "employee_no", "idno": "employee_no",
	"fullname": "full_name", "name": "full_name", "employeename": "full_name",
	"position": "position", "designation": "position", "title": "position", "positiontitle": "position",
	"office": "office", "department": "office", "division": "office", "unit": "office", "section": "office",
	"role": "role", "employment": "employment", "employmenttype": "employment", "employmentstatus": "employment", "status": "employment",
}

func squash(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// readGrid returns the first sheet or the CSV as rows of cells.
func readGrid(data []byte, filename string) ([][]string, error) {
	if bytes.HasPrefix(data, []byte("PK")) { // .xlsx is a zip archive
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			return nil, fileErr("the spreadsheet could not be opened. Save it as .xlsx or .csv and try again")
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, fileErr("the spreadsheet has no sheets")
		}
		grid, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, fileErr("the first sheet could not be read")
		}
		return grid, nil
	}
	if strings.HasSuffix(strings.ToLower(filename), ".xls") {
		return nil, fileErr("old .xls files are not supported. Save the file as .xlsx or .csv")
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var grid [][]string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fileErr("the CSV could not be read near line %d", len(grid)+1)
		}
		grid = append(grid, rec)
	}
	return grid, nil
}

// Parse reads and validates a file without touching the database.
func Parse(data []byte, filename string) ([]Row, error) {
	grid, err := readGrid(data, filename)
	if err != nil {
		return nil, err
	}
	if len(grid) == 0 {
		return nil, fileErr("the file is empty")
	}

	cols := map[string]int{}
	for i, h := range grid[0] {
		if key, ok := headerAliases[squash(h)]; ok {
			if _, dup := cols[key]; !dup {
				cols[key] = i
			}
		}
	}
	if _, ok := cols["employee_no"]; !ok {
		return nil, fileErr("add a column headed Employee No")
	}
	if _, ok := cols["full_name"]; !ok {
		return nil, fileErr("add a column headed Full Name")
	}

	cell := func(rec []string, key string) string {
		i, ok := cols[key]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	var rows []Row
	seen := map[string]int{}
	for i, rec := range grid[1:] {
		line := i + 2
		empty := true
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}
		if len(rows) >= MaxRows {
			return nil, fileErr("a file can hold %d employees at most. Split it and import in parts", MaxRows)
		}

		r := Row{Line: line, Status: "ok",
			EmployeeNo: strings.ToUpper(cell(rec, "employee_no")), FullName: cell(rec, "full_name"),
			Position: cell(rec, "position"), Office: cell(rec, "office"), Role: "employee", Employment: "plantilla"}

		if v := strings.ToLower(cell(rec, "role")); v != "" {
			r.Role = strings.ReplaceAll(v, " ", "_")
		}
		if v := cell(rec, "employment"); v != "" {
			if e, ok := employmentNames[squash(v)]; ok {
				r.Employment = e
			} else {
				r.Employment = v
			}
		}

		switch {
		case r.EmployeeNo == "" || len(r.EmployeeNo) > 30:
			r.invalid("employee number is required (30 characters at most)")
		case r.FullName == "" || len(r.FullName) > 120:
			r.invalid("full name is required (120 characters at most)")
		case len(r.Position) > 80 || len(r.Office) > 80:
			r.invalid("position and office can be 80 characters at most")
		case !allowedRoles[r.Role]:
			r.invalid("role must be employee, supervisor, timekeeper or auditor")
		case r.Employment != "plantilla" && r.Employment != "casual" && r.Employment != "contract_of_service" && r.Employment != "job_order":
			r.invalid("employment type must be plantilla, casual, contract of service or job order")
		}
		if r.Status == "ok" {
			if first, dup := seen[r.EmployeeNo]; dup {
				r.Status, r.Message = "duplicate", fmt.Sprintf("same employee number as line %d", first)
			} else {
				seen[r.EmployeeNo] = line
			}
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil, fileErr("no employees were found under the headings")
	}
	return rows, nil
}

func (r *Row) invalid(msg string) { r.Status, r.Message = "invalid", msg }
