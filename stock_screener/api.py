from __future__ import annotations
import asyncio
import logging
from contextlib import asynccontextmanager
from datetime import datetime, timezone
from fastapi import FastAPI, HTTPException, Query
from fastapi.encoders import jsonable_encoder
from fastapi.responses import JSONResponse
from stock_screener import timeframes as _tf
from stock_screener import timeframes
from stock_screener.config import Config, valid_match, parse_duration
from stock_screener.collector import native_timeframes
from stock_screener.screener import ALL_INDICATORS, aggregate_matches

log = logging.getLogger("stock_screener.api")


def _csv(v, default):
    if not v or not v.strip():
        return list(default)
    seen, out = set(), []
    for p in (x.strip() for x in v.split(",")):
        if p and p not in seen:
            seen.add(p)
            out.append(p)
    return out


async def _scheduler(collector, cfg):
    natives = native_timeframes(cfg.timeframes)
    intraday = [t for t in natives if _tf.get(t).bar_seconds < 86400]
    daily = [t for t in natives if _tf.get(t).bar_seconds >= 86400]
    intraday_s = parse_duration(cfg.collector.refresh.intraday)
    daily_s = parse_duration(cfg.collector.refresh.daily)

    async def loop(subset, period):
        while True:
            await asyncio.sleep(period)
            if subset:
                await asyncio.to_thread(collector.collect_timeframes, subset)

    await asyncio.to_thread(collector.collect_timeframes, natives)  # initial full pass
    await asyncio.gather(loop(intraday, intraday_s), loop(daily, daily_s))


def create_app(screener, store, cfg: Config, collector=None) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app):
        task = None
        if collector is not None and cfg.collector.enabled:
            task = asyncio.create_task(_scheduler(collector, cfg))
        try:
            yield
        finally:
            if task is not None:
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass

    app = FastAPI(title="stock-screener", lifespan=lifespan)

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
        for i in inds:
            if i not in ALL_INDICATORS:
                raise HTTPException(400, f"unknown indicator: {i}")
        return syms, tfs, m, inds

    def _criteria(syms, tfs, m):
        return {"match": m, "symbols": len(syms), "timeframes": tfs}

    def _warns(res):
        return [
            {"symbol": w.symbol, "timeframe": w.timeframe, "message": w.message}
            for w in res.warnings
        ]

    @app.get("/healthz")
    def healthz():
        try:
            store.ping()
        except Exception:
            raise HTTPException(503, "db unavailable")
        return "ok"

    @app.get("/screen")
    def screen(
        symbols: str | None = Query(None),
        timeframes: str | None = Query(None),
        match: str | None = Query(None),
        indicators: str | None = Query(None),
    ):
        _reject_dup_indicators(indicators)
        syms, tfs, m, inds = parse(symbols, timeframes, match, indicators)
        try:
            res = screener.screen(syms, tfs, m, inds)
        except Exception as e:
            log.error("screen failed: %s", e)
            raise HTTPException(500, "internal error")
        body = {
            "as_of": datetime.now(timezone.utc),
            "criteria": _criteria(syms, tfs, m),
            "results": [
                {
                    "symbol": r.symbol,
                    "timeframe": r.timeframe,
                    "bar_time": r.bar_time,
                    "price": r.price,
                    "triggered": r.triggered,
                    "indicators": {
                        n: {
                            "latest": ir.latest,
                            "trend": ir.trend,
                            "zone": ir.zone,
                            "triggered": ir.triggered,
                            "peaks": [
                                {"value": p.value, "time": p.time} for p in ir.peaks
                            ],
                            "valleys": [
                                {"value": p.value, "time": p.time}
                                for p in ir.valleys
                            ],
                        }
                        for n, ir in r.indicators.items()
                    },
                }
                for r in res.rows
            ],
            "warnings": _warns(res),
        }
        return JSONResponse(jsonable_encoder(body))

    @app.get("/matches")
    def matches(
        symbols: str | None = Query(None),
        timeframes: str | None = Query(None),
        match: str | None = Query(None),
        indicators: str | None = Query(None),
    ):
        _reject_dup_indicators(indicators)
        syms, tfs, m, inds = parse(symbols, timeframes, match, indicators)
        try:
            res = screener.screen(syms, tfs, m, inds)
        except Exception as e:
            log.error("matches failed: %s", e)
            raise HTTPException(500, "internal error")
        body = {
            "as_of": datetime.now(timezone.utc),
            "criteria": _criteria(syms, tfs, m),
            "matches": aggregate_matches(res, syms, tfs),
            "warnings": _warns(res),
        }
        return JSONResponse(jsonable_encoder(body))

    return app


def _reject_dup_indicators(indicators: str | None):
    if not indicators:
        return
    raw = [x.strip() for x in indicators.split(",") if x.strip()]
    if len(raw) != len(set(raw)):
        raise HTTPException(400, "duplicate indicator")
