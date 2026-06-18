import pytest
import pydantic
from stock_screener.config import Config, parse_duration, load_config


def test_parse_duration():
    assert parse_duration("15m") == 15 * 60
    assert parse_duration("6h") == 6 * 3600
    assert parse_duration("3mo") == 90 * 24 * 3600
    assert parse_duration("2d") == 2 * 24 * 3600
    assert parse_duration("1wk") == 7 * 24 * 3600


def test_load_valid():
    cfg = load_config("tests/data/valid.yaml")
    assert cfg.server.port == 8090
    assert cfg.stocks == ["AAPL", "MSFT"]
    assert cfg.screening.match == "any"
    assert cfg.indicators.rsi.length == 14
    assert cfg.indicators.rsi.detection.min_prominence == 8


def test_rejects_unknown_timeframe():
    with pytest.raises((ValueError, pydantic.ValidationError)):
        load_config("tests/data/bad_tf.yaml")


def test_rejects_bad_match():
    with pytest.raises((ValueError, pydantic.ValidationError)):
        load_config("tests/data/bad_match.yaml")


def _base():
    return dict(stocks=["AAA"], timeframes=["1d"])


def test_config_rejects_empty_stocks():
    with pytest.raises((ValueError, pydantic.ValidationError)):
        Config(stocks=[], timeframes=["1d"])


def test_config_rejects_bad_detection():
    with pytest.raises((ValueError, pydantic.ValidationError)):
        Config(**_base(), indicators={"rsi": {"detection": {"smoothing": 0}}})
    with pytest.raises((ValueError, pydantic.ValidationError)):
        Config(**_base(), indicators={"rsi": {"detection": {"min_distance": 0}}})


def test_config_rejects_bad_screening():
    with pytest.raises((ValueError, pydantic.ValidationError)):
        Config(**_base(), screening={"peaks_to_show": 0})
    with pytest.raises((ValueError, pydantic.ValidationError)):
        Config(**_base(), screening={"trend_lookback": 0})
