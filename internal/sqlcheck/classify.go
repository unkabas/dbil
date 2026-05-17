// Package sqlcheck classifies SQL statements as Read / DML / DDL / Other and
// flags the obvious "DELETE/UPDATE without WHERE" cases. Classification is
// intentionally lexical and best-effort: the audit chain captures every
// statement either way, and the Postgres role limits attached to a
// connection remain the real defence.
package sqlcheck

import (
	"regexp"
	"strings"
	"unicode"
)

// Class is the broad category of a SQL statement.
type Class int

// Classification values. Use the .String() method when emitting to audit.
const (
	ClassOther Class = iota
	ClassRead
	ClassDML
	ClassDDL
)

// String returns the lowercase tag for an audit entry.
func (c Class) String() string {
	switch c {
	case ClassRead:
		return "read"
	case ClassDML:
		return "dml"
	case ClassDDL:
		return "ddl"
	}
	return "other"
}

var (
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineComment  = regexp.MustCompile(`--[^\n]*`)
)

// stripComments removes /*...*/ block comments and -- line comments,
// replacing them with a single space so token boundaries stay intact.
func stripComments(s string) string {
	s = blockComment.ReplaceAllString(s, " ")
	s = lineComment.ReplaceAllString(s, " ")
	return s
}

// firstWord returns the first contiguous identifier of s, uppercased.
func firstWord(s string) string {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	end := 0
	for end < len(s) {
		r := rune(s[end])
		if !unicode.IsLetter(r) && r != '_' {
			break
		}
		end++
	}
	return strings.ToUpper(s[:end])
}

// Classify returns the broad category of sql based on its first keyword
// after comment stripping. Unknown / empty statements return ClassOther.
func Classify(sql string) Class {
	head := firstWord(stripComments(sql))
	switch head {
	case "SELECT", "WITH", "EXPLAIN", "SHOW", "VALUES", "TABLE", "FETCH":
		return ClassRead
	case "INSERT", "UPDATE", "DELETE", "MERGE", "TRUNCATE", "COPY", "CALL":
		return ClassDML
	case "CREATE", "ALTER", "DROP", "GRANT", "REVOKE", "COMMENT",
		"REINDEX", "VACUUM", "ANALYZE":
		return ClassDDL
	}
	return ClassOther
}

// IsDangerous best-effort flags DELETE / UPDATE statements that lack a
// WHERE clause. False negatives are possible (CTE-driven UPDATE, multi-
// statement); false positives should be rare. The check is intentionally
// loose around the SET token so "UPDATE t SET x=1" works whether or not
// there are extra spaces.
func IsDangerous(sql string) bool {
	cleaned := strings.TrimSpace(stripComments(sql))
	upper := strings.ToUpper(cleaned)
	switch {
	case strings.HasPrefix(upper, "DELETE FROM"):
		return !strings.Contains(upper, " WHERE ") && !strings.HasSuffix(upper, " WHERE")
	case strings.HasPrefix(upper, "UPDATE ") && strings.Contains(upper, " SET "):
		return !strings.Contains(upper, " WHERE ") && !strings.HasSuffix(upper, " WHERE")
	}
	return false
}
