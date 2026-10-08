// Package migrations nhúng file SQL của golang-migrate (NNNN_<slug>.up.sql / .down.sql) vào binary.
package migrations

import "embed"

// FS chứa mọi file migration; nguồn iofs của golang-migrate đọc từ thư mục gốc ".".
//
//go:embed *.sql
var FS embed.FS
