#!/usr/bin/env python3
"""Run shared NG-F004 golden vectors against the Python SDK."""
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "sdk/python/src"))
from normgate.normalization import CanonicalizationError, canonicalize, digest

vectors = json.loads((ROOT / "testkit/fixtures/normalization/vectors.json").read_text())
for vector in vectors:
    try:
        value = canonicalize(vector["input"], vector["profile"])
    except CanonicalizationError:
        assert vector.get("invalid"), vector["name"]
    else:
        assert not vector.get("invalid"), vector["name"]
        assert value == vector["canonical"], (vector["name"], value)
        assert digest(value) == vector["digest"], vector["name"]
print(f"Python: {len(vectors)} canonical vectors passed")
