package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockProviderSource(t *testing.T) (*DBProviderSource, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("open gorm db: %v", err)
	}

	return NewDBProviderSource(gormDB), mock, func() { _ = sqlDB.Close() }
}

func preferenceQueryPattern() string {
	return regexp.QuoteMeta(`SELECT * FROM "system_preferences" WHERE preference_key = $1`)
}

func TestDBProviderSource_GetCurrentProvider(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  Provider
	}{
		{name: "tendar", value: "tendar", want: ProviderTendar},
		{name: "prembly", value: "prembly", want: ProviderPrembly},
		{name: "mixed case is normalised", value: "PremBly", want: ProviderPrembly},
		{name: "surrounding whitespace is trimmed", value: "  prembly  ", want: ProviderPrembly},
		{name: "blank value falls back to tendar", value: "", want: ProviderTendar},
		{name: "unknown value falls back to tendar", value: "acme-kyc", want: ProviderTendar},
		{name: "wallet provider is not an identity provider", value: "optimus", want: ProviderTendar},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source, mock, cleanup := newMockProviderSource(t)
			defer cleanup()

			mock.ExpectQuery(preferenceQueryPattern()).
				WithArgs(validationProviderPreferenceKey, 1).
				WillReturnRows(sqlmock.NewRows([]string{"preference_key", "preference_value"}).
					AddRow(validationProviderPreferenceKey, tc.value))

			got, err := source.GetCurrentProvider(context.Background())
			if err != nil {
				t.Fatalf("GetCurrentProvider returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("provider = %q, want %q", got, tc.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet sqlmock expectations: %v", err)
			}
		})
	}
}

// A fresh database has no system_preferences row — the caller must still get a
// usable provider alongside the error so the fallback never lands on an empty value.
func TestDBProviderSource_GetCurrentProvider_MissingRowDefaultsToTendar(t *testing.T) {
	source, mock, cleanup := newMockProviderSource(t)
	defer cleanup()

	mock.ExpectQuery(preferenceQueryPattern()).
		WithArgs(validationProviderPreferenceKey, 1).
		WillReturnRows(sqlmock.NewRows([]string{"preference_key", "preference_value"}))

	got, err := source.GetCurrentProvider(context.Background())
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected %v, got %v", gorm.ErrRecordNotFound, err)
	}
	if got != ProviderTendar {
		t.Fatalf("provider = %q, want %q", got, ProviderTendar)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
