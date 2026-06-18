from __future__ import annotations


def classify(current: float, peaks, valleys) -> str:
    if peaks and current >= min(v for _, v in peaks):
        return "high"
    if valleys and current <= max(v for _, v in valleys):
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
