from sqlalchemy import make_url
from stock_screener.storage import dsn_from_env


def test_dsn_escapes_special_chars(monkeypatch):
    monkeypatch.setenv("DB_USER", "user")
    monkeypatch.setenv("DB_PASSWORD", "p/w@rd:x?y#z")   # all DSN-significant chars
    monkeypatch.setenv("DB_HOST", "db.example.com")
    monkeypatch.setenv("DB_NAME", "screener")
    monkeypatch.setenv("DB_PORT", "5544")
    monkeypatch.setenv("DB_SSLMODE", "require")
    dsn = dsn_from_env()
    url = make_url(dsn)              # round-trips without corruption
    assert url.drivername == "postgresql+psycopg"
    assert url.username == "user"
    assert url.password == "p/w@rd:x?y#z"     # password intact, not truncated by '/'
    assert url.host == "db.example.com"
    assert url.port == 5544
    assert url.database == "screener"
    assert url.query.get("sslmode") == "require"


def test_dsn_default_sslmode(monkeypatch):
    monkeypatch.setenv("DB_USER", "u")
    monkeypatch.setenv("DB_PASSWORD", "p")
    monkeypatch.setenv("DB_HOST", "h")
    monkeypatch.setenv("DB_NAME", "n")
    monkeypatch.delenv("DB_PORT", raising=False)
    monkeypatch.delenv("DB_SSLMODE", raising=False)
    url = make_url(dsn_from_env())
    assert url.port == 5432 and url.query.get("sslmode") == "require"
