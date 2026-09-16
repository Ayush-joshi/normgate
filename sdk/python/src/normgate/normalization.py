"""NormGate canonical JSON v1 (NG-F004); see shared golden vectors."""
from datetime import datetime, timedelta, timezone
import hashlib
import json
import re
import unicodedata


class CanonicalizationError(ValueError):
    """Invalid canonical input; messages never include payload contents."""


def _error(*_args):
    raise CanonicalizationError("invalid canonical input")


def _integer(value):
    n = int(value)
    if abs(n) > 9007199254740991:
        _error()
    return n


def _pairs(pairs):
    result = {}
    seen = set()
    for key, value in pairs:
        normalized = unicodedata.normalize("NFC", key)
        if normalized in seen:
            _error()
        seen.add(normalized)
        result[key] = value
    return result


def _check_string(value):
    value.encode("utf-8")
    run = 0
    for char in unicodedata.normalize("NFD", value):
        run = run + 1 if unicodedata.category(char).startswith("M") else 0
        if run > 30:
            _error()


def _timestamp(value):
    if not isinstance(value, str):
        _error()
    match = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})", value)
    if not match:
        _error()
    stamp, fraction, offset = match.groups()
    tz = timezone.utc
    if offset != "Z":
        hours, minutes = int(offset[1:3]), int(offset[4:])
        if hours > 23 or minutes > 59:
            _error()
        tz = timezone((1 if offset[0] == "+" else -1) * timedelta(hours=hours, minutes=minutes))
    dt = datetime.strptime(stamp, "%Y-%m-%dT%H:%M:%S").replace(tzinfo=tz).astimezone(timezone.utc)
    fraction = (fraction or "").rstrip("0")
    return f"{dt.year:04d}-{dt.month:02d}-{dt.day:02d}T{dt.hour:02d}:{dt.minute:02d}:{dt.second:02d}" + ("." + fraction if fraction else "") + "Z"


def canonicalize(data: str, profile: dict | None = None) -> str:
    profile = profile or {}

    def normalize(value, path="", depth=0):
        if depth > 64:
            _error()
        if isinstance(value, str) and any(0xD800 <= ord(c) <= 0xDFFF for c in value):
            _error()
        if path in profile.get("timestamps", []):
            return _timestamp(value)
        if path in profile.get("sets", []):
            if not isinstance(value, list) or any(not isinstance(x, str) for x in value):
                _error()
            result = sorted(normalize(x, path + "/" + str(i), depth + 1) for i, x in enumerate(value))
            if len(set(result)) != len(result):
                _error()
            return result
        if isinstance(value, str):
            return unicodedata.normalize("NFC", value)
        if isinstance(value, dict):
            result = {}
            for key, item in value.items():
                key = normalize(key, path + "/~key", depth + 1)
                pointer = path + "/" + key.replace("~", "~0").replace("/", "~1")
                if pointer not in profile.get("omit", []):
                    result[key] = normalize(item, pointer, depth + 1)
            return result
        if isinstance(value, list):
            return [normalize(item, path + "/" + str(i), depth + 1) for i, item in enumerate(value)]
        return value

    try:
        if len(data.encode("utf-8")) > 1048576:
            _error()
        value = json.loads(data, object_pairs_hook=_pairs, parse_int=_integer, parse_float=_error, parse_constant=_error)
        # Validate the entire tree before omissions, matching Go and TypeScript.
        def inspect(v, depth=0):
            if depth > 64:
                _error()
            if isinstance(v, str):
                _check_string(v)
            elif isinstance(v, dict):
                for k, x in v.items():
                    _check_string(k)
                    inspect(x, depth+1)
            elif isinstance(v, list):
                for x in v:
                    inspect(x, depth+1)
        inspect(value)
        return json.dumps(normalize(value), ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    except (ValueError, OverflowError, RecursionError, UnicodeError):
        raise CanonicalizationError("invalid canonical input") from None


def digest(canonical: str) -> str:
    return "sha256:" + hashlib.sha256(canonical.encode("utf-8")).hexdigest()
