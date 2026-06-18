# Stock Screener — Python Rewrite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the stock screener in Python (FastAPI) — feature-for-feature with the Go version — using yfinance, SQLAlchemy Core, pandas/pandas-ta, and `scipy.signal.find_peaks` (prominence + distance) for improved peak/valley detection.

**Architecture:** A `stock_screener/` package: pure-logic modules (`config`, `timeframes`, `indicators`, `detection`, `resample`, `rule`) plus I/O modules (`storage` SQLAlchemy, `datasource` yfinance), orchestrated by `screener`, exposed via `api` (FastAPI) and `cli` (Typer: `collect` / `serve`). Reuses the existing Postgres `screener` DB + `bars` schema and `config.yaml`.

**Tech Stack:** Python 3.12, FastAPI + uvicorn, yfinance, SQLAlchemy Core + psycopg, pandas + pandas-ta, scipy, pydantic, Typer, PyYAML; pytest/ruff/mypy.

**Spec:** `docs/superpowers/specs/2026-06-18-python-rewrite-design.md` (and the Go specs it references). The merged Go code is the behavioral reference.

**Dependency note (pandas-ta + numpy):** pandas-ta ≤0.3.14b uses `numpy.NaN` (removed in numpy 2). Pin `numpy<2` in `pyproject.toml` so pandas-ta imports cleanly; if that causes friction, the RSI fallback (pure-pandas Wilder, shown in Task 4) is acceptable. Decide at Task 0/4.

---

## File structure

```
pyproject.toml                 # deps + tool config (pytest, ruff, mypy)
config.yaml                    # reused; per-indicator `detection` block; `pivot_window` removed
.env.example                   # DB_* vars
stock_screener/
  __init__.py
  config.py        timeframes.py   indicators.py   detection.py
  resample.py      rule.py         storage.py      datasource.py
  screener.py      api.py          collector.py    cli.py
tests/
  test_config.py   test_timeframes.py test_indicators.py test_detection.py
  test_resample.py test_rule.py    test_storage.py test_datasource.py
  test_screener.py test_api.py     test_collector.py
```

Go files (`go.mod`, `go.sum`, `internal/`, `main.go`, `main_test.go`) are removed in Task 13.

---

## Task 0: Project scaffold

**Files:** Create `pyproject.toml`, `stock_screener/__init__.py`, `tests/__init__.py`, `.env.example`.

- [ ] **Step 1: Create `pyproject.toml`**

```toml
[project]
name = "stock-screener"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
    "fastapi>=0.115",
    "uvicorn[standard]>=0.30",
    "yfinance>=0.2.40",
    "sqlalchemy>=2.0",
    "psycopg[binary]>=3.2",
    "pandas>=2.2",
    "pandas-ta>=0.3.14b0",
    "numpy<2",
    "scipy>=1.11",
    "pydantic>=2.7",
    "pyyaml>=6.0",
    "typer>=0.12",
]

[project.optional-dependencies]
dev = ["pytest>=8", "httpx>=0.27", "ruff>=0.6", "mypy>=1.11"]

[project.scripts]
stock-screener = "stock_screener.cli:app"

[tool.pytest.ini_options]
testpaths = ["tests"]

[tool.ruff]
line-length = 100

[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.build_meta"
```

- [ ] **Step 2: Create package + test init files**

`stock_screener/__init__.py`: empty. `tests/__init__.py`: empty.

`.env.example`:
```
DB_USER=screener_app
DB_PASSWORD=change-me
DB_NAME=screener
DB_HOST=localhost
DB_PORT=5433
DB_SSLMODE=disable
```

- [ ] **Step 3: Create venv and install**

Run: `python3.12 -m venv .venv && . .venv/bin/activate && pip install -e '.[dev]'`
Expected: installs cleanly. `python -c "import pandas_ta, scipy.signal, fastapi, sqlalchemy, yfinance"` exits 0.

- [ ] **Step 4: Commit**

```bash
git add pyproject.toml stock_screener/__init__.py tests/__init__.py .env.example
git commit -m "chore: scaffold python package + deps"
```

---

## Task 1: config.py

**Files:** Create `stock_screener/config.py`; Test `tests/test_config.py`.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_config.py
import pytest
from stock_screener.config import parse_duration, load_config

def test_parse_duration():
    assert parse_duration("15m") == 15 * 60
    assert parse_duration("6h") == 6 * 3600
    assert parse_duration("3mo") == 90 * 24 * 3600
    assert parse_duration("2d") == 2 * 24 * 3600
    assert parse_duration("1wk") == 7 * 24 * 3600

def test_load_valid(tmp_path):
    cfg = load_config("tests/data/valid.yaml")
    assert cfg.server.port == 8090
    assert cfg.stocks == ["AAPL", "MSFT"]
    assert cfg.screening.match == "any"
    assert cfg.indicators.rsi.length == 14
    assert cfg.indicators.rsi.detection.min_prominence == 8

def test_rejects_unknown_timeframe(tmp_path):
    with pytest.raises(ValueError):
        load_config("tests/data/bad_tf.yaml")

def test_rejects_bad_match(tmp_path):
    with pytest.raises(ValueError):
        load_config("tests/data/bad_match.yaml")
```

Create `tests/data/valid.yaml`:
```yaml
server: { port: 8090 }
collector: { enabled: true, use_closed_bars_only: true, refresh: { intraday: 15m, daily: 6h } }
stocks: [AAPL, MSFT]
timeframes: [15m, 1h, 4h, 1d]
screening: { match: any, trend_lookback: 3, peaks_to_show: 3, peak_lookback: 3mo, trend_flat_epsilon: 0 }
indicators:
  rsi: { length: 14, source: close, smoothing: { type: SMA, length: 14, bb_stddev: 2 }, detection: { smoothing: 3, min_prominence: 8, min_distance: 5 } }
  volume_oscillator: { short_length: 5, long_length: 10, detection: { smoothing: 3, min_prominence: 5, min_distance: 5 } }
  distance_from_ma: { source: close, ma_type: EMA, length: 200, calculation: percent, detection: { smoothing: 3, min_prominence: 3, min_distance: 5 } }
