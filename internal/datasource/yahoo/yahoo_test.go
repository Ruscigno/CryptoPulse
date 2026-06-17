package yahoo

import (
	"os"
	"testing"
)

func TestParseChart(t *testing.T) {
	raw, err := os.ReadFile("testdata/aapl_1d.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	candles, err := parseChart(raw)
	if err != nil {
		t.Fatalf("parseChart: %v", err)
	}
	// The middle row with null open is dropped -> 2 valid candles.
	if len(candles) != 2 {
		t.Fatalf("len = %d, want 2 (null row skipped)", len(candles))
	}
	if candles[0].Close != 105.0 || candles[0].Volume != 1000 {
		t.Errorf("candle0 = %+v", candles[0])
	}
	if candles[1].Close != 106.0 {
		t.Errorf("candle1 close = %v, want 106", candles[1].Close)
	}
	if candles[0].Time.Unix() != 1718236800 {
		t.Errorf("candle0 time = %v", candles[0].Time.Unix())
	}
}

func TestParseChartError(t *testing.T) {
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"description":"Not Found"}}}`))
	if err == nil {
		t.Fatal("expected error for empty result")
	}
}
