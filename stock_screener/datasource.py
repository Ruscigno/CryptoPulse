from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pandas as pd
import yfinance as yf

_COLS = ["open", "high", "low", "close", "volume"]


def normalize(
    raw: pd.DataFrame,
    bar_seconds: int,
    now: datetime,
    closed_only: bool,
) -> pd.DataFrame:
    if raw is None or raw.empty:
        return pd.DataFrame(columns=_COLS)
    df = raw.rename(columns=str.lower)
    df = df[[c for c in _COLS if c in df.columns]].copy()
    df.index = pd.to_datetime(df.index, utc=True)
    df.index.name = "ts"
    df = df.dropna(subset=["open", "high", "low", "close"])
    if closed_only and len(df):
        cutoff = pd.Timestamp(now) - timedelta(seconds=bar_seconds)
        df = df[df.index <= cutoff]
    return df[_COLS]


def fetch(
    symbol: str,
    interval: str,
    bar_seconds: int,
    start: datetime | None,
    closed_only: bool,
) -> pd.DataFrame:
    kwargs: dict = {"interval": interval, "auto_adjust": True}
    if start is not None:
        kwargs["start"] = start
    else:
        kwargs["period"] = "max"
    raw = yf.Ticker(symbol).history(**kwargs)
    return normalize(raw, bar_seconds, datetime.now(timezone.utc), closed_only)
