from typer.testing import CliRunner
from stock_screener import cli

runner = CliRunner()

class _Cfg:  # minimal stand-in; fakes ignore it
    pass

def _patch_common(monkeypatch):
    monkeypatch.setattr(cli, "load_config", lambda p: _Cfg())
    monkeypatch.setattr(cli, "dsn_from_env", lambda: "x")
    class FakeStore:
        def __init__(self, dsn): pass
        def migrate(self): pass
    monkeypatch.setattr(cli, "Store", FakeStore)

def test_collect_exit_zero(monkeypatch):
    _patch_common(monkeypatch)
    class FakeCollector:
        def __init__(self, store, cfg): pass
        def collect_once(self): return []
    monkeypatch.setattr(cli, "Collector", FakeCollector)
    r = runner.invoke(cli.app, ["collect"])
    assert r.exit_code == 0

def test_collect_exit_nonzero_on_errors(monkeypatch):
    _patch_common(monkeypatch)
    class FakeCollector:
        def __init__(self, store, cfg): pass
        def collect_once(self): return [RuntimeError("boom")]
    monkeypatch.setattr(cli, "Collector", FakeCollector)
    r = runner.invoke(cli.app, ["collect"])
    assert r.exit_code == 1

def test_serve_wires_and_runs(monkeypatch):
    _patch_common(monkeypatch)
    monkeypatch.setattr(cli, "Collector", lambda store, cfg: object())
    monkeypatch.setattr(cli, "Screener", lambda store, cfg: object())
    monkeypatch.setattr(cli, "create_app", lambda *a, **k: "APP")
    ran = {}
    def fake_run(app, **kw):
        ran["app"] = app
        ran["kw"] = kw
    monkeypatch.setattr(cli.uvicorn, "run", fake_run)
    # server.port is read from cfg; give _Cfg a server.port
    class Srv:  # noqa
        port = 8099
    cfg = _Cfg()
    cfg.server = Srv()
    monkeypatch.setattr(cli, "load_config", lambda p: cfg)
    r = runner.invoke(cli.app, ["serve"])
    assert r.exit_code == 0
    assert ran["app"] == "APP"
    assert ran["kw"]["port"] == 8099
    assert ran["kw"]["timeout_keep_alive"] == 5
    assert ran["kw"]["limit_concurrency"] == 128
