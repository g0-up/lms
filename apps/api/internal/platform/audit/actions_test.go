package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// contractActions là danh sách action của hợp đồng API; thêm action mới phải sửa hợp đồng trước.
var contractActions = []string{
	"stage_version.published", "stage_version.cloned", "stage_version.archived", "stage_version.deleted",
	"course_version.published", "course_version.cloned", "course_version.archived", "course_version.deleted",
	"course.stage_version_applied", "class.course_version_changed",
	"class.activated", "class.ended", "class.member_invited", "class.invitation_resent", "class.member_dropped",
	"user.disabled", "user.enabled",
	"stage.created", "course.created", "class.created",
}

// TestActionsMatchContract đọc mọi hằng chuỗi trong actions.go bằng go/parser để hằng mới không thể lọt qua.
func TestActionsMatchContract(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "actions.go", nil, 0)
	require.NoError(t, err)

	var declared []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec) //nolint:forcetypeassert // const spec luôn là ValueSpec
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				require.True(t, ok && lit.Kind == token.STRING)
				s, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				declared = append(declared, s)
			}
		}
	}

	want := append([]string(nil), contractActions...)
	sort.Strings(want)
	sort.Strings(declared)
	require.Equal(t, want, declared)
	require.Len(t, knownActions, len(contractActions))
	for _, a := range contractActions {
		require.True(t, IsKnownAction(a), a)
	}
	for _, mixed := range []string{"user.invite", "stage_version.publish", "class.member_add", ""} {
		require.False(t, IsKnownAction(mixed), mixed)
	}
}
