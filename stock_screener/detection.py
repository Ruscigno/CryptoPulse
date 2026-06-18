from __future__ import annotations
import numpy as np
import pandas as pd
from scipy.signal import find_peaks

def smooth(series: pd.Series, period: int) -> pd.Series:
    if period <= 1:
        return series
    return series.ewm(span=period, min_periods=period, adjust=False).mean()

def find_extrema(series: pd.Series, min_prominence: float, min_distance: int):
    """Return (peaks, valleys) as lists of (index, value) in the ORIGINAL series
    index space. Detection runs on the non-NaN suffix; min_distance and
    min_prominence (when > 0) filter the peaks."""
    values = series.to_numpy(dtype=float)
    valid = ~np.isnan(values)
    if not valid.any():
        return [], []
    offset = int(np.argmax(valid))
    v = values[offset:]
    kw = {"distance": max(1, int(min_distance))}
    if min_prominence > 0:
        kw["prominence"] = float(min_prominence)
    peak_idx, _ = find_peaks(v, **kw)
    valley_idx, _ = find_peaks(-v, **kw)
    peaks = [(int(i + offset), float(v[i])) for i in peak_idx]
    valleys = [(int(i + offset), float(v[i])) for i in valley_idx]
    return peaks, valleys

def last_n(points, n):
    return points[-n:] if n < len(points) else points
