from __future__ import annotations
import pandas as pd
from stock_screener import timeframes

_PANDAS_RULE = {"4h": "4h", "3d": "3D"}


def to(df: pd.DataFrame, target_tf: str) -> pd.DataFrame:
    tf = timeframes.get(target_tf)
    if tf is None or tf.native or df.empty:
        return df.iloc[0:0]
    rule = _PANDAS_RULE[target_tf]
    agg = df.resample(rule, label="left", closed="left", origin="epoch").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    )
    return agg.dropna(subset=["open", "close"])


def to_closed(df: pd.DataFrame, target_tf: str) -> pd.DataFrame:
    tf = timeframes.get(target_tf)
    full = to(df, target_tf)
    if tf is None or full.empty:
        return full
    last_start = full.index[-1]
    n = int((df.index >= last_start).sum())
    return full.iloc[:-1] if n < tf.group_size else full
