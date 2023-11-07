# Test Suite

The test suite is a collection of test SQL statements that are organized per DBMS. The test suite is used to test the SQL obfuscator and normalizer for correctness and completeness. It is also intended to cover DBMS specific edge cases, that are not covered by the generic unit tests.

## Test Suite Structure

The test suite is organized in the following way:

```text
testdata
├── README.md
├── dbms1
│   ├── query_type1
│   │   ├── test1.sql
│   │   └── test1.expected
│   └── query_type2
│       ├── test1.sql
│       └── test1.expected
dbms_test.go
```

The test suite is organized per DBMS. Each DBMS has a number of query types. Each query type has a number of test cases. Each test case consists of a SQL statement and the expected output of the obfuscator/normalizer.

## Test File Format

The test files are simple text files come in pairs. The first `.sql` file is the SQL statement to be tested. The second `.expected` file is the expected output of the obfuscator/normalizer in json format. Both files have the same name, except for the file extension.

example.sql:

```sql
SELECT * FROM users WHERE id = 1;
```

example.expected:

```json
[
    {
        "expected": "SELECT * FROM users WHERE id = ?",
        "obfuscator_config": {...}, // optional
        "normalizer_config": {...}  // optional
    }
]
```

## How to write a new test case

1. Create a new directory for the DBMS, if it does not exist yet. (this step is often not necessary)
2. Create a new directory for the query type, if it does not exist yet.
3. Create a new `.sql` file with the SQL statement to be tested.
4. Create a new `.expected` file with the expected output of the obfuscator/normalizer in json format.
5. Run the test suite to verify that the test case is working as expected.
