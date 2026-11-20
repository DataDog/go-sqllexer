# Changelog

## v0.2.7

### Bug Fixes

- **Keep lexing after a `$` that does not open a dollar-quoted string** ([#129](https://github.com/DataDog/go-sqllexer/pull/129))
  A `$` that is not the start of a dollar-quoted string (a Snowflake session variable such as `$user_id`, a MySQL identifier starting with `$`, or a bare `$`) is now scanned as an identifier or an unknown token, and lexing continues. Previously the lexer consumed through the next `$`, or through the end of the query when there was none, so literals after the `$` could leak into the obfuscated query or the rest of the statement could be dropped.

- **Match dollar-quoted strings with a non-ASCII tag** ([#130](https://github.com/DataDog/go-sqllexer/pull/130))
  PostgreSQL tags such as `$é$...$é$` now close correctly. The lexer previously compared the closing tag byte by byte against the tag's runes, so a multi-byte character never matched and everything after the opening tag was swallowed.

- **Keep mixed-quote qualified identifiers intact** ([#116](https://github.com/DataDog/go-sqllexer/pull/116))
  Names such as `schema."table-name"` are now one qualified identifier for dialects other than MySQL. A double quote opens a quoted component only when it comes immediately after a dot.

- **Obfuscate signed prefixed numbers and digit separators** ([#126](https://github.com/DataDog/go-sqllexer/pull/126))
  A leading sign is scanned together with `0x`, `0b`, and `0o`, so `-0b1010`, `+0o17`, and `-0x1F` obfuscate to `?`. A PostgreSQL underscore stays inside a decimal, hex, binary, or octal literal when a digit of that base follows it (`0b_1010`). A trailing or doubled underscore stays outside the number, and the legacy unsigned `0123` octal form is unchanged for signed values.

- **Obfuscate binary and octal prefixed number literals** ([#125](https://github.com/DataDog/go-sqllexer/pull/125))
  `0b`/`0B` and `0o`/`0O` literals are now a single number token, the same way `0x` already was, in every dialect. Previously the leading `0` was replaced and the rest of the value (`b1010`, `o755`) stayed in the query. The prefix counts only when a valid digit follows it.

- **Obfuscate Oracle alternative quoted strings** ([#120](https://github.com/DataDog/go-sqllexer/pull/120))
  With the Oracle dialect, `q'[...]'` and `nq'[...]'` (any case) are scanned as one string through the matching closing delimiter, so quotes inside the literal no longer leak into the obfuscated query. `[`, `{`, `<`, and `(` close with the matching bracket; any other character closes with itself. A truncated alternative quote is an incomplete string and is still replaced. Other dialects are unchanged.

- **Preserve `CAST ... AS type` when removing aliases** ([#123](https://github.com/DataDog/go-sqllexer/pull/123))
  Alias removal no longer drops the `AS` and type from `CAST(expr AS type)`, including multi-word types such as `TIMESTAMP WITH TIME ZONE`. The same applies to `TRY_CAST`, `SAFE_CAST`, `XMLCAST`, `TREAT`, `PARSE`, `TRY_PARSE`, and `XMLSERIALIZE`. Aliases outside the cast are still removed. Normalized output changes for these expressions, so query signatures derived from it will change.

- **Preserve `AS MATERIALIZED` on CTEs when removing aliases** ([#122](https://github.com/DataDog/go-sqllexer/pull/122))
  `WITH c AS MATERIALIZED (...)` no longer loses `AS MATERIALIZED` during alias removal. `AS (` and `AS NOT MATERIALIZED` were already preserved. A leading parenthesized statement such as `(WITH c AS (SELECT 1) SELECT * FROM c)` also no longer gains a stray leading `AS`. Normalized output changes for these queries, so query signatures derived from it will change.

- **Obfuscate truncated dollar-quoted strings** ([#118](https://github.com/DataDog/go-sqllexer/pull/118))
  A dollar-quoted string whose opening tag is complete but whose closing tag is missing is now an incomplete string and is replaced with `?`. Previously it was emitted as an `ERROR` token and written through unchanged, so a truncated query such as `$$hunter2` kept the literal. A `$` whose tag itself never closes is unchanged by this fix.

### Performance Improvements

- **Keep the quote check out of the identifier loop** ([#127](https://github.com/DataDog/go-sqllexer/pull/127))
  Ordinary identifier characters are scanned without a second double-quote comparison on every character. Quoted components after a dot are handled only when the scan stops on a quote. Output and allocations are unchanged.

### Enhancements

- **Add opt-in PostgreSQL `IS` predicate literal preservation** ([#114](https://github.com/DataDog/go-sqllexer/pull/114))
  `WithKeepISPredicate(true)` (CLI `-keep-is-predicate`, JSON `keep_is_predicate`) preserves `TRUE`, `FALSE`, and `NULL` on the right of `IS` and `IS NOT` for PostgreSQL. It is off by default. `IS [NOT] DISTINCT FROM`, other dialects, ordinary literals, and the existing `keep_boolean` and `keep_null` behavior are unchanged.

## v0.2.6

### Bug Fixes

- **Handle IBM Db2 LUW string literals correctly** ([#115](https://github.com/DataDog/go-sqllexer/pull/115))
  Recognize the `ibm_db2` dialect and treat backslashes as ordinary characters inside strings, preserving literal boundaries and preventing literal values from appearing in normalized query text.

## v0.2.5

### Bug Fixes

- **Fix truncated obfuscation of PostgreSQL strings containing backslashes** ([#110](https://github.com/DataDog/go-sqllexer/pull/110))
  Backslash is now treated as a normal character in PostgreSQL strings (except `E'...'` strings), so these strings no longer truncate the obfuscated query. In all dialects, a doubled quote (`''`) inside a string now stays part of one literal, which can slightly change obfuscated output.

- **Keep MySQL qualified identifiers intact when spaced around dots** ([#108](https://github.com/DataDog/go-sqllexer/pull/108))
  Names like `db . table` (as found in `performance_schema` digest text) are now normalized to `db.table` and reported as a single table in metadata.

## v0.2.4

### Bug Fixes

- **Don't treat backslash as a string escape in SQL Server and Oracle** ([#103](https://github.com/DataDog/go-sqllexer/pull/103))
  SQL Server (T-SQL) and Oracle follow ANSI-SQL string semantics, where backslash is an ordinary character and a quote inside a literal is escaped by doubling it (`''`). The lexer previously treated `\` as an escape character for all dialects, so a literal like `ESCAPE '\'` had its closing quote misread as escaped, causing the scan to swallow the rest of the batch up to the next quote and truncate the obfuscated query.

## v0.2.3

### Bug Fixes

- **Preserve bracket-quoted T-SQL identifiers containing spaces** ([#101](https://github.com/DataDog/go-sqllexer/pull/101))
  Bracket-quoted identifiers whose content contains whitespace (e.g. `[Column With Spaces]`) are no longer de-bracketed during normalization. Stripping the brackets produces bare spaces in identifier position, which breaks query structure. Simple identifiers (`[schema]`, `[table]`) and dot-joined multi-part identifiers (`[schema].[table]`) are unaffected and continue to be de-bracketed as before. A secondary fix suppresses the spurious space that the normalizer was inserting between a dot-suffixed token and the bracket-quoted identifier that follows it (e.g. `t. [Col]` → `t.[Col]`).

## v0.2.2

### Bug Fixes

- **Obfuscate EXTRACT field keywords** ([#98](https://github.com/DataDog/go-sqllexer/pull/98))
  The obfuscator now replaces the field argument of `EXTRACT(field FROM source)` with a placeholder when it matches a known PostgreSQL field name (`epoch`, `year`, `month`, `dow`, `isodow`, `microseconds`, `timezone_hour`, etc.). Previously, queries captured via `pg_stat_activity` kept `epoch` as an unquoted identifier while queries from `pg_stat_statements` had it normalized to `$1` (and then to `?`), splitting one logical query across two DBM signatures. Both code paths now converge on `EXTRACT(? FROM source)`. Bare `epoch`/`year`/etc. outside an `EXTRACT(...)` call (e.g. as a column reference) and unrecognized field names are left untouched.

- **Fix handling of PostgreSQL VACUUM commands** ([#96](https://github.com/DataDog/go-sqllexer/pull/96))
  Fixes a typo and reclassifies `VACUUM` from a generic keyword to a command so it is correctly extracted into statement metadata.

- **Handle multiline comment after keyword** ([#89](https://github.com/DataDog/go-sqllexer/pull/89))
  The lexer now correctly tokenizes SQL keywords when they are directly followed by a multiline comment (e.g. `select/**/*/**/from/**/events`). Previously, the leading `/` of the comment could be absorbed into the preceding token, breaking keyword detection.

### Performance Improvements

- **Avoid allocation in `isExtractFieldKeyword`** ([#99](https://github.com/DataDog/go-sqllexer/pull/99))
  Replaces a `strings.ToUpper` + map lookup with an allocation-free `strings.EqualFold` scan over a small slice of EXTRACT field names.

## v0.2.1

### Bug Fixes

- **Fix table name metadata extraction** ([#91](https://github.com/DataDog/go-sqllexer/pull/91))
  The normalizer now correctly extracts all table names from comma-separated table lists (e.g., `SELECT * FROM t1, t2`). Previously, only the first table after a table indicator keyword was collected. This also adds `LATERAL` as a recognized keyword so it is no longer misidentified as a table name during metadata extraction.

### Maintenance

- **Pin GitHub Actions** ([#90](https://github.com/DataDog/go-sqllexer/pull/90))

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
