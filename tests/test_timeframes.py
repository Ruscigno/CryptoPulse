from datetime import datetime, timezone
from stock_screener import timeframes


def test_native():
    tf = timeframes.get("1h")
    assert tf and tf.native and tf.yahoo_interval == "1h"


def test_derived():
    tf = timeframes.get("4h")
    assert tf and not tf.native and tf.parent == "1h" and tf.group_size == 4


def test_unknown():
    assert timeframes.get("7m") is None


def test_bucket_start_4h():
    tf = timeframes.get("4h")
    got = tf.bucket_start(datetime(2026, 6, 16, 14, 30, tzinfo=timezone.utc))
    assert got == datetime(2026, 6, 16, 12, 0, tzinfo=timezone.utc)
