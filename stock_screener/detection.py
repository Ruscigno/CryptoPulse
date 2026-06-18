from __future__ import annotations
import numpy as np
import pandas as pd
from scipy.signal import find_peaks


def smooth(series: pd.Series, period: int) -> pd.Series:
    if period <= 1:
        return series
    return series.ewm(span=period, min_periods=period, adjust=False).mean()


def find_extrema(series: pd.Series, min_prominence: float):
    """Return (peaks, valleys) as lists of (index, value) in the ORIGINAL series
    index space — ALL prominence-qualified local maxima/minima, ascending by
    index. No min-distance filtering here: separation is applied recency-first by
    `select_recent` (scipy's `distance` keeps the *highest* peak in a cluster, but
    we want the *freshest*)."""
    values = series.to_numpy(dtype=float)
    valid = ~np.isnan(values)
    if not valid.any():
        return [], []
    offset = int(np.argmax(valid))
    v = values[offset:]
    kw = {}
    if min_prominence > 0:
        kw["prominence"] = float(min_prominence)
    peak_idx, _ = find_peaks(v, **kw)
    valley_idx, _ = find_peaks(-v, **kw)
    peaks = [(int(i + offset), float(v[i])) for i in peak_idx]
    valleys = [(int(i + offset), float(v[i])) for i in valley_idx]
    return peaks, valleys


def select_recent(points, min_distance: int, n: int):
    """Pick up to `n` pivots walking from the freshest (highest index) to the
    oldest, keeping a pivot only if it is at least `min_distance` bars older than
    the previously kept one. The freshest pivot is always taken as the anchor
    (so the gap between it and the current bar is unconstrained). Returns the
    kept pivots in ascending index order (oldest -> newest)."""
    selected = []
    last_idx = None
    for idx, val in reversed(points):  # freshest first
        if last_idx is None or last_idx - idx >= min_distance:
            selected.append((idx, val))
            last_idx = idx
            if len(selected) >= n:
                break
    selected.reverse()
    return selected
