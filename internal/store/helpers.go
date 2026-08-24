package store

import "strings"

// isUniqueViolation 判断 SQLite 唯一约束冲突错误（UNIQUE / PRIMARY KEY）。
// modernc.org/sqlite 返回的错误文本包含 "UNIQUE constraint failed"。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "constraint failed") ||
		strings.Contains(msg, "primary key")
}
