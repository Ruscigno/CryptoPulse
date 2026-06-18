from __future__ import annotations
import os
import pandas as pd
from sqlalchemy import (URL, Column, DateTime, Float, MetaData, String, Table,
                        create_engine, func, select, text)
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
    return URL.create(
        "postgresql+psycopg",
        username=u, password=p, host=h, port=int(port),
        database=name, query={"sslmode": sslmode},
    ).render_as_string(hide_password=False)


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
             "open": float(r.open), "high": float(r.high), "low": float(r.low),
             "close": float(r.close), "volume": float(r.volume)}
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
        return df.iloc[::-1]

    def last_bar_time(self, symbol: str, timeframe: str):
        q = select(func.max(bars.c.ts)).where(
            bars.c.symbol == symbol, bars.c.timeframe == timeframe)
        with self.engine.connect() as c:
            return c.execute(q).scalar()
