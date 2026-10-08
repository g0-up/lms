package stages

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const testMediaID = "0192f0c1-2b3c-7d4e-8f90-a1b2c3d4e5f6"

func TestMarkdownRendererSanitizes(t *testing.T) {
	r := NewMarkdownRenderer()
	tests := []struct {
		name    string
		src     string
		absent  []string
		present []string
	}{
		{"script thô", "# Hi <script>alert(1)</script>", []string{"<script", "alert(1)</script>"}, []string{"<h1>Hi"}},
		{"onerror", `<img src="x" onerror="alert(1)">`, []string{"onerror", "<img"}, nil},
		{"onerror trong thẻ cho phép", `<p onclick="alert(1)">x</p>`, []string{"onclick"}, nil},
		{"link javascript:", "[bấm](javascript:alert(1))", []string{"javascript:"}, nil},
		{"link data:", "[bấm](data:text/html;base64,PHNjcmlwdD4=)", []string{"data:"}, nil},
		{"ảnh ngoài", "![x](https://x/a.png)", []string{"<img", "x/a.png"}, nil},
		{"ảnh data:", "![x](data:image/png;base64,iVBORw0KGgo=)", []string{"<img", "data:"}, nil},
		{"ảnh không src hợp lệ", "![x](/api/v1/media/not-a-uuid/content)", []string{"<img"}, nil},
		{"ảnh nội bộ có hậu tố", "![x](/api/v1/media/" + testMediaID + "/content?x=1)", []string{"<img"}, nil},
		{"ảnh nội bộ", "![sơ đồ](/api/v1/media/" + testMediaID + "/content)", nil,
			[]string{`<img src="/api/v1/media/` + testMediaID + `/content" alt="sơ đồ"`}},
		{"iframe", `<iframe src="https://evil"></iframe>`, []string{"<iframe"}, nil},
		{"style", `<style>body{display:none}</style>`, []string{"<style"}, nil},
		{"link https", "[doc](https://go.dev)", nil, []string{`href="https://go.dev"`, `rel="nofollow`, `target="_blank"`}},
		{"code block", "```sql\nSELECT 1;\n```", nil, []string{`<code class="language-sql">`}},
		{"bảng GFM", "| a | b |\n|---|---|\n| 1 | 2 |", nil, []string{"<table>", "<td>1</td>"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := r.Render(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range tt.absent {
				if strings.Contains(html, s) {
					t.Errorf("output contains %q: %s", s, html)
				}
			}
			for _, s := range tt.present {
				if !strings.Contains(html, s) {
					t.Errorf("output missing %q: %s", s, html)
				}
			}
		})
	}
}

func TestMediaIDsInHTML(t *testing.T) {
	r := NewMarkdownRenderer()
	other := uuid.New().String()
	src := "![a](/api/v1/media/" + testMediaID + "/content) ![b](/api/v1/media/" + other + "/content) " +
		"![c](/api/v1/media/" + testMediaID + "/content) ![d](https://x/a.png)"
	html, err := r.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	got := mediaIDsInHTML(html)
	if len(got) != 2 || got[0].String() != testMediaID || got[1].String() != other {
		t.Fatalf("ids = %v", got)
	}
	if ids := mediaIDsInHTML("<p>/api/v1/media/" + testMediaID + "/content</p>"); len(ids) != 0 {
		t.Fatalf("plain text path must not count as image: %v", ids)
	}
}

func TestForeignImages(t *testing.T) {
	mediaSrc := "/api/v1/media/" + testMediaID + "/content"
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"ảnh ngoài", "![a](https://x/y.png)", []string{"https://x/y.png"}},
		{"ảnh media", "![a](" + mediaSrc + ")", nil},
		{"ảnh media có hậu tố", "![a](" + mediaSrc + "?x=1)", []string{mediaSrc + "?x=1"}},
		{"đường dẫn tương đối khác", "![a](/khac.png)", []string{"/khac.png"}},
		{"ảnh reference", "![a][r]\n\n[r]: https://x/y.png", []string{"https://x/y.png"}},
		{"reference không có định nghĩa", "![a][r]", nil},
		{"inline code", "`![a](https://x)`", nil},
		{"fenced code", "```md\n![a](https://x/y.png)\n```", nil},
		{"HTML thô", `<img src="https://x">`, nil},
		{"ảnh trong bảng", "| a |\n|---|\n| ![a](https://x/t.png) |", []string{"https://x/t.png"}},
		{"ảnh trong trích dẫn và danh sách", "> ![a](https://x/q.png)\n\n- ![b](" + mediaSrc + ")\n- ![c](/l.png)",
			[]string{"https://x/q.png", "/l.png"}},
		{"rỗng", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // parser dùng chung giữa các request: chạy song song để -race bắt được lỗi dùng chung
			if got := foreignImages(tt.src); !slices.Equal(got, tt.want) {
				t.Fatalf("foreignImages = %q, want %q", got, tt.want)
			}
		})
	}
}
