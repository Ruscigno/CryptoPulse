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


def test_resample_3d_epoch_anchored():
    # daily bars; 3d buckets must align to epoch (day - day%3), same as bucket_start
    from stock_screener import timeframes
    start = datetime(2026, 6, 1, tzinfo=timezone.utc)
    idx = [start + timedelta(days=i) for i in range(7)]
    df = pd.DataFrame(
        {"open": range(1, 8), "high": range(10, 17), "low": range(0, 7),
         "close": range(2, 9), "volume": [10] * 7},
        index=pd.DatetimeIndex(idx, name="ts"),
    ).astype(float)
    out = resample.to(df, "3d")
    tf = timeframes.get("3d")
    # every output bucket start must equal bucket_start of its own timestamp (epoch-anchored)
    for ts in out.index:
        assert tf.bucket_start(ts.to_pydatetime()) == ts.to_pydatetime()
    # volumes sum to the total (no bars dropped/duplicated)
    assert out["volume"].sum() == 70


def test_resample_closed_keeps_complete_bucket():
    start = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)  # 4h boundary
    df = _bars(start, 4, 1)   # exactly one full 4h bucket
    assert len(resample.to_closed(df, "4h")) == 1
