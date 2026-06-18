import pandas as pd
from stock_screener import detection

def test_smooth_identity_period1():
    s = pd.Series([1.0, 2.0, 3.0])
    assert list(detection.smooth(s, 1)) == [1.0, 2.0, 3.0]

def test_prominence_filters_small_bump():
    # big peak at idx 5 (value 10); tiny bump at idx 9 (1.2 over baseline 1)
    vals = [0, 1, 2, 5, 8, 10, 6, 2, 1, 1.2, 1, 1.2, 1]
    s = pd.Series(vals, dtype=float)
    peaks, _ = detection.find_extrema(s, min_prominence=3.0, min_distance=1)
    idxs = [p[0] for p in peaks]
    assert 5 in idxs and 9 not in idxs

def test_distance_keeps_higher_of_two_close():
    vals = [0, 5, 0, 9, 0]  # peaks at idx 1 (5) and 3 (9), 2 apart
    s = pd.Series(vals, dtype=float)
    peaks, _ = detection.find_extrema(s, min_prominence=0.0, min_distance=3)
    idxs = [p[0] for p in peaks]
    assert idxs == [3]  # only the higher within distance 3

def test_valleys():
    vals = [10, 1, 10, -5, 10]
    s = pd.Series(vals, dtype=float)
    _, valleys = detection.find_extrema(s, min_prominence=3.0, min_distance=1)
    idxs = [v[0] for v in valleys]
    assert 3 in idxs

def test_offset_with_leading_nan():
    # leading NaN warmup must not break index mapping
    s = pd.Series([float("nan"), float("nan"), 0.0, 5.0, 0.0, 9.0, 0.0])
    peaks, _ = detection.find_extrema(s, min_prominence=0.0, min_distance=1)
    idxs = [p[0] for p in peaks]
    assert 3 in idxs and 5 in idxs  # indices in ORIGINAL series space

def test_last_n():
    pts = [(1, 1.0), (2, 2.0), (3, 3.0), (4, 4.0)]
    assert detection.last_n(pts, 2) == [(3, 3.0), (4, 4.0)]
    assert detection.last_n(pts, 10) == pts
