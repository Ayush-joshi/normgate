#!/usr/bin/env python3
"""Reject unresolved or changed third-party licenses until explicitly reviewed."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
lock = json.loads((root / "scripts/licenses.lock.json").read_text())
raw = subprocess.check_output([os.environ.get("GO", "go"), "list", "-m", "-json", "all"], cwd=root, text=True)
decoder = json.JSONDecoder()
checked = set()
while raw.strip():
    module, end = decoder.raw_decode(raw.lstrip())
    raw = raw.lstrip()[end:]
    if module.get("Main"):
        continue
    assert not module.get("Replace"), "Review replacement module licenses"
    key = module["Path"] + "@" + module["Version"]
    assert key in lock, f"License review required: {key}"
    assert module.get("Dir"), f"Run make setup: {key} has not been downloaded"
    spec = lock[key]
    data = (Path(module["Dir"]) / spec["file"]).read_bytes()
    assert hashlib.sha256(data).hexdigest() == spec["sha256"], f"Changed license: {key}"
    checked.add(key)
for path, package in json.loads((root / "package-lock.json").read_text())["packages"].items():
    if not path:
        continue
    key = path.removeprefix("node_modules/") + "@" + package["version"]
    assert key in lock, f"License review required: {key}"
    spec = lock[key]
    assert hashlib.sha256((root / path / spec["file"]).read_bytes()).hexdigest() == spec["sha256"], f"Changed license: {key}"
    checked.add(key)
assert checked == set(lock), "License lock has stale entries"
print(f"Verified {len(checked)} reviewed dependency licenses")
