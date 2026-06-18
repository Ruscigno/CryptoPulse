import pandas as pd
from datetime import datetime, timezone, timedelta
from stock_screener import resample


def _bars(start, n, freq_h):
    idx = [start + timedelta(hours=freq_h * i) for i in range(n)]
    return pd.DataFrame(
        {"open": range(1, n + 1), "high": range(10, 10 + n),
         "low": range(-1, -1 - n, -1), "close": range(2, 2 + n), "volume": [100] * n},
        index=pd.DatetimeIndex(idx, name="ts"),
    ).astype(float)


def test_resample_4h():
    start = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    df = _bars(start, 4, 1)
    out = resample.to(df, "4h")
    assert len(out) == 1
    row = out.iloc[0]
    assert row["open"] == 1 and row["close"] == 5 and row["high"] == 13 and row["low"] == -4 and row["volume"] == 400


def test_resample_closed_drops_partial():
    start = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    df = _bars(start, 6, 1)  # 4 fill the 12:00 bucket, 2 start the 16:00 bucket
    out = resample.to_closed(df, "4h")
    assert len(out) == 1


def test_native_returns_empty():
    start = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    df = _bars(start, 4, 1)
    assert resample.to(df, "1d").empty   # 1d is native, not derived