```
`tests/data/bad_tf.yaml`: copy of valid.yaml with `timeframes: [1day]`.
`tests/data/bad_match.yaml`: copy with `match: nope`.

- [ ] **Step 2: Run → FAIL** (`pytest tests/test_config.py -q` → import error).

- [ ] **Step 3: Implement `stock_screener/config.py`**

```python
from __future__ import annotations
import re
import yaml
from pydantic import BaseModel, field_validator
from stock_screener import timeframes

_DUR = re.compile(r"^(\d+)(mo|wk|d|h|m|s|y)$")
_UNIT_SECONDS = {"s": 1, "m": 60, "h": 3600, "d": 86400, "wk": 604800, "mo": 2592000, "y": 31536000}

def parse_duration(s: str) -> int:
    """Return seconds for values like 15m, 6h, 3mo, 2d, 1wk, 1y."""
    m = _DUR.match(s.strip())
    if not m:
        raise ValueError(f"invalid duration {s!r}")
    return int(m.group(1)) * _UNIT_SECONDS[m.group(2)]

class Detection(BaseModel):
    smoothing: int = 1
    min_prominence: float = 0.0
    min_distance: int = 1

    @field_validator("smoothing")
    @classmethod
    def _sm(cls, v): 
        if v < 1: raise ValueError("detection.smoothing must be >= 1")
        return v

    @field_validator("min_distance")
    @classmethod
    def _md(cls, v):
        if v < 1: raise ValueError("detection.min_distance must be >= 1")
        return v

class RSICfg(BaseModel):
    length: int = 14
    source: str = "close"
    detection: Detection = Detection()

class VolOscCfg(BaseModel):
    short_length: int = 5
    long_length: int = 10
    detection: Detection = Detection()

class DistanceCfg(BaseModel):
    source: str = "close"
    ma_type: str = "EMA"
    length: int = 200
    calculation: str = "percent"
    detection: Detection = Detection()

class Indicators(BaseModel):
    rsi: RSICfg = RSICfg()
    volume_oscillator: VolOscCfg = VolOscCfg()
    distance_from_ma: DistanceCfg = DistanceCfg()

class Screening(BaseModel):
    match: str = "any"
    trend_lookback: int = 3
    peaks_to_show: int = 3
    peak_lookback: str = "3mo"
    trend_flat_epsilon: float = 0.0

class Refresh(BaseModel):
    intraday: str = "15m"
    daily: str = "6h"

class Collector(BaseModel):
    enabled: bool = True
    use_closed_bars_only: bool = True
    refresh: Refresh = Refresh()

class Server(BaseModel):
    port: int = 8080

class Config(BaseModel):
    server: Server = Server()
    collector: Collector = Collector()
    stocks: list[str]
    timeframes: list[str]
    screening: Screening = Screening()
    indicators: Indicators = Indicators()

    @field_validator("stocks", "timeframes")
    @classmethod
    def _nonempty(cls, v):
        if not v: raise ValueError("must not be empty")
        return v

    @field_validator("timeframes")
    @classmethod
    def _known_tf(cls, v):
        for tf in v:
            if timeframes.get(tf) is None:
                raise ValueError(f"unknown timeframe {tf!r}")
        return v

    @field_validator("screening")
    @classmethod
    def _screening(cls, s):
        if not valid_match(s.match):
            raise ValueError(f"invalid match mode {s.match!r}")
        if s.trend_lookback < 1: raise ValueError("trend_lookback must be >= 1")
        if s.peaks_to_show < 1: raise ValueError("peaks_to_show must be >= 1")
        parse_duration(s.peak_lookback)  # validates format
        return s

def valid_match(mode: str) -> bool:
    if mode in ("any", "all"):
        return True
    if mode.startswith("min:"):
        try:
            return int(mode[4:]) >= 1
        except ValueError:
            return False
    return False

def load_config(path: str) -> Config:
    with open(path) as f:
        raw = yaml.safe_load(f)
    return Config(**raw)
```

- [ ] **Step 4: Run → PASS** (`pytest tests/test_config.py -q`). Note: `timeframes.get` comes from Task 2; implement Task 2 first or stub. **Do Task 2 before running this** (config imports timeframes).

- [ ] **Step 5: Commit** (after Task 2 so it imports): defer commit to end of Task 2, or commit config + timeframes together.

---

## Task 2: timeframes.py

**Files:** Create `stock_screener/timeframes.py`; Test `tests/test_timeframes.py`.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_timeframes.py
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
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/timeframes.py`**

```python
from __future__ import annotations
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

@dataclass(frozen=True)
class TF:
    name: str
    native: bool
    yahoo_interval: str        # native only
    parent: str                # derived only
    group_size: int            # derived only
    bar_seconds: int           # approximate bar length

    def bucket_start(self, t: datetime) -> datetime:
        t = t.astimezone(timezone.utc)
        if self.name == "3d":
            day = int(t.timestamp()) // 86400
            start = day - day % 3
            return datetime.fromtimestamp(start * 86400, tz=timezone.utc)
        # truncate to bar_seconds since epoch (4h and native sub-day align cleanly)
        secs = int(t.timestamp())
        return datetime.fromtimestamp(secs - secs % self.bar_seconds, tz=timezone.utc)

_REG = {
    "15m": TF("15m", True, "15m", "", 0, 15 * 60),
    "30m": TF("30m", True, "30m", "", 0, 30 * 60),
    "1h":  TF("1h", True, "1h", "", 0, 3600),
    "4h":  TF("4h", False, "", "1h", 4, 4 * 3600),
    "1d":  TF("1d", True, "1d", "", 0, 86400),
    "3d":  TF("3d", False, "", "1d", 3, 3 * 86400),
    "1wk": TF("1wk", True, "1wk", "", 0, 7 * 86400),
    "1mo": TF("1mo", True, "1mo", "", 0, 30 * 86400),
}

def get(name: str) -> TF | None:
    return _REG.get(name)
```

- [ ] **Step 4: Run → PASS** for both `test_timeframes.py` and `test_config.py`.

- [ ] **Step 5: Commit**
```bash
git add stock_screener/config.py stock_screener/timeframes.py tests/test_config.py tests/test_timeframes.py tests/data/
git commit -m "feat: config + timeframes (pydantic config, TF registry)"
```

---

## Task 3: indicators.py

