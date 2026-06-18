from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone


@dataclass(frozen=True)
class TF:
    name: str
    native: bool
    yahoo_interval: str  # native only
    parent: str  # derived only
    group_size: int  # derived only
    bar_seconds: int  # approximate bar length

    def bucket_start(self, t: datetime) -> datetime:
        t = t.astimezone(timezone.utc)
        if self.name == "3d":
            day = int(t.timestamp()) // 86400
            start = day - day % 3
            return datetime.fromtimestamp(start * 86400, tz=timezone.utc)
        secs = int(t.timestamp())
        return datetime.fromtimestamp(secs - secs % self.bar_seconds, tz=timezone.utc)


_REG = {
    "15m": TF("15m", True, "15m", "", 0, 15 * 60),
    "30m": TF("30m", True, "30m", "", 0, 30 * 60),
    "1h": TF("1h", True, "1h", "", 0, 3600),
    "4h": TF("4h", False, "", "1h", 4, 4 * 3600),
    "1d": TF("1d", True, "1d", "", 0, 86400),
    "3d": TF("3d", False, "", "1d", 3, 3 * 86400),
    "1wk": TF("1wk", True, "1wk", "", 0, 7 * 86400),
    "1mo": TF("1mo", True, "1mo", "", 0, 30 * 86400),
}


def get(name: str) -> TF | None:
    return _REG.get(name)
