import os
import pytest
import pandas as pd
from datetime import datetime, timezone
from sqlalchemy import text
from stock_screener.storage import Store


def _store():
    dsn = os.getenv("SCREENER_TEST_DSN")
    if not dsn:
        pytest.skip("set SCREENER_TEST_DSN to run storage integration tests")
    s = Store(dsn)
    s.migrate()
    with s.engine.begin() as c:
        c.execute(text("DELETE FROM bars WHERE symbol='TST'"))
    return s


def test_upsert_and_get():
    s = _store()
    t0 = datetime(2026, 6, 16, tzinfo=timezone.utc)
    t1 = datetime(2026, 6, 17, tzinfo=timezone.utc)
    df = pd.DataFrame(
        {"open": [1.0, 1.5], "high": [2.0, 3.0], "low": [0.5, 1.0],
         "close": [1.5, 2.5], "volume": [100.0, 200.0]},
        index=pd.DatetimeIndex([t0, t1], name="ts"),
    )
    s.upsert_bars("TST", "1d", df)
    df.loc[t1, "close"] = 2.7
    s.upsert_bars("TST", "1d", df)            # idempotent update, no duplicate
    got = s.get_bars("TST", "1d")
    assert len(got) == 2
    assert got.index[0] < got.index[1]  # ascending by ts
    assert float(got["close"].iloc[-1]) == 2.7
    assert s.last_bar_time("TST", "1d") == t1
