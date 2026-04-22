package archive

import (
	"strings"
	"testing"
)

func TestSplitSchemaTable(t *testing.T) {
	tests := []struct {
		name       string
		fq         string
		wantSchema string
		wantTable  string
		wantErr    string // substring expected in error message; empty means no error
	}{
		{"schema and table", "user.users", "user", "users", ""},
		{"archived table", "user.archived_users", "user", "archived_users", ""},
		{"no dot falls back to public", "users", "public", "users", ""},

		{"empty rejected", "", "", "", "empty table name"},
		{"leading dot rejected", ".users", "", "", "empty schema"},
		{"trailing dot rejected", "user.", "", "", "empty table"},
		{"multi-dot rejected", "db.user.users", "", "", "more than one dot"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSchema, gotTable, err := SplitSchemaTable(tt.fq)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("SplitSchemaTable(%q) expected error containing %q, got nil", tt.fq, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("SplitSchemaTable(%q) error = %q, want substring %q", tt.fq, err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("SplitSchemaTable(%q) unexpected error: %v", tt.fq, err)
			}
			if gotSchema != tt.wantSchema || gotTable != tt.wantTable {
				t.Errorf("SplitSchemaTable(%q) = (%q, %q), want (%q, %q)",
					tt.fq, gotSchema, gotTable, tt.wantSchema, tt.wantTable)
			}
		})
	}
}

func TestQuoteIdents(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"empty", nil, ""},
		{"empty slice", []string{}, ""},
		{"one", []string{"id"}, `"id"`},
		{"many", []string{"id", "name", "created_at"}, `"id","name","created_at"`},
		{"embedded quote gets doubled", []string{`a"b`}, `"a""b"`},
		{"snake_case preserved", []string{"pwa_logged_in"}, `"pwa_logged_in"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteIdents(tt.input); got != tt.want {
				t.Errorf("QuoteIdents(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrefixIdents(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		input []string
		want  string
	}{
		{"empty", "src", nil, ""},
		{"one", "src", []string{"id"}, `src."id"`},
		{"many", "src", []string{"id", "name"}, `src."id",src."name"`},
		{"different alias", "t", []string{"id"}, `t."id"`},
		{"embedded quote doubled", "src", []string{`a"b`}, `src."a""b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrefixIdents(tt.alias, tt.input); got != tt.want {
				t.Errorf("PrefixIdents(%q, %v) = %q, want %q", tt.alias, tt.input, got, tt.want)
			}
		})
	}
}
