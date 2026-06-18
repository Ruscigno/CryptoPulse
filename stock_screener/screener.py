from __future__ import annotations
from dataclasses import dataclass, field
from datetime import datetime
import pandas as pd
from stock_screener import timeframes, indicators, detection, resample
from stock_screener.config import Config, parse_duration
from stock_screener.rule import classify, qualifies

ALL_INDICATORS = ["rsi", "volume_oscillator", "distance_from_ma"]

@dataclass
class Pivot:
    value: float
    time: datetime

@dataclass
class IndicatorResult:
    latest: float
    trend: str
    zone: str
    triggered: bool
    peaks: list[Pivot]
    valleys: list[Pivot]

@dataclass
class Row:
    symbol: str
    timeframe: str
    bar_time: datetime
    price: float
    triggered: list[str]
    indicators: dict[str, IndicatorResult]

@dataclass
class Warning:
    symbol: str
    timeframe: str
    message: str

@dataclass
class Result:
    rows: list[Row] = field(default_factory=list)
    warnings: list[Warning] = field(default_factory=list)

class Screener:
    def __init__(self, store, cfg: Config):
        self.store = store
        self.cfg = cfg

    def screen(self, symbols, timeframes_, match, indicators_) -> Result:
        res = Result()
        for symbol in symbols:
            for tf_name in timeframes_:
                tf = timeframes.get(tf_name)
                if tf is None:
                    res.warnings.append(Warning(symbol, tf_name, "unknown timeframe"))
                    continue
                df = self._load(symbol, tf)
                if df is None or df.empty:
                    res.warnings.append(Warning(symbol, tf_name, "no_data"))
                    continue
                row, warns = self._evaluate(symbol, tf_name, df, match, indicators_)
                res.warnings.extend(warns)
                if row is not None:
                    res.rows.append(row)
        return res

    def _load(self, symbol, tf):
        need = self._required_bars(tf)
        if tf.native:
            return self.store.get_bars(symbol, tf.name, need)
        parent = self.store.get_bars(symbol, tf.parent, need * tf.group_size)
        if parent is None or parent.empty:
            return parent
        if self.cfg.collector.use_closed_bars_only:
            return resample.to_closed(parent, tf.name)
        return resample.to(parent, tf.name)

    def _required_bars(self, tf) -> int:
        longest = max(self.cfg.indicators.rsi.length,
                      self.cfg.indicators.volume_oscillator.long_length,
                      self.cfg.indicators.distance_from_ma.length)
        warmup = longest + 5 * (self.cfg.screening.peaks_to_show + 1) + 50
        lookback_bars = parse_duration(self.cfg.screening.peak_lookback) // max(tf.bar_seconds, 1)
        return max(warmup, int(lookback_bars))

    def _series(self, ind, df):
        if ind == "rsi":
            return indicators.rsi(df["close"], self.cfg.indicators.rsi.length)
        if ind == "volume_oscillator":
            v = self.cfg.indicators.volume_oscillator
            return indicators.volume_oscillator(df["volume"], v.short_length, v.long_length)
        if ind == "distance_from_ma":
            d = self.cfg.indicators.distance_from_ma
            return indicators.distance_from_ma(df["close"], d.ma_type, d.length)
        return None

    def _min_bars(self, ind) -> int:
        if ind == "rsi":
            return self.cfg.indicators.rsi.length + 1
        if ind == "volume_oscillator":
            return self.cfg.indicators.volume_oscillator.long_length
        if ind == "distance_from_ma":
            return self.cfg.indicators.distance_from_ma.length
        return 0

    def _evaluate(self, symbol, tf_name, df, match, indicators_):
        warns: list[Warning] = []
        results: dict[str, IndicatorResult] = {}
        triggered: list[str] = []
        times = list(df.index)
        for ind in indicators_:
            raw = self._series(ind, df)
            if raw is None:
                warns.append(Warning(symbol, tf_name, f"unknown indicator: {ind}"))
                continue
            det = getattr(self.cfg.indicators, ind).detection
            series = detection.smooth(raw.reset_index(drop=True), det.smoothing)
            valid = series.dropna()
            if valid.empty:
                warns.append(Warning(symbol, tf_name,
                    f"insufficient_data: {ind} needs {self._min_bars(ind)} bars, have {len(df)}"))
                continue
            idx = int(valid.index[-1])
            peaks, valleys = detection.find_extrema(series, det.min_prominence, det.min_distance)
            peaks = detection.last_n(peaks, self.cfg.screening.peaks_to_show)
            valleys = detection.last_n(valleys, self.cfg.screening.peaks_to_show)
            zone = classify(float(series.iloc[idx]), peaks, valleys)
            ir = IndicatorResult(
                latest=float(series.iloc[idx]),
                trend=self._trend(series, idx),
                zone=zone,
                triggered=(zone != "neutral"),
                peaks=[Pivot(v, times[i]) for i, v in peaks],
                valleys=[Pivot(v, times[i]) for i, v in valleys],
            )
            results[ind] = ir
            if ir.triggered:
                triggered.append(ind)
        if not qualifies(len(triggered), len(indicators_), match):
            return None, warns
        return (Row(symbol, tf_name, times[-1], float(df["close"].iloc[-1]),
                    triggered, results), warns)

    def _trend(self, series, idx) -> str:
        prev = idx - self.cfg.screening.trend_lookback
        eps = self.cfg.screening.trend_flat_epsilon
        if prev < 0 or pd.isna(series.iloc[prev]):
            return "flat"
        diff = float(series.iloc[idx]) - float(series.iloc[prev])
        if diff > eps:
            return "rising"
        if diff < -eps:
            return "falling"
        return "flat"

def aggregate_matches(res: Result, symbols, timeframes_) -> list[dict]:
    by_sym: dict[str, dict] = {}
    for row in res.rows:
        a = by_sym.setdefault(row.symbol, {"tfs": set(), "inds": set()})
        a["tfs"].add(row.timeframe)
        a["inds"].update(row.triggered)
    out = []
    for sym in symbols:
        a = by_sym.get(sym)
        if not a:
            continue
        tfs = [t for t in timeframes_ if t in a["tfs"]]
        inds = [i for i in ALL_INDICATORS if i in a["inds"]]
        out.append({"symbol": sym, "timeframes": tfs, "indicators": inds})
    return out
