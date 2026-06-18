import pandas as pd
from stock_screener import indicators

def test_rsi_all_gains():
    s = pd.Series(range(1, 20), dtype=float)
    r = indicators.rsi(s, 14)
    assert abs(r.dropna().iloc[-1] - 100.0) < 1e-6

def test_rsi_all_losses():
    s = pd.Series(range(20, 1, -1), dtype=float)
    r = indicators.rsi(s, 14)
    assert abs(r.dropna().iloc[-1] - 0.0) < 1e-6

def test_rsi_warmup_nan():
    s = pd.Series(range(1, 20), dtype=float)
    r = indicators.rsi(s, 14)
    assert r.iloc[:14].isna().all()   # first 14 are warmup

def test_volume_oscillator_constant_zero():
    v = pd.Series([100.0] * 12)
    out = indicators.volume_oscillator(v, 5, 10)
    assert abs(out.dropna().iloc[-1]) < 1e-9

def test_distance_constant_zero():
    c = pd.Series([10.0] * 5)
    out = indicators.distance_from_ma(c, "EMA", 3)
    assert abs(out.dropna().iloc[-1]) < 1e-9

def test_distance_sma_known():
    out = indicators.distance_from_ma(pd.Series([10.0, 20.0]), "SMA", 2)
    assert abs(out.iloc[1] - (20 - 15) / 15 * 100) < 1e-9
