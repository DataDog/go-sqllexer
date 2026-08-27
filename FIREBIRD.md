# FirebirdSQL Support

This document describes what was done to add FirebirdSQL support to go-sqllexer, which
dialect behaviors are implemented, which upstream bugs were fixed along the way, and how
everything was validated against a live Firebird 5.0.3 server.

## Overview

Firebird is selected with the `firebird` DBMS type (alias `firebirdsql`, the name of the
popular Go driver):

```go
lexer := sqllexer.New(query, sqllexer.WithDBMS(sqllexer.DBMSFirebird))
// or
lexer := sqllexer.New(query, sqllexer.WithDBMS(sqllexer.DBMSType("firebirdsql")))
```

Like the other dialects, Firebird support is lexing-level only — the keyword trie, the
obfuscator and the normalizer stay shared; only the places where dialects genuinely
differ were made Firebird-aware.

## Dialect behavior

### String literals — ANSI semantics

Firebird follows standard SQL string rules, so it joined SQL Server and Oracle in the
"no backslash escapes" family:

- The backslash is an ordinary character: `LIKE '%\_%' ESCAPE '\'` terminates correctly
  at its closing quote even when more SQL follows.
- A literal quote inside a string is escaped by doubling it (`''`):
  `'O''Brien'` is a **single** `STRING` token. Previously (all ANSI dialects) it was
  split into two string literals, producing `? ?` instead of `?` after obfuscation.

### Bind parameters

| Form | Token | Notes |
|---|---|---|
| `?` | positional placeholder | driver-converted, as before |
| `:name` | `BIND_PARAMETER` | the common Firebird driver style, e.g. `WHERE SALARY > :min_salary` |
| `@name` | `BIND_PARAMETER` | used by the Firebird .NET provider |