**Files:** Create `stock_screener/indicators.py`; Test `tests/test_indicators.py`. All functions take/return `pandas.Series` (float), NaN warmup.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_indicators.py
import numpy as np, pandas as pd
from stock_screener import indicators

def test_rsi_all_gains():
    s = pd.Series(range(1, 20), dtype=float)
    r = indicators.rsi(s, 14)
    assert abs(r.dropna().iloc[-1] - 100.0) < 1e-6

def test_rsi_all_losses():
    s = pd.Series(range(20, 1, -1), dtype=float)
    r = indicators.rsi(s, 14)
    assert abs(r.dropna().iloc[-1] - 0.0) < 1e-6

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
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/indicators.py`**

```python
from __future__ import annotations
import pandas as pd

def rsi(close: pd.Series, length: int) -> pd.Series:
    """Wilder's RSI. NaN for the first `length` positions."""
    delta = close.diff()
    gain = delta.clip(lower=0.0)
    loss = (-delta).clip(lower=0.0)
    # Wilder smoothing = EMA with alpha = 1/length
    avg_gain = gain.ewm(alpha=1 / length, min_periods=length, adjust=False).mean()
    avg_loss = loss.ewm(alpha=1 / length, min_periods=length, adjust=False).mean()
    rs = avg_gain / avg_loss
    out = 100 - 100 / (1 + rs)
    out = out.where(avg_loss != 0, 100.0)        # avg_loss == 0 -> 100
    out = out.where(~((avg_gain == 0) & (avg_loss != 0)), 0.0)  # only losses -> 0
    out[avg_gain.isna() | avg_loss.isna()] = pd.NA
    return out.astype(float)

def _ema(s: pd.Series, length: int) -> pd.Series:
    return s.ewm(span=length, min_periods=length, adjust=False).mean()

def volume_oscillator(volume: pd.Series, short: int, long: int) -> pd.Series:
    """100 * (EMA_short - EMA_long) / EMA_long."""
    es, el = _ema(volume, short), _ema(volume, long)
    out = (es - el) / el * 100
    return out.where(el != 0)

def distance_from_ma(close: pd.Series, ma_type: str, length: int) -> pd.Series:
    """(close - MA) / MA * 100. ma_type 'SMA' or 'EMA' (default EMA)."""
    if ma_type.upper() == "SMA":
        ma = close.rolling(length).mean()
    else:
        ma = _ema(close, length)
    out = (close - ma) / ma * 100
    return out.where(ma.notna() & (ma != 0))
```

Note: the implementer should verify `test_rsi_all_gains/losses` pass with this Wilder formulation; if pandas-ta is preferred, `import pandas_ta as ta; ta.rsi(close, length)` is an acceptable substitute (Wilder). Keep whichever makes the extreme-value tests pass.

- [ ] **Step 4: Run → PASS.**

- [ ] **Step 5: Commit**
```bash
git add stock_screener/indicators.py tests/test_indicators.py
git commit -m "feat: indicators (Wilder RSI, volume oscillator, distance-from-MA)"
```

---

## Task 4: detection.py (smoothing + scipy find_peaks)

**Files:** Create `stock_screener/detection.py`; Test `tests/test_detection.py`.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_detection.py
import numpy as np, pandas as pd
from stock_screener import detection

def test_smooth_identity_period1():
    s = pd.Series([1.0, 2.0, 3.0])
    assert list(detection.smooth(s, 1)) == [1.0, 2.0, 3.0]

def test_prominence_filters_small_bump():
    # big peak at idx 5 (value 10), tiny bump at idx 9 (value ~1.2 above baseline 1)
    vals = [0, 1, 2, 5, 8, 10, 6, 2, 1, 1.2, 1, 1.2, 1]
    s = pd.Series(vals, dtype=float)
    peaks, _ = detection.find_extrema(s, min_prominence=3.0, min_distance=1)
    idxs = [p[0] for p in peaks]
    assert 5 in idxs and 9 not in idxs

def test_distance_keeps_higher_of_two_close():
    vals = [0, 5, 0, 9, 0]  # peaks at 1 (5) and 3 (9), 2 apart
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
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/detection.py`**

```python
from __future__ import annotations
import numpy as np
import pandas as pd
from scipy.signal import find_peaks

def smooth(series: pd.Series, period: int) -> pd.Series:
    if period <= 1:
        return series
    return series.ewm(span=period, min_periods=period, adjust=False).mean()

def find_extrema(series: pd.Series, min_prominence: float, min_distance: int):
    """Return (peaks, valleys) as lists of (index, value) over the original series
    index space. Detection runs on the non-NaN suffix."""
    values = series.to_numpy(dtype=float)
    valid = ~np.isnan(values)
    if not valid.any():
        return [], []
    offset = int(np.argmax(valid))          # first non-NaN position
    v = values[offset:]
    kw = dict(distance=max(1, min_distance))
    if min_prominence > 0:
        kw["prominence"] = min_prominence
    peak_idx, _ = find_peaks(v, **kw)
    valley_idx, _ = find_peaks(-v, **kw)
    peaks = [(int(i + offset), float(v[i])) for i in peak_idx]
    valleys = [(int(i + offset), float(v[i])) for i in valley_idx]
    return peaks, valleys

def last_n(points: list[tuple[int, float]], n: int) -> list[tuple[int, float]]:
    return points[-n:] if n < len(points) else points
```

- [ ] **Step 4: Run → PASS.**

- [ ] **Step 5: Commit**
```bash
git add stock_screener/detection.py tests/test_detection.py
git commit -m "feat: detection (ewm smoothing + scipy find_peaks prominence/distance)"
```

---

## Task 5: resample.py

**Files:** Create `stock_screener/resample.py`; Test `tests/test_resample.py`. Operates on a pandas DataFrame indexed by UTC ts with columns open/high/low/close/volume.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_resample.py
import pandas as pd
from datetime import datetime, timezone, timedelta
from stock_screener import resample

def _bars(start, n, freq_h):
    idx = [start + timedelta(hours=freq_h * i) for i in range(n)]
    return pd.DataFrame(
        {"open": range(1, n+1), "high": range(10, 10+n), "low": range(-1, -1-n, -1),
         "close": range(2, 2+n), "volume": [100]*n},
        index=pd.DatetimeIndex(idx, name="ts"),
    ).astype(float)

