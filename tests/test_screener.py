import pandas as pd
from datetime import datetime, timezone, timedelta
from stock_screener.config import Config
from stock_screener.screener import Screener, aggregate_matches, Result, Row

class FakeStore:
    def __init__(self, df): self._df = df
    def get_bars(self, symbol, timeframe, limit=0): return self._df
    def last_bar_time(self, *a): return None

def _cfg():
    return Config(stocks=["AAA"], timeframes=["1d"])

def _daily(closes):
    t0 = datetime(2026, 1, 1, tzinfo=timezone.utc)
    idx = pd.DatetimeIndex([t0 + timedelta(days=i) for i in range(len(closes))], name="ts")
    return pd.DataFrame({"open": closes, "high": closes, "low": closes,
                         "close": closes, "volume": [100.0] * len(closes)},
                        index=idx).astype(float)

def test_screen_runs_and_shapes():
    closes = [10,12,14,12,10,12,14,16,14,12,10,8,10,12,14,12,10,8,6,8,10,12,14]
    cfg = _cfg()
    d = cfg.indicators.distance_from_ma
    d.length = 3
    d.ma_type = "SMA"
    d.detection.smoothing = 1
    d.detection.min_prominence = 0.0
    d.detection.min_distance = 1
    s = Screener(FakeStore(_daily(closes)), cfg)
    res = s.screen(["AAA"], ["1d"], "any", ["distance_from_ma"])
    assert isinstance(res, Result)
    assert isinstance(res.rows, list) and isinstance(res.warnings, list)
    # there are real distance pivots here; with match=any a row is plausible — assert no crash & types
    for row in res.rows:
        assert row.symbol == "AAA" and row.timeframe == "1d"
        assert "distance_from_ma" in row.indicators

def test_insufficient_data_warns():
    cfg = _cfg()  # rsi length 14 but only 3 bars
    s = Screener(FakeStore(_daily([10.0, 11.0, 12.0])), cfg)
    res = s.screen(["AAA"], ["1d"], "any", ["rsi"])
    assert any("insufficient_data" in w.message for w in res.warnings)
    assert res.rows == []

def test_aggregate_matches():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[
        Row("AAPL","1d",now,200.0,["rsi","volume_oscillator"],{}),
        Row("AAPL","4h",now,200.0,["rsi"],{}),
        Row("MSFT","1d",now,100.0,["distance_from_ma"],{}),
    ])
    out = aggregate_matches(res, ["AAPL","MSFT","TSLA"], ["1d","4h"])
    assert [m["symbol"] for m in out] == ["AAPL","MSFT"]
    assert out[0]["timeframes"] == ["1d","4h"]
    assert out[0]["indicators"] == ["rsi","volume_oscillator"]
    assert out[1]["indicators"] == ["distance_from_ma"]
