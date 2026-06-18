import pandas as pd
from datetime import datetime, timezone
from stock_screener.config import Config
from stock_screener.collector import native_timeframes, Collector

def test_native_timeframes():
    assert set(native_timeframes(["15m", "4h", "1d", "3d", "1h"])) == {"15m", "1d", "1h"}

def test_collect_once_upserts():
    cfg = Config(stocks=["AAA"], timeframes=["1d"])
    upserts = []
    def fake_fetch(symbol, interval, bar_seconds, start, closed_only):
        idx = pd.DatetimeIndex([datetime(2026, 1, 1, tzinfo=timezone.utc)], name="ts")
        return pd.DataFrame({"open": [1.0], "high": [2.0], "low": [0.0],
                             "close": [1.5], "volume": [9.0]}, index=idx)
    class FakeStore:
        def last_bar_time(self, *a): return None
        def upsert_bars(self, s, tf, df): upserts.append((s, tf, len(df)))
    errs = Collector(FakeStore(), cfg, fetch=fake_fetch).collect_once()
    assert errs == []
    assert upserts == [("AAA", "1d", 1)]

def test_collect_once_collects_errors_and_continues():
    cfg = Config(stocks=["AAA"], timeframes=["1d", "1h"])
    def boom_fetch(*a, **k): raise RuntimeError("boom")
    class FakeStore:
        def last_bar_time(self, *a): return None
        def upsert_bars(self, *a): pass
    errs = Collector(FakeStore(), cfg, fetch=boom_fetch).collect_once()
    assert len(errs) == 2  # one per native timeframe, loop continued

def test_collect_passes_watermark_as_start():
    cfg = Config(stocks=["AAA"], timeframes=["1d"])
    at = datetime(2026, 6, 10, tzinfo=timezone.utc)
    seen = {}
    def fake_fetch(symbol, interval, bar_seconds, start, closed_only):
        seen["start"] = start
        return pd.DataFrame(columns=["open","high","low","close","volume"])
    class FakeStore:
        def last_bar_time(self, *a): return at
        def upsert_bars(self, *a): pass
    Collector(FakeStore(), cfg, fetch=fake_fetch).collect_once()
    assert seen["start"] == at
