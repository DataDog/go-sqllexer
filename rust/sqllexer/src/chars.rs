//! Rune decoding and character classification.
//!
//! Input is handled as raw bytes rather than `str`: the Go implementation accepts
//! arbitrary byte sequences and its behavior on invalid UTF-8 is observable (an
//! invalid byte decodes to U+FFFD but advances the cursor by one byte, while
//! `utf8.RuneLen(U+FFFD)` is 3, so some scanners then skip three bytes). Those
//! quirks are part of the compatibility contract, so they are reproduced here
//! instead of being normalized away.

use crate::unicode_tables::{LETTER_RANGES, NUMBER_RANGES, SPACE_RANGES};

/// Sentinel for "no more input", matching the Go lexer's use of rune 0.
pub(crate) const EOF: u32 = 0;
pub(crate) const RUNE_ERROR: u32 = 0xFFFD;

/// Decodes the rune at `pos`, mirroring `utf8.DecodeRuneInString`: invalid,
/// overlong, surrogate and out-of-range sequences yield `(U+FFFD, 1)`.
#[inline]
pub(crate) fn decode_rune(src: &[u8], pos: usize) -> (u32, usize) {
    let Some(&b0) = src.get(pos) else {
        return (RUNE_ERROR, 0);
    };
    if b0 < 0x80 {
        return (b0 as u32, 1);
    }
    let rest = &src[pos + 1..];
    let cont = |i: usize| -> Option<u32> {
        match rest.get(i) {
            Some(&b) if (0x80..0xC0).contains(&b) => Some((b & 0x3F) as u32),
            _ => None,
        }
    };
    match b0 {
        0xC2..=0xDF => match cont(0) {
            Some(c1) => (((b0 & 0x1F) as u32) << 6 | c1, 2),
            None => (RUNE_ERROR, 1),
        },
        0xE0..=0xEF => match (cont(0), cont(1)) {
            (Some(c1), Some(c2)) => {
                let r = ((b0 & 0x0F) as u32) << 12 | c1 << 6 | c2;
                // Reject overlong encodings and UTF-16 surrogate halves.
                if r < 0x800 || (0xD800..0xE000).contains(&r) {
                    (RUNE_ERROR, 1)
                } else {
                    (r, 3)
                }
            }
            _ => (RUNE_ERROR, 1),
        },
        0xF0..=0xF4 => match (cont(0), cont(1), cont(2)) {
            (Some(c1), Some(c2), Some(c3)) => {
                let r = ((b0 & 0x07) as u32) << 18 | c1 << 12 | c2 << 6 | c3;
                if !(0x10000..=0x10FFFF).contains(&r) {
                    (RUNE_ERROR, 1)
                } else {
                    (r, 4)
                }
            }
            _ => (RUNE_ERROR, 1),
        },
        _ => (RUNE_ERROR, 1),
    }
}

/// Encoded length of a rune, mirroring `utf8.RuneLen`. U+FFFD is 3 bytes, which is
/// why an invalid byte can advance the identifier scanner by three positions.
#[inline]
pub(crate) fn rune_len(ch: u32) -> usize {
    match ch {
        0..=0x7F => 1,
        0x80..=0x7FF => 2,
        0x800..=0xFFFF => 3,
        _ => 4,
    }
}

/// Appends a rune to `out` as UTF-8, mirroring `strings.Builder.WriteRune`.
pub(crate) fn write_rune(out: &mut Vec<u8>, ch: u32) {
    match ch {
        0..=0x7F => out.push(ch as u8),
        0x80..=0x7FF => out.extend_from_slice(&[0xC0 | (ch >> 6) as u8, 0x80 | (ch & 0x3F) as u8]),
        0x800..=0xFFFF => out.extend_from_slice(&[
            0xE0 | (ch >> 12) as u8,
            0x80 | ((ch >> 6) & 0x3F) as u8,
            0x80 | (ch & 0x3F) as u8,
        ]),
        _ => out.extend_from_slice(&[
            0xF0 | (ch >> 18) as u8,
            0x80 | ((ch >> 12) & 0x3F) as u8,
            0x80 | ((ch >> 6) & 0x3F) as u8,
            0x80 | (ch & 0x3F) as u8,
        ]),
    }
}

/// Iterates runes the way Go's `for _, r := range s` does: invalid bytes are
/// yielded individually as U+FFFD.
pub(crate) struct Runes<'a> {
    src: &'a [u8],
    pos: usize,
}

impl<'a> Runes<'a> {
    pub(crate) fn new(src: &'a [u8]) -> Self {
        Runes { src, pos: 0 }
    }
}

impl Iterator for Runes<'_> {
    type Item = u32;

    fn next(&mut self) -> Option<u32> {
        if self.pos >= self.src.len() {
            return None;
        }
        let (ch, size) = decode_rune(self.src, self.pos);
        self.pos += size.max(1);
        Some(ch)
    }
}

fn in_ranges(ranges: &[(u32, u32)], ch: u32) -> bool {
    ranges
        .binary_search_by(|&(lo, hi)| {
            if ch < lo {
                std::cmp::Ordering::Greater
            } else if ch > hi {
                std::cmp::Ordering::Less
            } else {
                std::cmp::Ordering::Equal
            }
        })
        .is_ok()
}

#[inline]
pub(crate) fn is_digit(ch: u32) -> bool {
    (b'0' as u32..=b'9' as u32).contains(&ch)
}

