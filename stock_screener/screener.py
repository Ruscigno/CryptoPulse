from __future__ import annotations
from dataclasses import dataclass, field
from datetime import datetime
import pandas as pd
from stock_screener import timeframes, indicators, detection, resample
from stock_screener.config import Config, parse_duration
from stock_screener.rule import classify, qualifies

ALL_INDICATORS = ["rsi", "volume_oscillator", "distance_from_ma"]

# Peaks/valleys must be at least this many bars apart (a hard floor over the
# per-indicator detection.min_distance), so detected pivots are real swings.
MIN_PIVOT_DISTANCE = 30

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

    def _max_smoothing(self) -> int:
        return max(self.cfg.indicators.rsi.detection.smoothing,
                   self.cfg.indicators.volume_oscillator.detection.smoothing,
                   self.cfg.indicators.distance_from_ma.detection.smoothing)

    # EMA convergence factor: a span-N EMA seeded N bars ago still carries ~7%
    # seed weight; ~3 spans drives that below ~0.3%, so warm with 3x the longest
    # period before the analysis window (otherwise e.g. distance-from-EMA200 is
    # computed on a freshly-seeded, inaccurate average).
    _WARMUP_SPANS = 3

    def _eff_distance(self, det) -> int:
        """Per-indicator pivot separation, floored at MIN_PIVOT_DISTANCE."""
        return max(det.min_distance, MIN_PIVOT_DISTANCE)

    def _min_valid_points(self, det) -> int:
        """Valid (post-warmup) points required before we detect pivots: enough to
        hold peaks_to_show pivots spaced at least _eff_distance apart."""
        return self._eff_distance(det) * (self.cfg.screening.peaks_to_show + 1)

    def _required_bars(self, tf) -> int:
        longest = max(self.cfg.indicators.rsi.length,
                      self.cfg.indicators.volume_oscillator.long_length,
                      self.cfg.indicators.distance_from_ma.length)
        # Warmup: enough for the longest MA to converge, plus the smoothing EMA.
        warmup = longest * self._WARMUP_SPANS + self._max_smoothing()
        # Analysis window: at least the peak_lookback scan, and enough valid points
        # to hold well-separated pivots. ADDED to the warmup (not consumed by it).
        pivot_room = max(
            self._min_valid_points(self.cfg.indicators.rsi.detection),
            self._min_valid_points(self.cfg.indicators.volume_oscillator.detection),
            self._min_valid_points(self.cfg.indicators.distance_from_ma.detection),
        )
        lookback_bars = parse_duration(self.cfg.screening.peak_lookback) // max(tf.bar_seconds, 1)
        analysis = max(int(lookback_bars), pivot_room)
        return warmup + analysis

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
            # Enough-data guard: need enough VALID (post-warmup) points to detect
            # pivots that are at least _eff_distance bars apart, else the pivots
            # are unreliable — skip with a warning rather than emit noise.
            need_valid = self._min_valid_points(det)
            if len(valid) < need_valid:
                warns.append(Warning(symbol, tf_name,
                    f"insufficient_data: {ind} needs {need_valid} valid bars, have {len(valid)}"))
                continue
            idx = int(valid.index[-1])
            # All prominence-qualified pivots, then pick the freshest `n` that are
            # >= eff_distance bars apart (recency-first; the gap between the
            # newest pivot and the current bar is intentionally unconstrained).
            eff = self._eff_distance(det)
            n = self.cfg.screening.peaks_to_show
            peaks_all, valleys_all = detection.find_extrema(series, det.min_prominence)
            peaks = detection.select_recent(peaks_all, eff, n)
            valleys = detection.select_recent(valleys_all, eff, n)
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
