# Changelog

## v0.2.0

### Breaking Changes

- **Minimum Go version bumped to 1.25** ([#87](https://github.com/DataDog/go-sqllexer/pull/87))
  The `go.mod` minimum Go version has been raised to Go 1.25. CI now tests through Go 1.25.7.

### Bug Fixes

- **Fix multi-byte UTF-8 character handling** ([#85](https://github.com/DataDog/go-sqllexer/pull/85))
  The lexer now correctly advances by the full rune length when scanning unknown tokens, double-quoted identifiers, and other multi-byte UTF-8 sequences (e.g., full-width punctuation, CJK characters). Previously, multi-byte characters could be incorrectly split into separate byte-level tokens or cause misaligned scans. This includes a fix for truncated UTF-8 sequences at the end of input.

### Performance Improvements

- **Use fixed-size array for trie nodes instead of a hashmap** ([#84](https://github.com/DataDog/go-sqllexer/pull/84))
  The keyword trie's `children` field was changed from `map[rune]*trieNode` to a fixed-size `[27]*trieNode` array (A–Z + underscore). This replaces map lookups with direct array indexing during keyword matching, reducing allocations and improving lexer throughput.

### Enhancements

- **Rework CLI and add missing normalizer option flags** ([#83](https://github.com/DataDog/go-sqllexer/pull/83))
  The `cmd/sqllexer` CLI was refactored for cleaner config plumbing and now exposes all normalizer options as flags:
  - `-keep-identifier-quotation`
  - `-dollar-quoted-func`
  - `-replace-positional-parameter`
  - `-collect-procedures`
  - `-uppercase-keywords`
  - `-remove-space-between-parentheses`
  - `-keep-trailing-semicolon`
