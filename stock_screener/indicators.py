from __future__ import annotations

import pandas as pd


def rsi(close: pd.Series, length: int) -> pd.Series:
    delta = close.diff()
    gain = delta.clip(lower=0.0)
    loss = (-delta).clip(lower=0.0)
    avg_gain = gain.ewm(alpha=1 / length, min_periods=length, adjust=False).mean()
    avg_loss = loss.ewm(alpha=1 / length, min_periods=length, adjust=False).mean()
    rs = avg_gain / avg_loss
    out = 100 - 100 / (1 + rs)
    # avg_loss == 0 (and defined) -> RSI is 100 (all gains, no losses)
    out = out.mask((avg_loss == 0) & avg_loss.notna(), 100.0)
    # avg_gain == 0 with positive loss -> RSI is 0
    out = out.mask((avg_gain == 0) & (avg_loss != 0), 0.0)
    # keep warmup positions as NaN
    out = out.mask(avg_gain.isna() | avg_loss.isna(), float("nan"))
    return out.astype(float)


def _ema(s: pd.Series, length: int) -> pd.Series:
    return s.ewm(span=length, min_periods=length, adjust=False).mean()


def volume_oscillator(volume: pd.Series, short: int, long: int) -> pd.Series:
    es, el = _ema(volume, short), _ema(volume, long)
    out = (es - el) / el * 100
    return out.where(el.notna() & (el != 0))


def distance_from_ma(close: pd.Series, ma_type: str, length: int) -> pd.Series:
    if ma_type.upper() == "SMA":
        ma = close.rolling(length).mean()
    else:
        ma = _ema(close, length)
    out = (close - ma) / ma * 100
    return out.where(ma.notna() & (ma != 0))
