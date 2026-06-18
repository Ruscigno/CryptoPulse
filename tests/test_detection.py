import pandas as pd
from stock_screener import detection


def test_smooth_identity_period1():
    s = pd.Series([1.0, 2.0, 3.0])
    assert list(detection.smooth(s, 1)) == [1.0, 2.0, 3.0]


def test_prominence_filters_small_bump():
    # big peak at idx 5 (value 10); tiny bump at idx 9 (1.2 over baseline 1)
    vals = [0, 1, 2, 5, 8, 10, 6, 2, 1, 1.2, 1, 1.2, 1]
    s = pd.Series(vals, dtype=float)
    peaks, _ = detection.find_extrema(s, min_prominence=3.0)
    idxs = [p[0] for p in peaks]
    assert 5 in idxs and 9 not in idxs


def test_valleys():
    vals = [10, 1, 10, -5, 10]
    s = pd.Series(vals, dtype=float)
    _, valleys = detection.find_extrema(s, min_prominence=3.0)
    idxs = [v[0] for v in valleys]
    assert 3 in idxs


def test_offset_with_leading_nan():
    # leading NaN warmup must not break index mapping
    s = pd.Series([float("nan"), float("nan"), 0.0, 5.0, 0.0, 9.0, 0.0])
    peaks, _ = detection.find_extrema(s, min_prominence=0.0)
    idxs = [p[0] for p in peaks]
    assert 3 in idxs and 5 in idxs  # indices in ORIGINAL series space


def test_select_recent_freshest_first_min_distance():
    # candidates ascending by index; min_distance 30, n=3
    pts = [(0, 1.0), (20, 9.0), (35, 2.0), (70, 4.0)]
    sel = detection.select_recent(pts, min_distance=30, n=3)
    # freshest 70 kept; next <=40 -> 35 (70-35=35>=30) kept; 20 dropped (35-20=15<30);
    # next <=5 -> 0 (35-0>=30) kept. Returned ascending (oldest -> newest).
    assert [i for i, _ in sel] == [0, 35, 70]


def test_select_recent_keeps_freshest_not_highest():
    # two close pivots: keep the FRESHER (idx 62, low value), not the higher (idx 60)
    pts = [(60, 5.0), (62, 1.0)]
    sel = detection.select_recent(pts, min_distance=30, n=3)
    assert sel == [(62, 1.0)]


def test_select_recent_limits_to_n():
    pts = [(0, 1.0), (40, 2.0), (80, 3.0), (120, 4.0)]
    sel = detection.select_recent(pts, min_distance=30, n=2)
    assert [i for i, _ in sel] == [80, 120]  # the 2 freshest, 40 apart


def test_select_recent_empty():
    assert detection.select_recent([], min_distance=30, n=3) == []
