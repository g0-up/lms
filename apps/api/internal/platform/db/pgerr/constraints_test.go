//go:build integration

package pgerr_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lms/api/internal/platform/testdb"
)

// registry đọc mọi hằng chuỗi trong constraints.go (tên hằng → giá trị) bằng go/parser,
// nên thêm hằng mới không cần cập nhật danh sách nào khác.
func registry(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "constraints.go", nil, 0)
	require.NoError(t, err)
	require.Empty(t, file.Imports, "constraints.go chỉ chứa hằng, không import")

	out := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		require.True(t, ok && gen.Tok == token.CONST, "constraints.go chỉ được khai báo const")
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			require.Len(t, vs.Values, len(vs.Names))
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				require.True(t, ok && lit.Kind == token.STRING, "hằng %s phải là chuỗi literal", name.Name)
				val, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				out[name.Name] = val
			}
		}
	}
	return out
}

func camel(snake string) string {
	var b strings.Builder
	for part := range strings.SplitSeq(snake, "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

func TestRegistryNamingConvention(t *testing.T) {
	reg := registry(t)
	require.NotEmpty(t, reg)
	seen := map[string]string{}
	for name, val := range reg {
		require.Regexp(t, `^(pk|uq|ck|fk)_[a-z0-9_]+$`, val, "hằng %s", name)
		require.Equal(t, camel(val), name, "tên hằng phải là CamelCase của %q", val)
		prev, dup := seen[val]
		require.False(t, dup, "%q khai báo hai lần (%s, %s)", val, prev, name)
		seen[val] = name
	}
}

// TestRegistryMatchesSchema so tập hằng với tên constraint thật (pg_constraint) và partial unique index uq_*
// (pg_class) của schema public theo cả hai chiều. Bảng schema_migrations của golang-migrate không thuộc schema
// ứng dụng nên bị loại.
func TestRegistryMatchesSchema(t *testing.T) {
	dbx := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var inDB []string
	err := dbx.SelectContext(ctx, &inDB, `
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_namespace n ON n.oid = c.connamespace
		WHERE n.nspname = 'public' AND c.conrelid <> 'public.schema_migrations'::regclass
		UNION
		SELECT i.relname
		FROM pg_class i
		JOIN pg_namespace n ON n.oid = i.relnamespace
		JOIN pg_index x ON x.indexrelid = i.oid
		WHERE n.nspname = 'public' AND i.relkind = 'i' AND x.indisunique AND i.relname LIKE 'uq\_%'`)
	require.NoError(t, err)

	var inCode []string
	for _, v := range registry(t) {
		inCode = append(inCode, v)
	}

	var missingConst, staleConst []string
	for _, name := range inDB {
		if !slices.Contains(inCode, name) {
			missingConst = append(missingConst, name)
		}
	}
	for _, name := range inCode {
		if !slices.Contains(inDB, name) {
			staleConst = append(staleConst, name)
		}
	}
	slices.Sort(missingConst)
	slices.Sort(staleConst)
	require.Empty(t, missingConst, "constraint trong schema chưa có hằng trong constraints.go")
	require.Empty(t, staleConst, "hằng trong constraints.go không còn trong schema")
}
