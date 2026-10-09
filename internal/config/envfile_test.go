package config

import (
	"strings"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	input := `
# comment
APP_ENV=local

HTTP_ADDR=:8080
DATABASE_URL=postgres://u:p@h:5432/db?sslmode=disable&x=y
EMPTY=
`
	got, err := ParseEnvFile(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"APP_ENV":      "local",
		"HTTP_ADDR":    ":8080",
		"DATABASE_URL": "postgres://u:p@h:5432/db?sslmode=disable&x=y",
		"EMPTY":        "",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseEnvFile_RejectsAmbiguousSyntax(t *testing.T) {
	for name, input := range map[string]string{
		"no equals sign": "APP_ENV",
		"export prefix":  "export APP_ENV=local",
		"quoted value":   `APP_ENV="local"`,
		"interpolation":  "DATABASE_URL=$OTHER",
		"duplicate key":  "APP_ENV=local\nAPP_ENV=stage",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseEnvFile(strings.NewReader(input)); err == nil {
				t.Fatalf("expected %q to be rejected", input)
			}
		})
	}
}
