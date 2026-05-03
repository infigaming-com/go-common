package archive

import "testing"

func TestPKSelectAndTuple(t *testing.T) {
	tests := []struct {
		name      string
		pkCols    []string
		wantList  string
		wantTuple string
	}{
		{
			name:      "single column",
			pkCols:    []string{"id"},
			wantList:  `"id"`,
			wantTuple: `("id")`,
		},
		{
			name:      "two columns",
			pkCols:    []string{"id", "user_id"},
			wantList:  `"id","user_id"`,
			wantTuple: `("id","user_id")`,
		},
		{
			name:      "three columns",
			pkCols:    []string{"id", "user_id", "settlement_currency"},
			wantList:  `"id","user_id","settlement_currency"`,
			wantTuple: `("id","user_id","settlement_currency")`,
		},
		{
			name:      "embedded quote escaped",
			pkCols:    []string{`weird"name`},
			wantList:  `"weird""name"`,
			wantTuple: `("weird""name")`,
		},
		{
			name:      "empty cols produces empty parens",
			pkCols:    []string{},
			wantList:  ``,
			wantTuple: `()`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotList, gotTuple := PKSelectAndTuple(tt.pkCols)
			if gotList != tt.wantList {
				t.Errorf("list mismatch:\n  got:  %q\n  want: %q", gotList, tt.wantList)
			}
			if gotTuple != tt.wantTuple {
				t.Errorf("tuple mismatch:\n  got:  %q\n  want: %q", gotTuple, tt.wantTuple)
			}
		})
	}
}
