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

def test_trend_rising_falling_flat():
    import pandas as pd
    from stock_screener.screener import Screener
    from stock_screener.config import Config
    cfg = Config(stocks=["AAA"], timeframes=["1d"])
    cfg.screening.trend_lookback = 1
    cfg.screening.trend_flat_epsilon = 0.0
    s = Screener(store=None, cfg=cfg)
    assert s._trend(pd.Series([1.0, 2.0, 3.0]), 2) == "rising"
    assert s._trend(pd.Series([3.0, 2.0, 1.0]), 2) == "falling"
    assert s._trend(pd.Series([5.0, 5.0]), 1) == "flat"
    # epsilon dead-band: small change within epsilon is flat
    cfg.screening.trend_flat_epsilon = 0.5
    assert s._trend(pd.Series([100.0, 100.3]), 1) == "flat"
    assert s._trend(pd.Series([100.0, 101.0]), 1) == "rising"
    # prev index out of range -> flat
    assert s._trend(pd.Series([1.0, 2.0]), 0) == "flat"


def _distance_cfg():
    cfg = _cfg()
    d = cfg.indicators.distance_from_ma
    d.length = 3
    d.ma_type = "SMA"
    d.detection.smoothing = 1
    d.detection.min_prominence = 0.0
    return cfg

def test_pivot_min_distance_floor():
    # detection.min_distance is floored at MIN_PIVOT_DISTANCE (30 bars).
    from stock_screener.screener import MIN_PIVOT_DISTANCE
    cfg = _cfg()
    s = Screener(store=None, cfg=cfg)
    cfg.indicators.rsi.detection.min_distance = 5
    assert s._eff_distance(cfg.indicators.rsi.detection) == MIN_PIVOT_DISTANCE  # 30
    cfg.indicators.rsi.detection.min_distance = 45
    assert s._eff_distance(cfg.indicators.rsi.detection) == 45  # configurable above the floor

def test_enough_data_guard_fires_below_threshold():
    # ~50 valid points < 30*(peaks_to_show+1)=120 -> guard warns, no pivots emitted.
    cfg = _distance_cfg()
    closes = [10.0 + (i % 5) for i in range(50)]
    s = Screener(FakeStore(_daily(closes)), cfg)
    res = s.screen(["AAA"], ["1d"], "any", ["distance_from_ma"])
    assert any("insufficient_data" in w.message and "valid bars" in w.message for w in res.warnings)
    assert res.rows == []

def test_enough_data_no_warning_when_sufficient():
    import math
    cfg = _distance_cfg()
    closes = [50 + 20 * math.sin(2 * math.pi * i / 40) for i in range(220)]  # 218 valid > 120
    s = Screener(FakeStore(_daily(closes)), cfg)
    res = s.screen(["AAA"], ["1d"], "any", ["distance_from_ma"])
    assert not any("insufficient_data" in w.message for w in res.warnings)
    # within peaks (and within valleys) consecutive pivots must be >= 30 bars apart
    for row in res.rows:
        ir = row.indicators["distance_from_ma"]
        for group in (ir.peaks, ir.valleys):
            times = sorted(p.time for p in group)
            gaps = [(b - a).days for a, b in zip(times, times[1:])]
            assert all(g >= 30 for g in gaps), f"pivots closer than 30 bars: {gaps}"


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


def test_required_bars_warms_long_ema():
    # distance-from-EMA(200) must be warmed with several spans before the
    # analysis window, else the average is freshly-seeded and inaccurate.
    from stock_screener.screener import Screener
    from stock_screener.config import Config
    from stock_screener import timeframes
    cfg = Config(stocks=["A"], timeframes=["1d"])
    cfg.indicators.distance_from_ma.length = 200
    s = Screener(store=None, cfg=cfg)
    need = s._required_bars(timeframes.get("1d"))
    assert need >= 200 * 3 + 50   # ~3 EMA spans of warmup + an analysis window
