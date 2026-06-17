package resample

import (
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/storage"
)

func TestResample4h(t *testing.T) {
	base := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) // 12:00 -> 12:00 4h bucket
	var in []storage.Bar
	for i := 0; i < 4; i++ {
		in = append(in, storage.Bar{
			Symbol: "X", Timeframe: "1h", Time: base.Add(time.Duration(i) * time.Hour),
			Open: float64(i + 1), High: float64(10 + i), Low: float64(-i), Close: float64(i + 2), Volume: 100,
		})
	}
	out := To(in, "4h")
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 bucket", len(out))
	}
	b := out[0]
	if b.Open != 1 || b.Close != 5 || b.High != 13 || b.Low != -3 || b.Volume != 400 {
		t.Errorf("agg = %+v (open1 close5 high13 low-3 vol400)", b)
	}
	if b.Timeframe != "4h" || !b.Time.Equal(base) {
		t.Errorf("bucket meta = %v %v", b.Timeframe, b.Time)
	}
}

func TestResampleDropsIncompleteTrailingBucket(t *testing.T) {
	base := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	var in []storage.Bar
	for i := 0; i < 6; i++ { // 4 fill the 12:00 bucket, 2 start the 16:00 bucket
		in = append(in, storage.Bar{Symbol: "X", Timeframe: "1h",
			Time: base.Add(time.Duration(i) * time.Hour), Open: 1, High: 1, Low: 1, Close: 1, Volume: 1})
	}
	out := ToClosed(in, "4h")
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (incomplete 16:00 bucket dropped)", len(out))
	}
}
