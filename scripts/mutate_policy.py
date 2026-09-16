#!/usr/bin/env python3
"""Run targeted composition mutants via Go overlays without editing the checkout."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

root = Path(__file__).resolve().parents[1]
source = root / "internal/policy/composition.go"
original = source.read_text()
mutants = {
    "deny_precedence": ("case deny:", "case false:"),
    "approval_precedence": ("case approval:", "case false:"),
    "transformation_merge": ("case transform:", "case false:"),
    "conflict_rejection": ("if conflict(a, b) {", "if false {"),
    "default_denial": ('return Deny("no_decision")', 'return Result{Outcome: "allow", ReasonCodes: []string{"ng.policy.test"}, Obligations: []v1.Obligation{}}'),
    "obligation_identity": ("if !same(previous, o) {", "if false {"),
}
for name, (before, after) in mutants.items():
    assert before in original, f"Refresh stale mutant: {name}"
    mutated = original.replace(before, after, 1)
    # Keep variables used so a killed mutant must be a behavioral failure.
    if name == "conflict_rejection":
        mutated = mutated.replace("if false {\n\t\t\t\treturn Deny", "if conflict(a, b) && false {\n\t\t\t\treturn Deny")
    if name == "obligation_identity":
        mutated = mutated.replace("if false {\n\t\t\t\t\treturn Deny", "if !same(previous, o) && false {\n\t\t\t\t\treturn Deny")
    for flag in ("deny", "approval", "transform"):
        if name == flag + "_precedence" or (flag == "transform" and name == "transformation_merge"):
            mutated = mutated.replace("case false:", f"case {flag} && false:", 1)
    with tempfile.TemporaryDirectory(prefix="normgate-mutation-") as directory:
        target = Path(directory) / "composition.go"
        target.write_text(mutated)
        overlay = Path(directory) / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {str(source): str(target)}}))
        result = subprocess.run(
            [os.environ.get("GO", "go"), "test", "-count=1", f"-overlay={overlay}",
             "./internal/policy", "-run", "TestComposition|TestCanonicalAndCompositionBoundaries"],
            cwd=root, text=True, capture_output=True, timeout=120,
        )
        output = result.stdout + result.stderr
        if result.returncode == 0 or "--- FAIL:" not in output:
            raise SystemExit(f"Mutant survived or did not compile: {name}\n{output}")
        print(f"Killed {name}")
print(f"Killed {len(mutants)}/{len(mutants)} behavioral composition mutants")