#[inline]
pub(crate) fn is_leading_sign(ch: u32) -> bool {
    ch == b'+' as u32 || ch == b'-' as u32
}

#[inline]
pub(crate) fn is_exponent(ch: u32) -> bool {
    ch == b'e' as u32 || ch == b'E' as u32
}

#[inline]
pub(crate) fn is_space(ch: u32) -> bool {
    ch == b' ' as u32 || ch == b'\t' as u32 || ch == b'\n' as u32 || ch == b'\r' as u32
}

#[inline]
pub(crate) fn is_ascii_letter(ch: u32) -> bool {
    (b'a' as u32..=b'z' as u32).contains(&ch) || (b'A' as u32..=b'Z' as u32).contains(&ch)
}

#[inline]
pub(crate) fn is_letter(ch: u32) -> bool {
    is_ascii_letter(ch) || ch == b'_' as u32 || (ch > 127 && in_ranges(&LETTER_RANGES, ch))
}

#[inline]
pub(crate) fn is_alpha_numeric(ch: u32) -> bool {
    is_letter(ch) || is_digit(ch) || (ch > 127 && in_ranges(&NUMBER_RANGES, ch))
}

#[inline]
pub(crate) fn is_double_quote(ch: u32) -> bool {
    ch == b'"' as u32
}

#[inline]
pub(crate) fn is_single_quote(ch: u32) -> bool {
    ch == b'\'' as u32
}

#[inline]
pub(crate) fn is_operator(ch: u32) -> bool {
    matches!(
        ch as u8 as char,
        '+' | '-'
            | '*'
            | '/'
            | '='
            | '<'
            | '>'
            | '!'
            | '&'
            | '|'
            | '^'
            | '%'
            | '~'
            | '?'
            | '@'
            | ':'
            | '#'
    ) && ch < 128
}

#[inline]
pub(crate) fn is_wildcard(ch: u32) -> bool {
    ch == b'*' as u32
}

#[inline]
pub(crate) fn is_single_line_comment(ch: u32, next_ch: u32) -> bool {
    ch == b'-' as u32 && next_ch == b'-' as u32
}

#[inline]
pub(crate) fn is_multi_line_comment(ch: u32, next_ch: u32) -> bool {
    ch == b'/' as u32 && next_ch == b'*' as u32
}

#[inline]
pub(crate) fn is_punctuation(ch: u32) -> bool {
    matches!(
        ch as u8 as char,
        '(' | ')' | ',' | ';' | '.' | ':' | '[' | ']' | '{' | '}'
    ) && ch < 128
}

#[inline]
pub(crate) fn is_eof(ch: u32) -> bool {
    ch == EOF
}

/// Unicode-aware space test, mirroring `unicode.IsSpace` (used by `strings.TrimSpace`,
/// which is applied to obfuscated and normalized output).
#[inline]
pub(crate) fn is_unicode_space(ch: u32) -> bool {
    match ch {
        0x09..=0x0D | 0x20 => true,
        0..=0x7F => false,
        _ => in_ranges(&SPACE_RANGES, ch),
    }
}

/// Decodes the rune ending at `end`, mirroring `utf8.DecodeLastRuneInString`.
fn decode_last_rune(src: &[u8], end: usize) -> (u32, usize) {
    if end == 0 {
        return (RUNE_ERROR, 0);
    }
    let b = src[end - 1];
    if b < 0x80 {
        return (b as u32, 1);
    }
    // Scan back to the most recent lead byte, at most the max sequence length.
    let lim = end.saturating_sub(4);
    let mut start = end - 1;
    while start > lim && (src[start] & 0xC0) == 0x80 {
        start -= 1;
    }
    let (ch, size) = decode_rune(src, start);
    if start + size != end {
        return (RUNE_ERROR, 1);
    }
    (ch, size)
}

/// Byte range `strings.TrimSpace` would keep. Trimming is always done in place,
/// so the range is returned instead of a subslice.
pub(crate) fn trim_space_range(src: &[u8]) -> (usize, usize) {
    let mut start = 0;
    while start < src.len() {
        let (ch, size) = decode_rune(src, start);
        if !is_unicode_space(ch) {
            break;
        }
        start += size.max(1);
    }
    let mut end = src.len();
    while end > start {
        let (ch, size) = decode_last_rune(src, end);
        if !is_unicode_space(ch) {
            break;
        }
        end -= size.max(1);
    }
    (start, end)
}

/// `strings.EqualFold` against an ASCII-only needle. Two non-ASCII runes case-fold
/// onto ASCII letters (U+017F to `s`, U+212A to `k`) and Go's comparison honors
/// them, so they are handled explicitly.
pub(crate) fn equal_fold_ascii(value: &[u8], needle: &[u8]) -> bool {
    debug_assert!(needle.is_ascii());
    let mut runes = Runes::new(value);
    for &n in needle {
        let Some(ch) = runes.next() else {
            return false;
        };
        let folded = match ch {
            0x017F => b's' as u32,
            0x212A => b'k' as u32,
            _ if ch < 128 => (ch as u8).to_ascii_lowercase() as u32,
            _ => return false,
        };
        if folded != n.to_ascii_lowercase() as u32 {
            return false;
        }
    }
    runes.next().is_none()
}

#[inline]
pub(crate) fn is_identifier(ch: u32) -> bool {
    (ch < 128
        && matches!(
            ch as u8 as char,
            '"' | '.' | '?' | '$' | '#' | '/' | '@' | '!'
        ))
        || is_letter(ch)
        || is_digit(ch)
}
