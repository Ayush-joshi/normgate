# Canonical JSON profile v1

Requirement: NG-F004. The shared vectors in
`testkit/fixtures/normalization/vectors.json` are the compatibility authority.
Go, Python and TypeScript must produce byte-identical UTF-8 and SHA-256 digests.
This is a NormGate profile, not a claim of RFC 8785 conformance.

## Parsing

- Limit input to 1 MiB and nesting to 64 value levels (root depth zero).
- Accept exactly one JSON value and JSON whitespace only.
- Reject invalid UTF-8, unpaired UTF-16 surrogate escapes, duplicate object keys,
  and keys that collide after NFC normalization. Never repair invalid Unicode.
- Reject more than 30 consecutive Unicode Mark characters after canonical
  decomposition (NFD), including in keys and omitted fields. This shared bound
  prevents Go's stream-safe normalization from inserting a grapheme joiner that
  Python and JavaScript would not insert. Explicit joiners count as marks too.
- Permit only integer JSON number tokens between -9007199254740991 and
  9007199254740991 inclusive. Reject decimal and exponent spellings, including
  `1.0` and `1e0`. Normalize `-0` to `0`.
- Preserve absent fields, explicit null, empty objects, empty lists and empty
  strings as distinct values. Public schemas further restrict where null is legal.
- Validate the entire input before omitting transport fields. Invalid ignored data
  must not be accepted by one SDK but rejected by another.

## Normalization and serialization

1. Normalize all strings and object keys to Unicode NFC.
2. Sort object keys by Unicode scalar value (UTF-8 lexicographic order). JavaScript
   must not use its default UTF-16 sort for this operation.
3. Preserve array order unless its explicit RFC 6901 path is a set in the profile.
   Sets contain strings only; normalize and sort them, rejecting duplicates.
4. Convert explicit timestamp paths to UTC, requiring uppercase `T` and `Z` (or
   a numeric offset), valid calendar fields, no leap seconds and at most nine
   fractional digits. Remove trailing fractional zeros. Reject UTC years outside
   0001–9999. Offsets must have hours 00–23 and minutes 00–59.
5. Omit only paths declared non-authoritative by the versioned protocol profile.
   In particular `/transport` may be omitted from an event, but
   `/operation/arguments/transport` is not the same path.
6. Emit no insignificant whitespace. Escape quote, backslash and JSON control
   characters; use lowercase hex for other controls. Do not escape `/`, HTML
   characters, non-ASCII scalars or U+2028/U+2029.
7. SHA-256 hashes precisely those UTF-8 bytes. Text digests use `sha256:` followed
   by 64 lowercase hexadecimal characters.

Profiles are internal protocol constants, **never options accepted from untrusted
API callers**. The foundation exposes the algorithm and path selection mechanism;
Phase 3 must define and test the complete event/permit binding profiles before
issuing permits. Schema `x-normgate-set` annotations identify set-like fields;
`format: date-time` identifies timestamps. Nested records and arrays require their
own exact paths. A generic empty profile performs canonical JSON only.

These functions do not grant authority or authenticate data. Go contracts must be
validated before generated types are used. Python TypedDict and TypeScript types
provide static transport shapes, not runtime validators. Opaque extension data in
signed objects must obey the same integer and Unicode constraints.

Unicode tables come from each pinned runtime. New runtime versions must pass all
vectors. The Phase 6 compatibility campaign must cover newly assigned Unicode
characters before declaring broad cross-version compatibility.
