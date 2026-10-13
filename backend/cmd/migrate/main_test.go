package main

import "testing"

func TestMigrationURLAcceptsOnlyPostgres(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"postgres://user:secret@postgres/db?sslmode=disable", "pgx5://user:secret@postgres/db?sslmode=disable"},
		{"postgresql://postgres/db", "pgx5://postgres/db"},
		{"file:///tmp/data", ""}, {"", ""},
	} {
		got, err := migrationURL(tc.in)
		if tc.want == "" {
			if err == nil {
				t.Fatalf("accepted %q", tc.in)
			}
		} else if err != nil || got != tc.want {
			t.Fatalf("got=%q err=%v", got, err)
		}
	}
}
