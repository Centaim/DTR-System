package api

import "testing"

func TestCSVCellNeutralisesFormulas(t *testing.T) {
	cases := map[string]string{
		"Maria Santos":    "Maria Santos",
		`=HYPERLINK("x")`: `'=HYPERLINK("x")`,
		"+1+1":            "'+1+1",
		"-2":              "'-2",
		"@SUM(A1)":        "'@SUM(A1)",
		"":                "",
	}
	for in, want := range cases {
		if got := safeCell(in); got != want {
			t.Errorf("safeCell(%q) = %s, want %s", in, got, want)
		}
	}
}
