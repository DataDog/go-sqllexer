//! The harness protocol's `Text` encoding.
//!
//! JSON strings cannot carry invalid UTF-8, but the lexer must accept arbitrary
//! bytes, so the protocol encodes valid UTF-8 as a plain string and anything else
//! as `{"b64": "..."}`.

use base64::engine::general_purpose::STANDARD;
use base64::Engine;
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

impl Serialize for Text {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        match std::str::from_utf8(&self.0) {
            Ok(s) => serializer.serialize_str(s),
            Err(_) => {
                use serde::ser::SerializeMap;
                let mut map = serializer.serialize_map(Some(1))?;
                map.serialize_entry("b64", &STANDARD.encode(&self.0))?;
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
                        out = Some(STANDARD.decode(&value).map_err(de::Error::custom)?);
                    }
                }
                out.map(Text).ok_or_else(|| de::Error::missing_field("b64"))
            }
        }

        deserializer.deserialize_any(TextVisitor)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
