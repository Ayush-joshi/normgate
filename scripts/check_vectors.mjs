import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import { canonicalize, digest, CanonicalizationError } from "../sdk/typescript/src/normalization.ts";

const vectors = JSON.parse(readFileSync(new URL("../testkit/fixtures/normalization/vectors.json", import.meta.url)));
for (const v of vectors) {
  if (v.invalid) assert.throws(() => canonicalize(v.input, v.profile), CanonicalizationError, v.name);
  else {
    const canonical = canonicalize(v.input, v.profile);
    assert.equal(canonical, v.canonical, v.name);
    assert.equal(await digest(canonical), v.digest, v.name);
  }
}
console.log(`TypeScript: ${vectors.length} canonical vectors passed`);
