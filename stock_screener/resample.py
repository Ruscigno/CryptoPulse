from __future__ import annotations
import pandas as pd
from stock_screener import timeframes

def to(df: pd.DataFrame, target_tf: str) -> pd.DataFrame:
    """Aggregate native bars into a derived timeframe, bucketed by fixed UTC
    boundaries (timeframes.TF.bucket_start) — consistent for 4h and 3d."""
    tf = timeframes.get(target_tf)
    if tf is None or tf.native or df.empty:
        return df.iloc[0:0]
    keys = df.index.map(tf.bucket_start)
    agg = df.groupby(keys).agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    )
    agg.index.name = "ts"
    return agg.sort_index()


def to_closed(df: pd.DataFrame, target_tf: str) -> pd.DataFrame:
    """Like to(), but drop the final bucket if it has fewer than group_size
    parent bars (still forming)."""
    tf = timeframes.get(target_tf)
    full = to(df, target_tf)
    if tf is None or full.empty:
        return full
    last_start = full.index[-1]
    n = int((df.index.map(tf.bucket_start) == last_start).sum())
    return full.iloc[:-1] if n < tf.group_size else full
