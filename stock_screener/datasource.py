from __future__ import annotations

import logging
import time
from datetime import datetime, timedelta, timezone

import pandas as pd
import yfinance as yf

log = logging.getLogger("stock_screener.datasource")

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
    # A NaN volume on an otherwise-valid bar would violate the NOT NULL column
    # and poison the volume oscillator (and suppress nearby pivots). Treat a
    # missing volume as 0, keeping the OHLC bar (matches the Go behavior).
    if "volume" in df.columns:
        df["volume"] = df["volume"].fillna(0.0)
    if closed_only and len(df):
        cutoff = pd.Timestamp(now) - timedelta(seconds=bar_seconds)
        df = df[df.index <= cutoff]
    return df[_COLS]


def _history(
    symbol: str,
    kwargs: dict,
    timeout: int,
    attempts: int,
    base_delay: float,
    sleep=time.sleep,
) -> pd.DataFrame:
    last: Exception | None = None
    for i in range(attempts):
        try:
            return yf.Ticker(symbol).history(timeout=timeout, **kwargs)
        except Exception as e:  # noqa: BLE001 - transient network/HTTP errors are retryable
            last = e
            log.warning("yfinance %s attempt %d/%d failed: %s", symbol, i + 1, attempts, e)
            if i + 1 < attempts:
                sleep(base_delay * (2**i))
    raise last


def fetch(
    symbol: str,
    interval: str,
    bar_seconds: int,
    start: datetime | None,
    closed_only: bool,
    timeout: int = 30,
    attempts: int = 3,
    base_delay: float = 0.5,
) -> pd.DataFrame:
    kwargs: dict = {"interval": interval, "auto_adjust": True}
    if start is not None:
        kwargs["start"] = start
    else:
        kwargs["period"] = "max"
    raw = _history(symbol, kwargs, timeout, attempts, base_delay)
    return normalize(raw, bar_seconds, datetime.now(timezone.utc), closed_only)
