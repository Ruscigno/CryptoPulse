from __future__ import annotations

import re
import yaml
from pydantic import BaseModel, ConfigDict, field_validator
from stock_screener import timeframes

_DUR = re.compile(r"^(\d+)(mo|wk|d|h|m|s|y)$")
_UNIT_SECONDS = {"s": 1, "m": 60, "h": 3600, "d": 86400, "wk": 604800, "mo": 2592000, "y": 31536000}


def parse_duration(s: str) -> int:
    m = _DUR.match(s.strip())
    if not m:
        raise ValueError(f"invalid duration {s!r}")
    return int(m.group(1)) * _UNIT_SECONDS[m.group(2)]


def valid_match(mode: str) -> bool:
    if mode in ("any", "all"):
        return True
    if mode.startswith("min:"):
        try:
            return int(mode[4:]) >= 1
        except ValueError:
            return False
    return False


class Detection(BaseModel):
    model_config = ConfigDict(extra="ignore")

    smoothing: int = 1
    min_prominence: float = 0.0
    min_distance: int = 30

    @field_validator("smoothing")
    @classmethod
    def _sm(cls, v: int) -> int:
        if v < 1:
            raise ValueError("detection.smoothing must be >= 1")
        return v

    @field_validator("min_distance")
    @classmethod
    def _md(cls, v: int) -> int:
        if v < 1:
            raise ValueError("detection.min_distance must be >= 1")
        return v


class RSICfg(BaseModel):
    model_config = ConfigDict(extra="ignore")

    length: int = 14
    source: str = "close"
    detection: Detection = Detection()


class VolOscCfg(BaseModel):
    model_config = ConfigDict(extra="ignore")

    short_length: int = 5
    long_length: int = 10
    detection: Detection = Detection()


class DistanceCfg(BaseModel):
    model_config = ConfigDict(extra="ignore")

    source: str = "close"
    ma_type: str = "EMA"
    length: int = 200
    calculation: str = "percent"
    detection: Detection = Detection()


class Indicators(BaseModel):
    model_config = ConfigDict(extra="ignore")

    rsi: RSICfg = RSICfg()
    volume_oscillator: VolOscCfg = VolOscCfg()
    distance_from_ma: DistanceCfg = DistanceCfg()


class Screening(BaseModel):
    model_config = ConfigDict(extra="ignore")

    match: str = "any"
    trend_lookback: int = 3
    peaks_to_show: int = 3
    peak_lookback: str = "3mo"
    trend_flat_epsilon: float = 0.0


class Refresh(BaseModel):
    model_config = ConfigDict(extra="ignore")

    intraday: str = "15m"
    daily: str = "6h"


class Collector(BaseModel):
    model_config = ConfigDict(extra="ignore")

    enabled: bool = True
    use_closed_bars_only: bool = True
    refresh: Refresh = Refresh()


class Server(BaseModel):
    model_config = ConfigDict(extra="ignore")

    port: int = 8080


class Config(BaseModel):
    model_config = ConfigDict(extra="ignore")

    server: Server = Server()
    collector: Collector = Collector()
    stocks: list[str]
    timeframes: list[str]
    screening: Screening = Screening()
    indicators: Indicators = Indicators()

    @field_validator("stocks", "timeframes")
    @classmethod
    def _nonempty(cls, v: list[str]) -> list[str]:
        if not v:
            raise ValueError("must not be empty")
        return v

    @field_validator("timeframes")
    @classmethod
    def _known_tf(cls, v: list[str]) -> list[str]:
        for tf in v:
            if timeframes.get(tf) is None:
                raise ValueError(f"unknown timeframe {tf!r}")
        return v

    @field_validator("screening")
    @classmethod
    def _screening(cls, s: Screening) -> Screening:
        if not valid_match(s.match):
            raise ValueError(f"invalid match mode {s.match!r}")
        if s.trend_lookback < 1:
            raise ValueError("trend_lookback must be >= 1")
        if s.peaks_to_show < 1:
            raise ValueError("peaks_to_show must be >= 1")
        parse_duration(s.peak_lookback)
        return s


def load_config(path: str) -> Config:
    with open(path) as f:
        raw = yaml.safe_load(f)
    return Config(**raw)
