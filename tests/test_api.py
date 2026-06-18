from datetime import datetime, timezone
from fastapi.testclient import TestClient
from stock_screener.config import Config
from stock_screener.api import create_app
from stock_screener.screener import Result, Row, IndicatorResult, Pivot

class FakeScreener:
    def __init__(self, res): self._res = res
    def screen(self, symbols, timeframes_, match, indicators_): return self._res

class FakeStore:
    def ping(self): pass

def _cfg():
    return Config(stocks=["AAPL"], timeframes=["1d", "4h"])

def _client(res):
    return TestClient(create_app(FakeScreener(res), FakeStore(), _cfg()))

def test_healthz():
    assert _client(Result()).get("/healthz").status_code == 200

def test_screen_shape():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[Row("AAPL", "1d", now, 200.0, ["rsi"],
        {"rsi": IndicatorResult(28.3, "rising", "low", True, [Pivot(70.0, now)], [])})])
    body = _client(res).get("/screen").json()
    assert body["results"][0]["symbol"] == "AAPL"
    assert body["results"][0]["indicators"]["rsi"]["zone"] == "low"
    assert body["results"][0]["indicators"]["rsi"]["peaks"][0]["value"] == 70.0

def test_matches_shape():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[
        Row("AAPL", "1d", now, 200.0, ["rsi", "volume_oscillator"], {}),
        Row("AAPL", "4h", now, 200.0, ["rsi"], {}),
    ])
    body = _client(res).get("/matches").json()
    assert len(body["matches"]) == 1
    m = body["matches"][0]
    assert m["timeframes"] == ["1d", "4h"]
    assert m["indicators"] == ["rsi", "volume_oscillator"]

def test_validation_400():
    c = _client(Result())
    assert c.get("/matches?timeframes=7m").status_code == 400
    assert c.get("/matches?indicators=bogus").status_code == 400
    assert c.get("/matches?symbols=NOPE").status_code == 400
    assert c.get("/matches?match=min:0").status_code == 400
    assert c.get("/matches?indicators=rsi,rsi").status_code == 400

def test_matches_dedupes_symbols():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[Row("AAPL", "1d", now, 200.0, ["rsi"], {})])
    body = _client(res).get("/matches?symbols=AAPL,AAPL").json()
    assert len(body["matches"]) == 1
    assert body["criteria"]["symbols"] == 1

def test_warnings_passthrough():
    from stock_screener.screener import Warning
    res = Result(warnings=[Warning("TSLA", "1d", "no_data")])
    body = _client(res).get("/screen").json()
    assert body["warnings"][0]["message"] == "no_data"
