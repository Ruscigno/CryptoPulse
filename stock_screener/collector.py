from __future__ import annotations
import logging
import time
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
            seen.add(n)
            out.append(n)
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
                except Exception as e:  # noqa: BLE001 - per-item isolation
                    log.error("collect %s %s: %s", symbol, tf_name, e)
                    errs.append(e)
        return errs

    def collect_once(self) -> list[Exception]:
        return self.collect_timeframes(native_timeframes(self.cfg.timeframes))
