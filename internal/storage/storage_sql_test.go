package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// These tests exercise the SQL-bearing methods without a real database, so the
// query/transaction logic is covered in CI even when SCREENER_TEST_DSN is unset.

func TestUpsertBarsSQL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	s := &PostgresStore{db: db}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO bars")
	mock.ExpectExec("INSERT INTO bars").
		WithArgs("AAA", "1d", sqlmock.AnyArg(), 1.0, 2.0, 0.5, 1.5, 100.0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = s.UpsertBars(context.Background(), []Bar{
		{Symbol: "AAA", Timeframe: "1d", Time: time.Now(), Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 100},
	})
	if err != nil {
		t.Fatalf("UpsertBars: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpsertBarsEmptyIsNoop(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{db: db}
	// No expectations set: an empty batch must not touch the DB.
	if err := s.UpsertBars(context.Background(), nil); err != nil {
		t.Fatalf("UpsertBars(nil): %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB calls: %v", err)
	}
}

func TestGetBarsReturnsAscendingAndUsesLimit(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{db: db}

	newer := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// The query orders DESC, so rows arrive newest-first; GetBars must reverse.
	rows := sqlmock.NewRows([]string{"symbol", "timeframe", "ts", "open", "high", "low", "close", "volume"}).
		AddRow("AAA", "1d", newer, 2.0, 3.0, 1.0, 2.5, 200.0).
		AddRow("AAA", "1d", older, 1.0, 2.0, 0.5, 1.5, 100.0)
	mock.ExpectQuery("SELECT .+ FROM bars WHERE symbol = .+ ORDER BY ts DESC LIMIT").
		WithArgs("AAA", "1d", 5).
		WillReturnRows(rows)

	got, err := s.GetBars(context.Background(), "AAA", "1d", 5)
	if err != nil {
		t.Fatalf("GetBars: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if !got[0].Time.Equal(older) || !got[1].Time.Equal(newer) {
		t.Errorf("not ascending: got[0]=%v got[1]=%v", got[0].Time, got[1].Time)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestLastBarTime(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{db: db}

	ts := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT ts FROM bars").
		WithArgs("AAA", "1d").
		WillReturnRows(sqlmock.NewRows([]string{"ts"}).AddRow(ts))
	got, ok, err := s.LastBarTime(context.Background(), "AAA", "1d")
	if err != nil || !ok {
		t.Fatalf("LastBarTime: ok=%v err=%v", ok, err)
	}
	if !got.Equal(ts) {
		t.Errorf("ts = %v, want %v", got, ts)
	}

	// No rows -> ok=false, no error.
	mock.ExpectQuery("SELECT ts FROM bars").
		WithArgs("ZZZ", "1d").
		WillReturnRows(sqlmock.NewRows([]string{"ts"}))
	_, ok, err = s.LastBarTime(context.Background(), "ZZZ", "1d")
	if err != nil || ok {
		t.Errorf("no-rows: ok=%v err=%v, want false/nil", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