def test_resample_4h():
    start = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    df = _bars(start, 4, 1)
    out = resample.to(df, "4h")
    assert len(out) == 1
    row = out.iloc[0]
    assert row["open"] == 1 and row["close"] == 5 and row["volume"] == 400

def test_resample_closed_drops_partial():
    start = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    df = _bars(start, 6, 1)  # 4 fill 12:00 bucket, 2 start 16:00
    out = resample.to_closed(df, "4h")
    assert len(out) == 1
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/resample.py`**

```python
from __future__ import annotations
import pandas as pd
from stock_screener import timeframes

_PANDAS_RULE = {"4h": "4h", "3d": "3D"}

def to(df: pd.DataFrame, target_tf: str) -> pd.DataFrame:
    tf = timeframes.get(target_tf)
    if tf is None or tf.native or df.empty:
        return df.iloc[0:0]
    rule = _PANDAS_RULE[target_tf]
    agg = df.resample(rule, label="left", closed="left", origin="epoch").agg(
        {"open": "first", "high": "max", "low": "min", "close": "last", "volume": "sum"}
    )
    return agg.dropna(subset=["open", "close"])

def to_closed(df: pd.DataFrame, target_tf: str) -> pd.DataFrame:
    tf = timeframes.get(target_tf)
    full = to(df, target_tf)
    if tf is None or full.empty:
        return full
    # count parent bars in the last bucket; drop it if < group_size
    last_start = full.index[-1]
    n = int(((df.index >= last_start)).sum())
    return full.iloc[:-1] if n < tf.group_size else full
```

- [ ] **Step 4: Run → PASS.**

- [ ] **Step 5: Commit**
```bash
git add stock_screener/resample.py tests/test_resample.py
git commit -m "feat: resample derived timeframes via pandas"
```

---

## Task 6: rule.py

**Files:** Create `stock_screener/rule.py`; Test `tests/test_rule.py`.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_rule.py
from stock_screener.rule import classify, qualifies

def test_classify():
    peaks = [(1, 70.0), (5, 60.0)]   # min 60
    valleys = [(2, 30.0), (6, 40.0)] # max 40
    assert classify(65, peaks, valleys) == "high"
    assert classify(35, peaks, valleys) == "low"
    assert classify(50, peaks, valleys) == "neutral"

def test_qualifies():
    assert qualifies(1, 3, "any")
    assert not qualifies(0, 3, "any")
    assert qualifies(3, 3, "all")
    assert not qualifies(2, 3, "all")
    assert qualifies(2, 3, "min:2")
    assert not qualifies(1, 3, "min:2")
    assert not qualifies(5, 3, "min:0")
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/rule.py`**

```python
from __future__ import annotations

def classify(current: float, peaks, valleys) -> str:
    """high if current >= min(peak values); low if current <= max(valley values);
    else neutral. High takes precedence (deliberate)."""
    if peaks:
        if current >= min(v for _, v in peaks):
            return "high"
    if valleys:
        if current <= max(v for _, v in valleys):
            return "low"
    return "neutral"

def qualifies(triggered: int, requested: int, match: str) -> bool:
    if match == "any":
        return triggered >= 1
    if match == "all":
        return requested > 0 and triggered == requested
    if match.startswith("min:"):
        try:
            n = int(match[4:])
        except ValueError:
            return False
        return n >= 1 and triggered >= n
    return False
```

- [ ] **Step 4: Run → PASS.**

- [ ] **Step 5: Commit**
```bash
git add stock_screener/rule.py tests/test_rule.py
git commit -m "feat: screening rule (classify zone + qualifies)"
```

---

## Task 7: storage.py (SQLAlchemy Core)

**Files:** Create `stock_screener/storage.py`; Test `tests/test_storage.py` (gated on `SCREENER_TEST_DSN`).

- [ ] **Step 1: Write the gated test**

```python
# tests/test_storage.py
import os, pytest, pandas as pd
from datetime import datetime, timezone
from stock_screener.storage import Store

def store():
    dsn = os.getenv("SCREENER_TEST_DSN")
    if not dsn:
        pytest.skip("set SCREENER_TEST_DSN")
    s = Store(dsn); s.migrate()
    with s.engine.begin() as c:
        from sqlalchemy import text
        c.execute(text("DELETE FROM bars WHERE symbol='TST'"))
    return s

def test_upsert_and_get():
    s = store()
    t0 = datetime(2026, 6, 16, tzinfo=timezone.utc)
    df = pd.DataFrame(
        {"open":[1,1.5],"high":[2,3],"low":[0.5,1],"close":[1.5,2.5],"volume":[100,200]},
        index=pd.DatetimeIndex([t0, t0.replace(day=17)], name="ts"),
    ).astype(float)
    s.upsert_bars("TST", "1d", df)
    df.loc[df.index[1], "close"] = 2.7
    s.upsert_bars("TST", "1d", df)          # idempotent update
    got = s.get_bars("TST", "1d")
    assert len(got) == 2
    assert got["close"].iloc[-1] == 2.7
    assert s.last_bar_time("TST", "1d") == df.index[1]
```

- [ ] **Step 2: Run → FAIL/skip** (compiles, skips without DSN).

- [ ] **Step 3: Implement `stock_screener/storage.py`**

