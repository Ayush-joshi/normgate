#!/usr/bin/env python3
"""Enforce NG-F008 statement coverage without removing production packages."""
from pathlib import Path
import sys

totals = {}
blocks = {}
for line in Path(sys.argv[1]).read_text().splitlines()[1:]:
    location, count, hits = line.split()
    statements = int(count)
    # -coverpkg includes the same blocks across test binaries; merge their hits.
    previous = blocks.get(location, (statements, False))
    blocks[location] = (statements, previous[1] or int(hits) > 0)
for location, (count, covered) in blocks.items():
    package = location.rsplit("/", 1)[0]
    total, hit = totals.get(package, (0, 0))
    totals[package] = (total + count, hit + (count if covered else 0))
failed = False
for suffix in ("internal/contracts", "internal/normalization", "internal/config", "internal/policy", "internal/policy/opa"):
    matches = [(total, hit) for package, (total, hit) in totals.items() if package.endswith("/" + suffix)]
    total = sum(x[0] for x in matches)
    hit = sum(x[1] for x in matches)
    value = 100 * hit / total if total else 0
    print(f"{suffix}: {value:.2f}% (minimum 90%)")
    failed |= value < 90
total = sum(x[0] for x in totals.values())
hit = sum(x[1] for x in totals.values())
value = 100 * hit / total if total else 0
print(f"Repository: {value:.2f}% (minimum 85%)")
failed |= value < 85
sys.exit(1 if failed else 0)