Parameter names may contain multi-byte letters (`:größe`, `:šablon`, `:шаблон`) — see
[Bind parameter UTF-8 fix](#bind-parameter-utf-8-fix) below. Bind parameters are kept
as-is by the obfuscator by default (`WithReplaceBindParameter(true)` replaces them with
`?`).

### The `$` character

- `$` is valid **inside** Firebird identifiers and is preserved there:
  `RDB$RELATION_NAME`, `RDB$RELATIONS`, `MY$VAR` all scan as single `IDENT` tokens.
- A **leading** `$` no longer starts a PostgreSQL-style dollar-quoted string (which
  would otherwise swallow the rest of the query looking for a closing tag) and `$1` is
  no longer treated as a positional parameter — neither exists in Firebird. A leading
  `$` is emitted as `UNKNOWN`.

### Quoted identifiers

`"..."` identifiers are case-sensitive in Firebird (unlike unquoted ones, which are
folded to uppercase). As with PostgreSQL/SQL Server, normalization strips the quotes for
simple identifiers (`"EMPNO"` → `EMPNO`) and keeps them when
`WithKeepIdentifierQuotation(true)` is set. Quoted identifiers may contain non-ASCII
letters and are still recognized as "simple" (strippable).

### Comments

`--` line comments and `/* ... */` block comments work as usual. Block comments are
**not nested** in Firebird (verified against the server — an inner `/*` is just comment
text and the comment ends at the first `*/`), so the standard scanner applies unchanged.

### Keywords and commands

Added to the shared keyword set (reserved words in Firebird, unlikely to collide as
identifiers in other dialects):

| Word | Classification | Why |
|---|---|---|
| `RECREATE` | `COMMAND` | Firebird drop-and-recreate DDL; shows up in statement metadata `commands` |
| `SUSPEND` | `KEYWORD` | PSQL row yield in selectable procedures / `EXECUTE BLOCK` |
| `STARTING` | `KEYWORD` | `STARTING WITH` match operator |
| `CONTAINING` | `KEYWORD` | case-insensitive substring match |
| `SIMILAR` | `KEYWORD` | `SIMILAR TO` regex match |
| `SINGULAR` | `KEYWORD` | singular subquery predicate |
| `LEAVE` | `KEYWORD` | PSQL loop exit |

Deliberately *not* added: `FIRST`/`NEXT`/`VALUE`/`ROWS`/`PLAN` — they are plausible
column/table names in other dialects, and reclassifying them as keywords would silently
change table metadata collection there. They lex as plain identifiers, which is harmless
for Firebird as well.

### Constructs that tokenize cleanly

`FIRST n SKIP m`, `ROWS a TO b`, `OFFSET ... ROWS FETCH NEXT/FIRST ... ROWS/ONLY`,
`INSERT ... RETURNING`, `EXECUTE BLOCK (x TYPE = :param) RETURNS (...) AS BEGIN ... END`,
`EXECUTE PROCEDURE name(...)`, `SELECT ... FROM procedure(...)`, `NEXT VALUE FOR seq`,
`GEN_ID(seq, n)`, hex numerics `0x1F`, binary strings `X'0A'`, charset introducers
`_UTF8'...'`, `||`, `!=`, `IS [NOT] DISTINCT FROM`.

## Fixes to existing dialects made along the way

### Quote-doubling completion for SQL Server and Oracle

Upstream PR #103 stopped treating `\` as a string escape in T-SQL/Oracle but never
implemented the other half of ANSI semantics: doubled quotes. `'don''t'` still lexed as
two string literals, and dynamic SQL mis-lexed badly:

```sql
N'UPDATE orders SET status = ''' + @newStatus + ''' WHERE id = '
```

Obfuscated before: `N ? ? + @newStatus + ? ? + ...` (four placeholders, empty-string
tokens). After: `N ? + @newStatus + ? + ...` — one placeholder per real literal.
The fix applies only to the backslash-free dialects (SQL Server, Oracle, Firebird);
backslash dialects (MySQL, PostgreSQL, Snowflake) keep their exact previous behavior,
including the `''` split. One mssql testdata expectation was updated accordingly.

### Bind parameter UTF-8 fix

`scanBindParameter` advanced one **byte** per iteration, so a parameter whose name
contains a multi-byte letter was split mid-rune:

```
:größe  →  BIND_PARAMETER ":gr" + garbage tokens   (before)
:größe  →  BIND_PARAMETER ":größe"                  (after)
```

The scanner now advances by `utf8.RuneLen(ch)`, matching the existing identifier
scanners. This affected Oracle (`:имя`) and any other dialect using `:name`/`@name`
parameters, not just Firebird. ASCII names are bit-for-bit unaffected.

## Validation against a live Firebird 5.0.3 server

All assumptions were probed with `isql` against the sample `EMPLOYEE` database before
being encoded in the lexer or tests:

| Behavior | Result |
|---|---|
| `'don''t'` doubling, backslash as literal | supported (as implemented) |
| `0x1F` hex numeric, `X'0A'` binary string, `_UTF8'...'` introducer | supported |
| `FIRST n SKIP m`, `ROWS a TO b`, `OFFSET/FETCH` (mutually exclusive families) | supported |
| `CONTAINING`, `STARTING WITH`, `SIMILAR TO`, `IS DISTINCT FROM`, `\|\|`, `!=` | supported |
| `EXECUTE BLOCK ... SUSPEND`, `EXECUTE PROCEDURE`, `SELECT FROM procedure()` | supported |
| `INSERT ... RETURNING` | supported |
| Cyrillic quoted identifiers `"ЗАМЕТКИ"` + Cyrillic string literal | full round-trip works |
| **Nested** block comments `/* /* */ */` | **not supported** — inner `/*` is plain text |
| **`1_000_000` digit separators** | **not supported** (even in 5.0.3) — not implemented |
| **Multi-row `INSERT ... VALUES (...), (...)`** | **not supported** — test case intentionally omitted |

Negative results matter as much as the positive ones: they prevented implementing
features Firebird does not actually have (nested comment scanning, digit separators).

## Test coverage

- `testdata/firebird/` — data-driven end-to-end cases (obfuscate + normalize +
  statement metadata), grounded in the EMPLOYEE schema: `select/` (paging, operators,
  literals, `RDB$` tables, quoted identifiers, comments), `insert/`, `update/`,
  `delete/`, `procedure/` (`EXECUTE BLOCK`, procedures), `ddl/` (`RECREATE`),
  `complex/` (join + aggregate report).
- `testdata/firebird/unicode/` — Russian, Serbian Cyrillic, Serbian Latin, Portuguese,
  Spanish and German sentences covering multi-byte string literals, quoted identifiers
  and bind parameter names. Metadata `size` counts **bytes** (Go `len`), so Cyrillic
  identifiers count 2 bytes per character.
- `sqllexer_test.go` — token-level tests: quote doubling, `ESCAPE '\'`,
  `:name` bind parameters, `$` handling, keyword classification, UTF-8 tokenization,
  plus a guard that backslash dialects (MySQL) keep the old `''` split behavior.
- `normalizer_test.go` — Firebird normalization and metadata cases.

Run everything with:

```bash
go test ./...
```

## Usage

```go
obfuscator := sqllexer.NewObfuscator()
normalizer := sqllexer.NewNormalizer(
    sqllexer.WithCollectTables(true),
    sqllexer.WithCollectCommands(true),
)

sql, metadata, err := sqllexer.ObfuscateAndNormalize(
    "SELECT FIRST 10 SKIP 5 LAST_NAME FROM EMPLOYEE WHERE SALARY > :min_salary",
    obfuscator, normalizer,
    sqllexer.WithDBMS(sqllexer.DBMSFirebird),
)
// SELECT FIRST ? SKIP ? LAST_NAME FROM EMPLOYEE WHERE SALARY > :min_salary
// metadata.Tables == ["EMPLOYEE"], metadata.Commands == ["SELECT"]
```

CLI:

```bash
echo "SELECT * FROM EMPLOYEE WHERE LAST_NAME = 'O''Brien'" | sqllexer -dbms firebird
sqllexer -mode normalize -dbms firebirdsql -input query.sql -with-metadata
```

## Known limitations

- A doubled quote inside a quoted identifier (`"AB""CD"`) is not merged into a single
  identifier — the same pre-existing limitation applies to PostgreSQL upstream.
- Obfuscated `FIRST ? SKIP ?` keeps two placeholders (they are not parenthesized, so
  the normalizer's placeholder grouping does not apply).
- `EXECUTE BLOCK` bodies contribute their inner commands (`BEGIN`, `SELECT`) to
  statement metadata — a consequence of the shared keyword trie, consistent with how
  other dialects report procedure bodies.