```python
from __future__ import annotations
import os
from datetime import datetime
from urllib.parse import quote
import pandas as pd
from sqlalchemy import (Column, DateTime, Float, MetaData, String, Table,
                        create_engine, select, func, text)
from sqlalchemy.dialects.postgresql import insert

_md = MetaData()
bars = Table(
    "bars", _md,
    Column("symbol", String, primary_key=True),
    Column("timeframe", String, primary_key=True),
    Column("ts", DateTime(timezone=True), primary_key=True),
    Column("open", Float, nullable=False),
    Column("high", Float, nullable=False),
    Column("low", Float, nullable=False),
    Column("close", Float, nullable=False),
    Column("volume", Float, nullable=False),
)

def dsn_from_env() -> str:
    u, p = os.environ["DB_USER"], os.getenv("DB_PASSWORD", "")
    h, port = os.environ["DB_HOST"], os.getenv("DB_PORT", "5432")
    name = os.environ["DB_NAME"]
    sslmode = os.getenv("DB_SSLMODE", "require")
    return f"postgresql+psycopg://{quote(u)}:{quote(p)}@{h}:{port}/{name}?sslmode={sslmode}"

class Store:
    def __init__(self, dsn: str):
        self.engine = create_engine(dsn, pool_size=10, max_overflow=5, pool_pre_ping=True)

    def migrate(self) -> None:
        _md.create_all(self.engine)

    def ping(self) -> None:
        with self.engine.connect() as c:
            c.execute(text("SELECT 1"))

    def upsert_bars(self, symbol: str, timeframe: str, df: pd.DataFrame) -> None:
        if df.empty:
            return
        rows = [
            {"symbol": symbol, "timeframe": timeframe, "ts": ts.to_pydatetime(),
             "open": r.open, "high": r.high, "low": r.low, "close": r.close, "volume": r.volume}
            for ts, r in df.iterrows()
        ]
        stmt = insert(bars).values(rows)
        stmt = stmt.on_conflict_do_update(
            index_elements=["symbol", "timeframe", "ts"],
            set_={c: stmt.excluded[c] for c in ("open", "high", "low", "close", "volume")},
        )
        with self.engine.begin() as c:
            c.execute(stmt)

    def get_bars(self, symbol: str, timeframe: str, limit: int = 0) -> pd.DataFrame:
        q = (select(bars.c.ts, bars.c.open, bars.c.high, bars.c.low, bars.c.close, bars.c.volume)
             .where(bars.c.symbol == symbol, bars.c.timeframe == timeframe)
             .order_by(bars.c.ts.desc()))
        if limit > 0:
            q = q.limit(limit)
        with self.engine.connect() as c:
            df = pd.read_sql(q, c, index_col="ts")
        return df.iloc[::-1]  # ascending

    def last_bar_time(self, symbol: str, timeframe: str):
        q = select(func.max(bars.c.ts)).where(
            bars.c.symbol == symbol, bars.c.timeframe == timeframe)
        with self.engine.connect() as c:
            return c.execute(q).scalar()
```

- [ ] **Step 4: Run with DSN → PASS** (best-effort with a throwaway Postgres / the shared `screener` DB). Without DSN: SKIP, must import cleanly.

- [ ] **Step 5: Commit**
```bash
git add stock_screener/storage.py tests/test_storage.py
git commit -m "feat: storage (SQLAlchemy Core bars table, upsert, get, last_bar_time)"
```

---

## Task 8: datasource.py (yfinance)

**Files:** Create `stock_screener/datasource.py`; Test `tests/test_datasource.py` (mock yfinance — no network).

- [ ] **Step 1: Write failing test**

```python
# tests/test_datasource.py
import pandas as pd
from datetime import datetime, timezone
from stock_screener import datasource

def test_normalize_drops_unclosed(monkeypatch):
    now = datetime(2026, 6, 16, 12, tzinfo=timezone.utc)
    idx = pd.DatetimeIndex([now.replace(hour=10), now.replace(hour=11, minute=30)], name="ts")
    raw = pd.DataFrame({"Open":[1,2],"High":[1,2],"Low":[1,2],"Close":[1,2],"Volume":[10,20]}, index=idx)
    out = datasource.normalize(raw, bar_seconds=3600, now=now, closed_only=True)
    # 10:00 bar closes 11:00 (<=12:00 keep); 11:30 bar closes 12:30 (>12:00 drop)
    assert len(out) == 1
    assert list(out.columns) == ["open", "high", "low", "close", "volume"]
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/datasource.py`**

```python
from __future__ import annotations
from datetime import datetime, timezone, timedelta
import pandas as pd
import yfinance as yf

def normalize(raw: pd.DataFrame, bar_seconds: int, now: datetime, closed_only: bool) -> pd.DataFrame:
    if raw.empty:
        return pd.DataFrame(columns=["open", "high", "low", "close", "volume"])
    df = raw.rename(columns=str.lower)[["open", "high", "low", "close", "volume"]].copy()
    df.index = pd.to_datetime(df.index, utc=True)
    df.index.name = "ts"
    df = df.dropna(subset=["open", "high", "low", "close"])
    if closed_only and len(df):
        dur = timedelta(seconds=bar_seconds)
        df = df[df.index + dur <= pd.Timestamp(now)]
    return df

def fetch(symbol: str, interval: str, bar_seconds: int, start: datetime | None,
          closed_only: bool) -> pd.DataFrame:
    kwargs = dict(interval=interval, auto_adjust=True)
    if start is not None:
        kwargs["start"] = start
    else:
        kwargs["period"] = "max"
    raw = yf.Ticker(symbol).history(**kwargs)
    return normalize(raw, bar_seconds, datetime.now(timezone.utc), closed_only)
```

- [ ] **Step 4: Run → PASS** (unit test mocks via constructed DataFrame; `fetch` itself is exercised in the smoke test / collector integration).

- [ ] **Step 5: Commit**
```bash
git add stock_screener/datasource.py tests/test_datasource.py
git commit -m "feat: datasource (yfinance fetch + normalize, closed-bars-only)"
```

---

## Task 9: screener.py

**Files:** Create `stock_screener/screener.py`; Test `tests/test_screener.py`.

- [ ] **Step 1: Write failing test**

```python
# tests/test_screener.py
import pandas as pd
from datetime import datetime, timezone, timedelta
from stock_screener.config import Config
from stock_screener.screener import Screener

class FakeStore:
    def __init__(self, df): self._df = df
    def get_bars(self, symbol, timeframe, limit=0): return self._df
    def last_bar_time(self, *a): return None

def _cfg():
    return Config(stocks=["AAA"], timeframes=["1d"])

def _daily(closes):
    t0 = datetime(2026, 1, 1, tzinfo=timezone.utc)
    idx = pd.DatetimeIndex([t0 + timedelta(days=i) for i in range(len(closes))], name="ts")
    return pd.DataFrame({"open": closes, "high": closes, "low": closes,
                         "close": closes, "volume": [100]*len(closes)}, index=idx).astype(float)

def test_screen_returns_rows_and_matches():
    closes = [10,12,14,12,10,12,14,16,14,12,10,8,10,12,14,12,10]
    cfg = _cfg()
    cfg.indicators.distance_from_ma.length = 3
    cfg.indicators.distance_from_ma.ma_type = "SMA"
    cfg.indicators.distance_from_ma.detection.min_prominence = 0.0
    cfg.indicators.distance_from_ma.detection.min_distance = 1
    cfg.indicators.distance_from_ma.detection.smoothing = 1
    s = Screener(FakeStore(_daily(closes)), cfg)
    res = s.screen(symbols=["AAA"], timeframes=["1d"], match="any", indicators=["distance_from_ma"])
    assert isinstance(res.rows, list)
    assert isinstance(res.warnings, list)
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/screener.py`**

