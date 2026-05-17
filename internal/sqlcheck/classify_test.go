package sqlcheck

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]Class{
		"":                                                ClassOther,
		"   ":                                             ClassOther,
		"SELECT 1":                                        ClassRead,
		"select 1":                                        ClassRead,
		"  SELECT * FROM t":                               ClassRead,
		"WITH x AS (SELECT 1) SELECT * FROM x":            ClassRead,
		"EXPLAIN SELECT 1":                                ClassRead,
		"SHOW server_version":                             ClassRead,
		"VALUES (1)":                                      ClassRead,
		"INSERT INTO t VALUES (1)":                        ClassDML,
		"UPDATE t SET x = 1 WHERE id = 2":                 ClassDML,
		"DELETE FROM t WHERE id = 1":                      ClassDML,
		"MERGE INTO t USING s ON 1=1 WHEN MATCHED THEN ":  ClassDML,
		"TRUNCATE TABLE t":                                ClassDML,
		"CALL my_proc()":                                  ClassDML,
		"CREATE TABLE t (id int)":                         ClassDDL,
		"DROP TABLE t":                                    ClassDDL,
		"ALTER TABLE t ADD COLUMN c text":                 ClassDDL,
		"GRANT SELECT ON t TO u":                          ClassDDL,
		"REINDEX TABLE t":                                 ClassDDL,
		"VACUUM ANALYZE t":                                ClassDDL,
		"NOTIFY chan, 'payload'":                          ClassOther,
		"-- only a comment\n":                             ClassOther,
		"/* block */ SELECT 1":                            ClassRead,
		"-- intro\nSELECT 1":                              ClassRead,
		"/*\nmultiline\ncomment\n*/ DROP TABLE t":         ClassDDL,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := Classify(input); got != want {
				t.Fatalf("Classify(%q) = %v, want %v", input, got, want)
			}
		})
	}
}

func TestClass_String(t *testing.T) {
	for _, c := range []struct {
		class Class
		want  string
	}{
		{ClassRead, "read"}, {ClassDML, "dml"}, {ClassDDL, "ddl"}, {ClassOther, "other"},
	} {
		if got := c.class.String(); got != c.want {
			t.Fatalf("String(%v) = %q, want %q", c.class, got, c.want)
		}
	}
}

func TestIsDangerous(t *testing.T) {
	cases := map[string]bool{
		"DELETE FROM users":                                       true,
		"delete from users":                                       true,
		"DELETE FROM users WHERE id = 1":                          false,
		"DELETE FROM users -- WHERE id=1\n":                       true,
		"UPDATE users SET name = 'x'":                             true,
		"UPDATE users SET name = 'x' WHERE id = 1":                false,
		"update USERS set name = 'x' /* WHERE id=1 */":            true,
		"SELECT * FROM users":                                     false,
		"INSERT INTO users (name) VALUES ('x')":                   false,
		"":                                                        false,
		"DELETE FROM users WHERE name LIKE '%admin%'":             false,
		"WITH cte AS (SELECT 1) UPDATE t SET x = 1 WHERE id = 1":  false,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := IsDangerous(input); got != want {
				t.Fatalf("IsDangerous(%q) = %v, want %v", input, got, want)
			}
		})
	}
}
