//! The harness protocol's `Text` encoding.
//!
//! JSON strings cannot carry invalid UTF-8, but the lexer must accept arbitrary
//! bytes, so the protocol encodes valid UTF-8 as a plain string and anything else
//! as `{"b64": "..."}`. Base64 is implemented here rather than pulled in as a
//! dependency: it is a few lines and keeps the harness binaries trivial to audit.

use serde::de::{self, Deserializer, Visitor};
use serde::{Deserialize, Serialize, Serializer};
use std::fmt;

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Text(pub Vec<u8>);

impl Text {
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl From<Vec<u8>> for Text {
    fn from(v: Vec<u8>) -> Self {
        Text(v)
    }
}

impl From<&[u8]> for Text {
    fn from(v: &[u8]) -> Self {
        Text(v.to_vec())
    }
}

impl Serialize for Text {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        match std::str::from_utf8(&self.0) {
            Ok(s) => serializer.serialize_str(s),
            Err(_) => {
                use serde::ser::SerializeMap;
                let mut map = serializer.serialize_map(Some(1))?;
                map.serialize_entry("b64", &encode_base64(&self.0))?;
                map.end()
            }
        }
    }
}

impl<'de> Deserialize<'de> for Text {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct TextVisitor;

        impl<'de> Visitor<'de> for TextVisitor {
            type Value = Text;

            fn expecting(&self, f: &mut fmt::Formatter) -> fmt::Result {
                f.write_str("a string or a {\"b64\": \"...\"} object")
            }

            fn visit_str<E: de::Error>(self, v: &str) -> Result<Text, E> {
                Ok(Text(v.as_bytes().to_vec()))
            }

            fn visit_map<A: de::MapAccess<'de>>(self, mut map: A) -> Result<Text, A::Error> {
                let mut out = None;
                while let Some(key) = map.next_key::<String>()? {
                    let value: String = map.next_value()?;
                    if key == "b64" {
                        out = Some(decode_base64(&value).map_err(de::Error::custom)?);
                    }
                }
                out.map(Text).ok_or_else(|| de::Error::missing_field("b64"))
            }
        }

        deserializer.deserialize_any(TextVisitor)
    }
}

const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

pub fn encode_base64(input: &[u8]) -> String {
    let mut out = String::with_capacity(input.len().div_ceil(3) * 4);
    for chunk in input.chunks(3) {
        let b = [
            chunk[0],
            *chunk.get(1).unwrap_or(&0),
            *chunk.get(2).unwrap_or(&0),
        ];
        let n = (b[0] as u32) << 16 | (b[1] as u32) << 8 | b[2] as u32;
        out.push(ALPHABET[(n >> 18) as usize & 63] as char);
        out.push(ALPHABET[(n >> 12) as usize & 63] as char);
        out.push(if chunk.len() > 1 {
            ALPHABET[(n >> 6) as usize & 63] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            ALPHABET[n as usize & 63] as char
        } else {
            '='
        });
    }
    out
}

pub fn decode_base64(input: &str) -> Result<Vec<u8>, String> {
    let mut out = Vec::with_capacity(input.len() / 4 * 3);
    let mut acc: u32 = 0;
    let mut bits = 0;
    for c in input.bytes() {
        if c == b'=' || c == b'\n' || c == b'\r' {
            continue;
        }
        let v = ALPHABET
            .iter()
            .position(|&a| a == c)
            .ok_or_else(|| format!("invalid base64 character {:?}", c as char))?;
        acc = acc << 6 | v as u32;
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push((acc >> bits) as u8);
        }
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn base64_round_trips_arbitrary_bytes() {
        for len in 0..64usize {
            let bytes: Vec<u8> = (0..len).map(|i| (i * 37 % 256) as u8).collect();
            assert_eq!(decode_base64(&encode_base64(&bytes)).unwrap(), bytes);
        }
    }

    #[test]
    fn invalid_utf8_survives_a_json_round_trip() {
        let text = Text(vec![0x41, 0xff, 0xfe, 0x42]);
        let encoded = serde_json::to_string(&text).unwrap();
        assert_eq!(encoded, r#"{"b64":"Qf/+Qg=="}"#);
        let decoded: Text = serde_json::from_str(&encoded).unwrap();
        assert_eq!(decoded, text);
    }

    #[test]
    fn valid_utf8_encodes_as_a_plain_string() {
        let text = Text("sélect".as_bytes().to_vec());
        let encoded = serde_json::to_string(&text).unwrap();
        assert_eq!(encoded, "\"sélect\"");
        let decoded: Text = serde_json::from_str(&encoded).unwrap();
        assert_eq!(decoded, text);
    }
}