```python
from __future__ import annotations
from dataclasses import dataclass, field
from datetime import datetime
import math
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
                if df.empty:
                    res.warnings.append(Warning(symbol, tf_name, "no_data"))
                    continue
                row, warns = self._evaluate(symbol, tf_name, df, match, indicators_)
                res.warnings.extend(warns)
                if row is not None:
                    res.rows.append(row)
        return res

    # accept the alias used by api/cli
    def screen_request(self, *, symbols, timeframes, match, indicators):
        return self.screen(symbols, timeframes, match, indicators)

    def _load(self, symbol, tf):
        need = self._required_bars(tf)
        if tf.native:
            return self.store.get_bars(symbol, tf.name, need)
        parent = self.store.get_bars(symbol, tf.parent, need * tf.group_size)
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

    def _det_cfg(self, ind):
        return getattr(self.cfg.indicators, ind).detection

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
            d = self._det_cfg(ind)
            series = detection.smooth(raw.reset_index(drop=True), d.smoothing)
            valid = series.dropna()
            if valid.empty:
                warns.append(Warning(symbol, tf_name,
                    f"insufficient_data: {ind} needs more bars, have {len(df)}"))
                continue
            idx = int(valid.index[-1])
            peaks, valleys = detection.find_extrema(series, d.min_prominence, d.min_distance)
            peaks = detection.last_n(peaks, self.cfg.screening.peaks_to_show)
            valleys = detection.last_n(valleys, self.cfg.screening.peaks_to_show)
            zone = classify(series.iloc[idx], peaks, valleys)
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
        last_ts = times[-1]
        return (Row(symbol, tf_name, last_ts, float(df["close"].iloc[-1]),
                    triggered, results), warns)

    def _trend(self, series, idx) -> str:
        prev = idx - self.cfg.screening.trend_lookback
        eps = self.cfg.screening.trend_flat_epsilon
        if prev < 0 or pd.isna(series.iloc[prev]):
            return "flat"
        diff = series.iloc[idx] - series.iloc[prev]
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
```

- [ ] **Step 4: Run → PASS.** (The test asserts shape; if `min_prominence`/smoothing fixture needs tweaking to yield a row, adjust the `closes`/detection params in the test only — production stays as written.)

- [ ] **Step 5: Commit**
```bash
git add stock_screener/screener.py tests/test_screener.py
git commit -m "feat: screener orchestration + matches aggregation"
```

---

## Task 10: api.py (FastAPI)

**Files:** Create `stock_screener/api.py`; Test `tests/test_api.py`.

- [ ] **Step 1: Write failing tests**

```python
# tests/test_api.py
from fastapi.testclient import TestClient
from stock_screener.config import Config
from stock_screener.api import create_app
from stock_screener.screener import Result, Row, IndicatorResult, Pivot
from datetime import datetime, timezone

class FakeScreener:
    def __init__(self, res): self._res = res
    def screen(self, symbols, timeframes_, match, indicators_): return self._res

class FakeStore:
    def ping(self): pass

def _cfg():
    return Config(stocks=["AAPL"], timeframes=["1d", "4h"])

def _client(res):
    app = create_app(FakeScreener(res), FakeStore(), _cfg())
    return TestClient(app)

def test_healthz():
    assert _client(Result()).get("/healthz").status_code == 200

def test_screen_shape():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[Row("AAPL","1d",now,200.0,["rsi"],
        {"rsi": IndicatorResult(28.3,"rising","low",True,[Pivot(70.0,now)],[])})])
    body = _client(res).get("/screen").json()
    assert body["results"][0]["symbol"] == "AAPL"
    assert body["results"][0]["indicators"]["rsi"]["zone"] == "low"

def test_matches_shape():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[
        Row("AAPL","1d",now,200.0,["rsi","volume_oscillator"],{}),
        Row("AAPL","4h",now,200.0,["rsi"],{}),
    ])
    body = _client(res).get("/matches").json()
    assert len(body["matches"]) == 1
    m = body["matches"][0]
    assert m["timeframes"] == ["1d","4h"]
    assert m["indicators"] == ["rsi","volume_oscillator"]

def test_validation_400():
    c = _client(Result())
    assert c.get("/matches?timeframes=7m").status_code == 400
    assert c.get("/matches?indicators=bogus").status_code == 400
    assert c.get("/matches?symbols=NOPE").status_code == 400
    assert c.get("/matches?match=min:0").status_code == 400

def test_matches_dedupes_symbols():
    now = datetime(2026, 6, 16, tzinfo=timezone.utc)
    res = Result(rows=[Row("AAPL","1d",now,200.0,["rsi"],{})])
    body = _client(res).get("/matches?symbols=AAPL,AAPL").json()
    assert len(body["matches"]) == 1
    assert body["criteria"]["symbols"] == 1
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/api.py`**

