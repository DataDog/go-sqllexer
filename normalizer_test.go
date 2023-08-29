package sqllexer

import (
	"reflect"
	"testing"
)

func TestNormalizer(t *testing.T) {
	tests := []struct {
		input          string
		want           string
		normalizedInfo NormalizedInfo
	}{
		{
			input: "select ?",
			want:  "select ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: "SELECT * FROM users where id = ?",
			want:  "SELECT * FROM users where id = ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: "SELECT * FROM users where id in (?, ?) and name in ARRAY[?, ?]",
			want:  "SELECT * FROM users where id in (?) and name in ARRAY[?]",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `
			SELECT h.id, h.org_id, h.name, ha.name as alias, h.created 
			FROM vs?.host h 
				JOIN vs?.host_alias ha on ha.host_id = h.id 
			WHERE ha.org_id = ? AND ha.name = ANY ( ?, ? )
			`,
			want: "SELECT h.id, h.org_id, h.name, ha.name, h.created FROM vs?.host h JOIN vs?.host_alias ha on ha.host_id = h.id WHERE ha.org_id = ? AND ha.name = ANY (?)",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"vs?.host", "vs?.host_alias"},
				Comments: []string{},
				Commands: []string{"SELECT", "JOIN"},
			},
		},
		{
			input: "/* this is a comment */ SELECT * FROM users where id = ?",
			want:  "SELECT * FROM users where id = ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{"/* this is a comment */"},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `
			/* this is a comment */
			SELECT * FROM users /* comment comment */ where id = ?
			/* this is another comment */
			`,
			want: "SELECT * FROM users where id = ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{"/* this is a comment */", "/* comment comment */", "/* this is another comment */"},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: "SELECT u.id as ID, u.name as Name FROM users as u where u.id = ?",
			want:  "SELECT u.id, u.name FROM users where u.id = ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: "UPDATE users SET name = (SELECT name FROM test_users where id = ?) where id = ?",
			want:  "UPDATE users SET name = (SELECT name FROM test_users where id = ?) where id = ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users", "test_users"},
				Comments: []string{},
				Commands: []string{"UPDATE", "SELECT"},
			},
		},
		{
			input: `
			INSERT INTO order_status_change ( dbm_order_id, message, price, state ) 
			VALUES ( ( 
				SELECT id as dbm_order_id 
				FROM dbm_order 
				WHERE id = ? 
			) ( 
				SELECT ( t.price * t.quantity * d.discount_percent ) AS price 
				FROM dbm_order o 
					JOIN order_item t ON o.id = t.dbm_order_id 
					JOIN discount d ON d.dbm_item_id = t.id 
				WHERE o.id = ? 
				LIMIT ? 
			) )`,
			want: "INSERT INTO order_status_change ( dbm_order_id, message, price, state ) VALUES ( ( SELECT id FROM dbm_order WHERE id = ? ) ( SELECT ( t.price * t.quantity * d.discount_percent ) FROM dbm_order o JOIN order_item t ON o.id = t.dbm_order_id JOIN discount d ON d.dbm_item_id = t.id WHERE o.id = ? LIMIT ? ) )",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"order_status_change", "dbm_order", "order_item", "discount"},
				Comments: []string{},
				Commands: []string{"INSERT", "SELECT", "JOIN"},
			},
		},
		{
			input: "DELETE FROM users where id in (?, ?)",
			want:  "DELETE FROM users where id in (?)",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"DELETE"},
			},
		},
		{
			input: `
			CREATE PROCEDURE test_procedure()
			BEGIN
				SELECT * FROM users where id = ?;
				Update test_users set name = ? where id = ?;
				Delete from user? where id = ?;
			END
			`,
			want: "CREATE PROCEDURE test_procedure() BEGIN SELECT * FROM users where id = ?; Update test_users set name = ? where id = ?; Delete from user? where id = ?; END",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users", "test_users", "user?"},
				Comments: []string{},
				Commands: []string{"CREATE", "BEGIN", "SELECT", "UPDATE", "DELETE"},
			},
		},
		{
			input: `
			SELECT org_id, resource_type, meta_key, meta_value 
			FROM public.schema_meta 
			WHERE org_id in ( ? ) AND resource_type in ( ? ) AND meta_key in ( ? )
			`,
			want: "SELECT org_id, resource_type, meta_key, meta_value FROM public.schema_meta WHERE org_id in (?) AND resource_type in (?) AND meta_key in (?)",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"public.schema_meta"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `
			WITH cte AS (
				SELECT id, name, age
				FROM person
				WHERE age > ?
			  )
			UPDATE person
			SET age = ?
			WHERE id IN (SELECT id FROM cte);
			INSERT INTO person (name, age)
			SELECT name, ?
			FROM cte
			WHERE age <= ?;
			`,
			want: "WITH cte AS ( SELECT id, name, age FROM person WHERE age > ? ) UPDATE person SET age = ? WHERE id IN (SELECT id FROM cte); INSERT INTO person (name, age) SELECT name, ? FROM cte WHERE age <= ?;",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"person", "cte"},
				Comments: []string{},
				Commands: []string{"SELECT", "UPDATE", "INSERT"},
			},
		},
		{
			input: "WITH updates AS ( UPDATE metrics_metadata SET metric_type = ? updated = ? :: timestamp, interval = ? unit_id = ? per_unit_id = ? description = ? orientation = ? integration = ? short_name = ? WHERE metric_key = ? AND org_id = ? RETURNING ? ) INSERT INTO metrics_metadata ( org_id, metric_key, metric_type, interval, unit_id, per_unit_id, description, orientation, integration, short_name ) SELECT ? WHERE NOT EXISTS ( SELECT ? FROM updates )",
			want:  "WITH updates AS ( UPDATE metrics_metadata SET metric_type = ? updated = ? :: timestamp, interval = ? unit_id = ? per_unit_id = ? description = ? orientation = ? integration = ? short_name = ? WHERE metric_key = ? AND org_id = ? RETURNING ? ) INSERT INTO metrics_metadata ( org_id, metric_key, metric_type, interval, unit_id, per_unit_id, description, orientation, integration, short_name ) SELECT ? WHERE NOT EXISTS ( SELECT ? FROM updates )",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"metrics_metadata", "updates"},
				Comments: []string{},
				Commands: []string{"UPDATE", "INSERT", "SELECT"},
			},
		},
		{
			input: `
			/* Multi-line comment */
			SELECT * FROM clients WHERE (clients.first_name = ?) LIMIT ? BEGIN INSERT INTO owners (created_at, first_name, locked, orders_count, updated_at) VALUES (?, ?, ?, ?, ?) COMMIT`,
			want: "SELECT * FROM clients WHERE (clients.first_name = ?) LIMIT ? BEGIN INSERT INTO owners (created_at, first_name, locked, orders_count, updated_at) VALUES (?) COMMIT",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"clients", "owners"},
				Comments: []string{"/* Multi-line comment */"},
				Commands: []string{"SELECT", "BEGIN", "INSERT", "COMMIT"},
			},
		},
		{
			input: `-- Single line comment
			-- Another single line comment
			-- Another another single line comment
			GRANT USAGE, DELETE ON SCHEMA datadog TO datadog`,
			want: "GRANT USAGE, DELETE ON SCHEMA datadog TO datadog",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{"-- Single line comment", "-- Another single line comment", "-- Another another single line comment"},
				Commands: []string{"GRANT", "DELETE"},
			},
		},
		{
			input: `-- Testing table value constructor SQL expression
			SELECT * FROM (VALUES (?, ?)) AS d (id, animal)`,
			want: "SELECT * FROM (VALUES (?)) (id, animal)",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{"-- Testing table value constructor SQL expression"},
				Commands: []string{"SELECT"},
			},
		},
		{
			input: `ALTER TABLE table DROP COLUMN column`,
			want:  "ALTER TABLE table DROP COLUMN column",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"table"},
				Comments: []string{},
				Commands: []string{"ALTER", "DROP"},
			},
		},
		{
			input: `REVOKE ALL ON SCHEMA datadog FROM datadog`,
			want:  "REVOKE ALL ON SCHEMA datadog FROM datadog",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"datadog"},
				Comments: []string{},
				Commands: []string{"REVOKE"},
			},
		},
		{
			input: "/* Testing explicit table SQL expression */ WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE CITY = ?), T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, ? * WEIGHT AS NEW_WEIGHT, ? AS NEW_CITY FROM T1), T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT AS WEIGHT, NEW_CITY AS CITY FROM T2), T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1) TABLE T4 UNION CORRESPONDING TABLE T3",
			want:  "WITH T1 AS (SELECT PNO , PNAME , COLOR , WEIGHT , CITY FROM P WHERE CITY = ?), T2 AS (SELECT PNO, PNAME, COLOR, WEIGHT, CITY, ? * WEIGHT, ? FROM T1), T3 AS ( SELECT PNO , PNAME, COLOR, NEW_WEIGHT, NEW_CITY FROM T2), T4 AS ( TABLE P EXCEPT CORRESPONDING TABLE T1) TABLE T4 UNION CORRESPONDING TABLE T3",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"P", "T1", "T2", "T4", "T3"},
				Comments: []string{"/* Testing explicit table SQL expression */"},
				Commands: []string{"SELECT"},
			},
		},
		{
			// truncated
			input: "SELECT * FROM users where id =",
			want:  "SELECT * FROM users where id =",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{"users"},
				Comments: []string{},
				Commands: []string{"SELECT"},
			},
		},
	}

	normalizer := NewSQLNormalizer(&SQLNormalizerConfig{
		CollectComments: true,
		CollectCommands: true,
		TableNames:      true,
		KeepSQLAlias:    false,
	})

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			got, normalizedInfo, err := normalizer.Normalize(test.input)
			if err != nil {
				t.Errorf("error during normalization: %v", err)
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
			if !reflect.DeepEqual(normalizedInfo.Commands, test.normalizedInfo.Commands) {
				t.Errorf("got %v, want %v", normalizedInfo.Commands, test.normalizedInfo.Commands)
			}
			if !reflect.DeepEqual(normalizedInfo.Comments, test.normalizedInfo.Comments) {
				t.Errorf("got %v, want %v", normalizedInfo.Comments, test.normalizedInfo.Comments)
			}
			if !reflect.DeepEqual(normalizedInfo.Tables, test.normalizedInfo.Tables) {
				t.Errorf("got %v, want %v", normalizedInfo.Tables, test.normalizedInfo.Tables)
			}
		})
	}
}

func TestNormalizerNotCollectMetadata(t *testing.T) {
	tests := []struct {
		input          string
		want           string
		normalizedInfo NormalizedInfo
	}{
		{
			input: "select ?",
			want:  "select ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{},
			},
		},
		{
			input: "SELECT * FROM users where id = ?",
			want:  "SELECT * FROM users where id = ?",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{},
			},
		},
		{
			input: "SELECT id as ID, name as Name from users where id in (?, ?)",
			want:  "SELECT id as ID, name as Name from users where id in (?)",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{},
			},
		},
		{
			input: `TRUNCATE TABLE datadog`,
			want:  "TRUNCATE TABLE datadog",
			normalizedInfo: NormalizedInfo{
				Tables:   []string{},
				Comments: []string{},
				Commands: []string{},
			},
		},
	}

	normalizer := NewSQLNormalizer(&SQLNormalizerConfig{
		CollectComments: false,
		CollectCommands: false,
		TableNames:      false,
		KeepSQLAlias:    true,
	})

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			got, normalizedInfo, err := normalizer.Normalize(test.input)
			if err != nil {
				t.Errorf("error during normalization: %v", err)
			}
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
			if !reflect.DeepEqual(normalizedInfo.Commands, test.normalizedInfo.Commands) {
				t.Errorf("got %v, want %v", normalizedInfo.Commands, test.normalizedInfo.Commands)
			}
			if !reflect.DeepEqual(normalizedInfo.Comments, test.normalizedInfo.Comments) {
				t.Errorf("got %v, want %v", normalizedInfo.Comments, test.normalizedInfo.Comments)
			}
			if !reflect.DeepEqual(normalizedInfo.Tables, test.normalizedInfo.Tables) {
				t.Errorf("got %v, want %v", normalizedInfo.Tables, test.normalizedInfo.Tables)
			}
		})
	}
}

func TestGroupObfuscatedValues(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "(?)",
			want:  "(?)",
		},
		{
			input: "(?, ?)",
			want:  "(?)",
		},
		{
			input: "( ?, ?, ? )",
			want:  "(?)",
		},
		{
			input: "( ? )",
			want:  "(?)",
		},
		{
			input: "( ?, ? )",
			want:  "(?)",
		},
		{
			input: "( ?,?)",
			want:  "(?)",
		},
		{
			input: "[?]",
			want:  "[?]",
		},
		{
			input: "[?, ?]",
			want:  "[?]",
		},
		{
			input: "[ ?, ?, ? ]",
			want:  "[?]",
		},
		{
			input: "[ ? ]",
			want:  "[?]",
		},
		{
			input: "[ ?, ? ]",
			want:  "[?]",
		},
		{
			input: "[ ?,?]",
			want:  "[?]",
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			got := groupObfuscatedValues(test.input)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
