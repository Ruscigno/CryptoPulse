import pandas as pd
from datetime import datetime, timezone
from stock_screener import datasource

def test_normalize_drops_unclosed():
    now = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    idx = pd.DatetimeIndex(
        [now.replace(hour=10), now.replace(hour=11, minute=30)], name="Date")
    raw = pd.DataFrame(
        {"Open": [1.0, 2.0], "High": [1.0, 2.0], "Low": [1.0, 2.0],
         "Close": [1.0, 2.0], "Volume": [10, 20]}, index=idx)
    out = datasource.normalize(raw, bar_seconds=3600, now=now, closed_only=True)
    # 10:00 bar closes 11:00 (<=12:00 keep); 11:30 bar closes 12:30 (>12:00 drop)
    assert len(out) == 1
    assert list(out.columns) == ["open", "high", "low", "close", "volume"]
    assert out.index.name == "ts"

def test_normalize_empty():
    out = datasource.normalize(pd.DataFrame(), bar_seconds=3600,
                               now=datetime(2026, 6, 16, tzinfo=timezone.utc), closed_only=True)
    assert out.empty
    assert list(out.columns) == ["open", "high", "low", "close", "volume"]

def test_normalize_keeps_all_when_not_closed_only():
    now = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    idx = pd.DatetimeIndex([now.replace(hour=10), now.replace(hour=11, minute=30)], name="Date")
    raw = pd.DataFrame({"Open":[1.,2.],"High":[1.,2.],"Low":[1.,2.],"Close":[1.,2.],"Volume":[10,20]}, index=idx)
    out = datasource.normalize(raw, bar_seconds=3600, now=now, closed_only=False)
    assert len(out) == 2

def test_normalize_fills_nan_volume():
    import numpy as np
    now = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    idx = pd.DatetimeIndex([now.replace(hour=9), now.replace(hour=10)], name="Date")
    raw = pd.DataFrame(
        {"Open": [1.0, 2.0], "High": [1.0, 2.0], "Low": [1.0, 2.0],
         "Close": [1.0, 2.0], "Volume": [10.0, np.nan]}, index=idx)
    out = datasource.normalize(raw, bar_seconds=3600, now=now, closed_only=False)
    assert len(out) == 2                 # NaN-volume bar kept (OHLC valid)
    assert out["volume"].iloc[1] == 0.0  # NaN volume -> 0
    assert out["volume"].isna().sum() == 0