```python
from __future__ import annotations
import logging
from datetime import datetime, timezone
from fastapi import FastAPI, HTTPException, Query
from stock_screener import timeframes
from stock_screener.config import Config, valid_match
from stock_screener.screener import ALL_INDICATORS, aggregate_matches

log = logging.getLogger("stock_screener.api")

def _csv(v: str | None, default: list[str]) -> list[str]:
    if not v or not v.strip():
        return default
    seen, out = set(), []
    for p in (x.strip() for x in v.split(",")):
        if p and p not in seen:
            seen.add(p); out.append(p)
    return out

def create_app(screener, store, cfg: Config) -> FastAPI:
    app = FastAPI(title="stock-screener")

    def parse(symbols, timeframes_, match, indicators_):
        syms = _csv(symbols, cfg.stocks)
        tfs = _csv(timeframes_, cfg.timeframes)
        m = match or cfg.screening.match
        inds = _csv(indicators_, ALL_INDICATORS)
        for tf in tfs:
            if timeframes.get(tf) is None:
                raise HTTPException(400, f"unknown timeframe: {tf}")
        allowed = set(cfg.stocks)
        for s in syms:
            if s not in allowed:
                raise HTTPException(400, f"unknown symbol: {s}")
        if not valid_match(m):
            raise HTTPException(400, f"invalid match mode: {m}")
        seen = set()
        for i in inds:
            if i not in ALL_INDICATORS:
                raise HTTPException(400, f"unknown indicator: {i}")
            if i in seen:
                raise HTTPException(400, f"duplicate indicator: {i}")
            seen.add(i)
        return syms, tfs, m, inds

    def criteria(syms, tfs, m):
        return {"match": m, "symbols": len(syms), "timeframes": tfs}

    @app.get("/healthz")
    def healthz():
        try:
            store.ping()
        except Exception:
            raise HTTPException(503, "db unavailable")
        return "ok"

    @app.get("/screen")
    def screen(symbols: str | None = Query(None), timeframes: str | None = Query(None),
               match: str | None = Query(None), indicators: str | None = Query(None)):
        syms, tfs, m, inds = parse(symbols, timeframes, match, indicators)
        try:
            res = screener.screen(syms, tfs, m, inds)
        except Exception as e:
            log.error("screen failed: %s", e)
            raise HTTPException(500, "internal error")
        return {
            "as_of": datetime.now(timezone.utc),
            "criteria": criteria(syms, tfs, m),
            "results": [
                {"symbol": r.symbol, "timeframe": r.timeframe, "bar_time": r.bar_time,
                 "price": r.price, "triggered": r.triggered,
                 "indicators": {n: {"latest": ir.latest, "trend": ir.trend, "zone": ir.zone,
                                    "triggered": ir.triggered,
                                    "peaks": [{"value": p.value, "time": p.time} for p in ir.peaks],
                                    "valleys": [{"value": p.value, "time": p.time} for p in ir.valleys]}
                                for n, ir in r.indicators.items()}}
                for r in res.rows],
            "warnings": [{"symbol": w.symbol, "timeframe": w.timeframe, "message": w.message}
                         for w in res.warnings],
        }

    @app.get("/matches")
    def matches(symbols: str | None = Query(None), timeframes: str | None = Query(None),
                match: str | None = Query(None), indicators: str | None = Query(None)):
        syms, tfs, m, inds = parse(symbols, timeframes, match, indicators)
        try:
            res = screener.screen(syms, tfs, m, inds)
        except Exception as e:
            log.error("matches failed: %s", e)
            raise HTTPException(500, "internal error")
        return {
            "as_of": datetime.now(timezone.utc),
            "criteria": criteria(syms, tfs, m),
            "matches": aggregate_matches(res, syms, tfs),
            "warnings": [{"symbol": w.symbol, "timeframe": w.timeframe, "message": w.message}
                         for w in res.warnings],
        }

    return app
```

- [ ] **Step 4: Run → PASS.**

- [ ] **Step 5: Commit**
```bash
git add stock_screener/api.py tests/test_api.py
git commit -m "feat: FastAPI app (/screen, /matches, /healthz) with validation"
```

---

## Task 11: collector.py + cli.py

**Files:** Create `stock_screener/collector.py`, `stock_screener/cli.py`; Test `tests/test_collector.py`.

- [ ] **Step 1: Write failing test**

```python
# tests/test_collector.py
import pandas as pd
from datetime import datetime, timezone
from stock_screener.config import Config
from stock_screener.collector import native_timeframes, Collector

def test_native_timeframes():
    assert set(native_timeframes(["15m","4h","1d","3d","1h"])) == {"15m","1d","1h"}

def test_collect_once_upserts():
    cfg = Config(stocks=["AAA"], timeframes=["1d"])
    fetched = {}
    upserts = []
    def fake_fetch(symbol, interval, bar_seconds, start, closed_only):
        idx = pd.DatetimeIndex([datetime(2026,1,1,tzinfo=timezone.utc)], name="ts")
        return pd.DataFrame({"open":[1.0],"high":[2.0],"low":[0.0],"close":[1.5],"volume":[9.0]}, index=idx)
    class FakeStore:
        def last_bar_time(self, *a): return None
        def upsert_bars(self, s, tf, df): upserts.append((s, tf, len(df)))
    errs = Collector(FakeStore(), cfg, fetch=fake_fetch).collect_once()
    assert errs == []
    assert upserts == [("AAA", "1d", 1)]
```

- [ ] **Step 2: Run → FAIL.**

- [ ] **Step 3: Implement `stock_screener/collector.py`**

```python
from __future__ import annotations
import logging, time
from stock_screener import timeframes, datasource
from stock_screener.config import Config

log = logging.getLogger("stock_screener.collector")

def native_timeframes(tfs: list[str]) -> list[str]:
    seen, out = set(), []
    for name in tfs:
        tf = timeframes.get(name)
        if tf is None:
            continue
        n = tf.name if tf.native else tf.parent
        if n not in seen:
            seen.add(n); out.append(n)
    return out

class Collector:
    def __init__(self, store, cfg: Config, fetch=datasource.fetch):
        self.store = store
        self.cfg = cfg
        self._fetch = fetch

    def collect_timeframes(self, tf_names: list[str]) -> list[Exception]:
        errs: list[Exception] = []
        for symbol in self.cfg.stocks:
            for tf_name in tf_names:
                tf = timeframes.get(tf_name)
                try:
                    start = self.store.last_bar_time(symbol, tf_name)
                    df = self._fetch(symbol, tf.yahoo_interval, tf.bar_seconds, start,
                                     self.cfg.collector.use_closed_bars_only)
                    self.store.upsert_bars(symbol, tf_name, df)
                    log.info("collected %s %s: %d bars", symbol, tf_name, len(df))
                    time.sleep(0.2)
                except Exception as e:  # noqa: BLE001 — per-item isolation
                    log.error("collect %s %s: %s", symbol, tf_name, e)
                    errs.append(e)
        return errs

    def collect_once(self) -> list[Exception]:
        return self.collect_timeframes(native_timeframes(self.cfg.timeframes))
```

