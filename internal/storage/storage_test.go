package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) *PostgresStore {
	dsn := os.Getenv("SCREENER_TEST_DSN")
	if dsn == "" {
		t.Skip("set SCREENER_TEST_DSN to run storage integration tests")
	}
	s, err := NewPostgresStore(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM bars WHERE symbol = 'TST'")
	return s
}

func TestUpsertAndGet(t *testing.T) {
	s := testStore(t)
	defer s.Close()
	ctx := context.Background()
	t0 := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	bars := []Bar{
		{Symbol: "TST", Timeframe: "1d", Time: t0, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 100},
		{Symbol: "TST", Timeframe: "1d", Time: t0.AddDate(0, 0, 1), Open: 1.5, High: 3, Low: 1, Close: 2.5, Volume: 200},
	}
	if err := s.UpsertBars(ctx, bars); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	bars[1].Close = 2.7
	if err := s.UpsertBars(ctx, bars); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err := s.GetBars(ctx, "TST", "1d", 0)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (no duplicates)", len(got))
	}
	if got[0].Time.After(got[1].Time) {
		t.Error("bars must be ascending by time")
	}
	if got[1].Close != 2.7 {
		t.Errorf("close = %v, want 2.7 (upsert update)", got[1].Close)
	}
	last, ok, err := s.LastBarTime(ctx, "TST", "1d")
	if err != nil || !ok {
		t.Fatalf("LastBarTime: ok=%v err=%v", ok, err)
	}
	if !last.Equal(t0.AddDate(0, 0, 1)) {
		t.Errorf("last = %v, want %v", last, t0.AddDate(0, 0, 1))
	}
}
