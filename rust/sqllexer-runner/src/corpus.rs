//! Reader for the shared JSONL corpora, so the Rust benchmark replays exactly the
//! statements the Go benchmark replays.

use std::fs::File;
use std::io::{self, BufRead, BufReader};

use serde::Deserialize;
use sqllexer::Dbms;

use crate::text::Text;

#[derive(Debug, Deserialize)]
struct WireEntry {
    sql: Text,
    #[serde(default)]
    dbms: String,
}

/// One replayable statement.
#[derive(Debug, Clone)]
pub struct Entry {
    pub sql: Vec<u8>,
    pub dbms: Dbms,
}

/// Reads every statement of a corpus file, ignoring the fields that only matter
/// to the differential runner.
pub fn read(path: &str) -> io::Result<Vec<Entry>> {
    let reader = BufReader::new(File::open(path)?);
    let mut entries = Vec::new();
    for line in reader.lines() {
        let line = line?;
        if line.is_empty() {
            continue;
        }
        let entry: WireEntry = serde_json::from_str(&line)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))?;
        entries.push(Entry {
            sql: entry.sql.0,
            dbms: dbms_from_name(&entry.dbms),
        });
    }
    Ok(entries)
}

/// Resolves a DBMS name the way `WithDBMS` does, aliases included.
pub fn dbms_from_name(name: &str) -> Dbms {
    match name {
        "mssql" | "sql-server" | "sqlserver" => Dbms::SqlServer,
        "postgresql" | "postgres" => Dbms::Postgres,
        "mysql" => Dbms::MySql,
        "oracle" => Dbms::Oracle,
        "snowflake" => Dbms::Snowflake,
        _ => Dbms::None,
    }
}