- [ ] **Step 4: Implement `stock_screener/cli.py`**

```python
from __future__ import annotations
import logging, sys
import typer
import uvicorn
from stock_screener.config import load_config
from stock_screener.storage import Store, dsn_from_env
from stock_screener.collector import Collector
from stock_screener.screener import Screener
from stock_screener.api import create_app

logging.basicConfig(level=logging.INFO)
app = typer.Typer(add_completion=False)

@app.command()
def collect(config: str = "config.yaml"):
    cfg = load_config(config)
    store = Store(dsn_from_env()); store.migrate()
    errs = Collector(store, cfg).collect_once()
    for e in errs:
        logging.error("collect error: %s", e)
    if errs:
        logging.error("collect finished with %d error(s)", len(errs))
        raise typer.Exit(code=1)
    logging.info("collect finished: ok")

@app.command()
def serve(config: str = "config.yaml"):
    cfg = load_config(config)
    store = Store(dsn_from_env()); store.migrate()
    application = create_app(Screener(store, cfg), store, cfg)
    uvicorn.run(application, host="0.0.0.0", port=cfg.server.port)

if __name__ == "__main__":
    app()
```

- [ ] **Step 5: Run → PASS** (`pytest tests/test_collector.py -q`).

- [ ] **Step 6: Commit**
```bash
git add stock_screener/collector.py stock_screener/cli.py tests/test_collector.py
git commit -m "feat: collector + Typer CLI (collect/serve)"
```

---

## Task 12: config.yaml update + full test run

**Files:** Modify `config.yaml`.

- [ ] **Step 1: Rewrite `config.yaml`** to the Python schema (per-indicator `detection`, no `pivot_window`):

```yaml
server:
  port: 8090

collector:
  enabled: true
  use_closed_bars_only: true
  refresh: { intraday: 15m, daily: 6h }

stocks: [AAPL, GOOGL, MSFT, TSLA, AMZN]
timeframes: [15m, 30m, 1h, 4h, 1d, 3d, 1wk, 1mo]

screening:
  match: any
  trend_lookback: 3
  peaks_to_show: 3
  peak_lookback: 3mo
  trend_flat_epsilon: 0

indicators:
  rsi:
    length: 14
    source: close
    smoothing: { type: SMA, length: 14, bb_stddev: 2 }
    detection: { smoothing: 3, min_prominence: 8, min_distance: 5 }
  volume_oscillator:
    short_length: 5
    long_length: 10
    detection: { smoothing: 3, min_prominence: 5, min_distance: 5 }
  distance_from_ma:
    source: close
    ma_type: EMA
    length: 200
    calculation: percent
    detection: { smoothing: 3, min_prominence: 3, min_distance: 5 }
```

- [ ] **Step 2: Run the full suite + lint**

Run: `ruff check . && pytest -q`
Expected: ruff clean; all tests pass (storage skips without DSN).

- [ ] **Step 3: Best-effort live smoke** (shared DB has data from the Go collector):
```bash
set -a && . ./.env && set +a
. .venv/bin/activate
stock-screener serve --config config.yaml &   # uvicorn on :8090
sleep 3
curl -s "http://localhost:8090/healthz"; echo
curl -s "http://localhost:8090/matches?symbols=AAPL&timeframes=1d,4h" | python3 -m json.tool | head -30
kill %1
```
Expected: `ok`; `/matches` returns valid JSON. (Indicators recompute from the existing `bars` rows — no re-collect needed.)

- [ ] **Step 4: Commit**
```bash
git add config.yaml
git commit -m "feat: config.yaml for python screener (per-indicator detection)"
```

---

## Task 13: Remove Go implementation

**Files:** Delete Go sources.

- [ ] **Step 1: Remove Go files**
```bash
git rm -r go.mod go.sum main.go main_test.go internal/
```
(Keep `docs/`, `config.yaml`, `.env.example`, `docker-compose.yaml`, `README.md`.)

- [ ] **Step 2: Verify Python still builds/tests**

Run: `pytest -q && ruff check .`
Expected: green.

- [ ] **Step 3: Update README.md** — replace Go run instructions with Python:

```markdown
## Run
    python3.12 -m venv .venv && . .venv/bin/activate && pip install -e '.[dev]'
    set -a && . ./.env && set +a
    stock-screener collect --config config.yaml   # fetch + store bars
    stock-screener serve   --config config.yaml   # FastAPI on :server.port (OpenAPI at /docs)
```
(Keep the API section: `/screen`, `/matches`, `/healthz` — behavior unchanged.)

- [ ] **Step 4: Commit**
```bash
git add -A
git commit -m "chore: remove Go implementation (superseded by Python rewrite)"
```

---

## Self-Review Notes (completed during planning)

- **Spec coverage:** stack (Task 0); config+durations+validation (Task 1); timeframes/bucketing (Task 2); indicators (Task 3); detection prominence+distance+smoothing (Task 4); resample 4h/3d (Task 5); rule (Task 6); storage SQLAlchemy upsert/get/last (Task 7); datasource yfinance + closed-bars (Task 8); screener orchestration + matches aggregation + required-bars + trend (Task 9); FastAPI /screen,/matches,/healthz + validation + dedup (Task 10); collector + CLI collect/serve (Task 11); config.yaml + smoke (Task 12); Go removal + README (Task 13). All spec sections map to a task.
- **Type consistency:** `Store` (get_bars/last_bar_time/upsert_bars/ping/migrate) used by screener (Task 9), api healthz (Task 10), collector (Task 11), cli (Task 11). `Screener.screen(symbols, timeframes_, match, indicators_)` signature consistent between screener (9) and api/cli fakes/calls (10/11). `aggregate_matches(res, symbols, timeframes_)` consistent. `detection.find_extrema/smooth/last_n` consistent across detection (4) and screener (9). `timeframes.get`/`TF` consistent (2) across config (1), resample (5), screener (9), collector (11).
- **Known risks flagged:** pandas-ta/numpy2 (Task 0/4 note, with pure-pandas RSI fallback already used in Task 3); yfinance avoids the Go `range=max` quarterly bug by using period/start internally. Storage + smoke are best-effort against the shared Postgres.
