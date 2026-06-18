from __future__ import annotations
import logging
import typer
import uvicorn
from stock_screener.api import create_app
from stock_screener.collector import Collector
from stock_screener.config import load_config
from stock_screener.screener import Screener
from stock_screener.storage import Store, dsn_from_env

logging.basicConfig(level=logging.INFO)
app = typer.Typer(add_completion=False)

@app.command()
def collect(config: str = "config.yaml"):
    cfg = load_config(config)
    store = Store(dsn_from_env())
    store.migrate()
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
    store = Store(dsn_from_env())
    store.migrate()
    collector = Collector(store, cfg)
    application = create_app(Screener(store, cfg), store, cfg, collector=collector)
    uvicorn.run(
        application, host="0.0.0.0", port=cfg.server.port,
        timeout_keep_alive=5,
        timeout_graceful_shutdown=10,
        limit_concurrency=128,
        limit_max_requests=10000,
    )

if __name__ == "__main__":
    app()
