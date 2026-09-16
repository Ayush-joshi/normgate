// NormGate canonical JSON profile v1. Transport types alone do not validate input.
export type Profile = { sets?: string[]; timestamps?: string[]; omit?: string[] };
type Value = null | boolean | number | string | Value[] | { [key: string]: Value };

export class CanonicalizationError extends Error {
  constructor() { super("invalid canonical input"); this.name = "CanonicalizationError"; }
}
function invalid(): never { throw new CanonicalizationError(); }
function validString(value: string): string {
  for (const char of value) {
    const point = char.codePointAt(0)!;
    if (point >= 0xd800 && point <= 0xdfff) invalid();
  }
  let run = 0;
  for (const char of value.normalize("NFD")) {
    run = /\p{M}/u.test(char) ? run + 1 : 0;
    if (run > 30) invalid();
  }
  return value;
}
// JSON.parse alone loses duplicate keys and number spellings. Keep both until checked.
function decode(input: string): Value {
  if (new TextEncoder().encode(input).length > 1048576) invalid();
  let position = 0;
  const space = () => { while (/[ \t\r\n]/.test(input[position] ?? "x")) position++; };
  function quoted(): string {
    const start = position++;
    while (position < input.length) {
      const c = input[position++];
      if (c === "\\") { position++; continue; }
      if (c === '"') return validString(JSON.parse(input.slice(start, position)) as string);
    }
    return invalid();
  }
  function read(depth: number): Value {
    if (depth > 64) invalid();
    space(); const c = input[position];
    if (c === '"') return quoted();
    if (c === "{") {
      position++; space();
      const result: { [key: string]: Value } = Object.create(null);
      const seen = new Set<string>();
      if (input[position] === "}") { position++; return result; }
      while (true) {
        space(); if (input[position] !== '"') invalid();
        const key = quoted(); const normalized = key.normalize("NFC");
        if (seen.has(normalized)) invalid(); seen.add(normalized);
        space(); if (input[position++] !== ":") invalid();
        result[key] = read(depth + 1); space();
        const end = input[position++];
        if (end === "}") return result;
        if (end !== ",") invalid();
      }
    }
    if (c === "[") {
      position++; space(); const result: Value[] = [];
      if (input[position] === "]") { position++; return result; }
      while (true) {
        result.push(read(depth + 1)); space(); const end = input[position++];
        if (end === "]") return result;
        if (end !== ",") invalid();
      }
    }
    for (const [token, value] of [["null", null], ["true", true], ["false", false]] as const) {
      if (input.startsWith(token, position)) { position += token.length; return value; }
    }
    const match = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/.exec(input.slice(position));
    if (!match || /[.eE]/.test(match[0])) invalid();
    const number = Number(match[0]); if (!Number.isSafeInteger(number)) invalid();
    position += match[0].length; return number;
  }
  const value = read(0); space(); if (position !== input.length) invalid(); return value;
}

function compare(a: string, b: string): number {
  // Unicode scalar ordering, not JavaScript's UTF-16 code-unit ordering.
  const x = Array.from(a, c => c.codePointAt(0)!); const y = Array.from(b, c => c.codePointAt(0)!);
  for (let i = 0; i < Math.min(x.length, y.length); i++) { if (x[i] !== y[i]) return x[i] - y[i]; }
  return x.length - y.length;
}
function timestamp(value: Value): string {
  if (typeof value !== "string") invalid();
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/.exec(value);
  if (!m) invalid();
  const [year, month, day, hour, minute, second] = m.slice(1, 7).map(Number);
  if (year < 1 || month < 1 || month > 12 || day < 1 || hour > 23 || minute > 59 || second > 59) invalid();
  const date = new Date(0); date.setUTCFullYear(year, month - 1, day); date.setUTCHours(hour, minute, second, 0);
  if (date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) invalid();
  if (m[8] !== "Z") {
    const hours = Number(m[8].slice(1, 3)); const minutes = Number(m[8].slice(4));
    if (hours > 23 || minutes > 59) invalid();
    date.setTime(date.getTime() - (m[8][0] === "+" ? 1 : -1) * (hours * 60 + minutes) * 60000);
  }
  if (date.getUTCFullYear() < 1 || date.getUTCFullYear() > 9999) invalid();
  const fraction = (m[7] ?? "").replace(/0+$/, "");
  return date.toISOString().slice(0, 19) + (fraction ? "." + fraction : "") + "Z";
}

export function canonicalize(input: string, profile: Profile = {}): string {
  function normalize(value: Value, path: string): Value {
    if (profile.timestamps?.includes(path)) return timestamp(value);
    if (profile.sets?.includes(path)) {
      if (!Array.isArray(value) || value.some(x => typeof x !== "string")) invalid();
      const result = (value as string[]).map(x => x.normalize("NFC")).sort(compare);
      if (new Set(result).size !== result.length) invalid(); return result;
    }
    if (typeof value === "string") return value.normalize("NFC");
    if (Array.isArray(value)) return value.map((x, i) => normalize(x, path + "/" + i));
    if (value && typeof value === "object") {
      const result: { [key: string]: Value } = Object.create(null);
      for (const [raw, item] of Object.entries(value)) {
        const key = raw.normalize("NFC"); const next = path + "/" + key.replace(/~/g, "~0").replace(/\//g, "~1");
        if (!profile.omit?.includes(next)) result[key] = normalize(item, next);
      }
      return result;
    }
    return value;
  }
  function encode(value: Value): string {
    if (Array.isArray(value)) return "[" + value.map(encode).join(",") + "]";
    if (value && typeof value === "object") {
      return "{" + Object.keys(value).sort(compare).map(k => JSON.stringify(k) + ":" + encode(value[k])).join(",") + "}";
    }
    return JSON.stringify(value);
  }
  try { return encode(normalize(decode(input), "")); } catch { return invalid(); }
}

export async function digest(canonical: string): Promise<string> {
  const hash = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(canonical));
  return "sha256:" + Array.from(new Uint8Array(hash), b => b.toString(16).padStart(2, "0")).join("");
}
