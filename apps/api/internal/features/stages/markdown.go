package stages

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// MarkdownRenderer đổi markdown_source thành HTML đã lọc, an toàn để trình duyệt hiển thị thẳng.
type MarkdownRenderer interface {
	Render(source string) (html string, err error)
}

// mediaContentPath là dạng duy nhất của ảnh được phép trong markdown: ảnh đã upload qua media, phát qua API.
const mediaContentPath = `/api/v1/media/([0-9a-f-]{36})/content`

var (
	mediaSrcPattern   = regexp.MustCompile(`^` + mediaContentPath + `$`)
	codeClassPattern  = regexp.MustCompile(`^language-[a-z0-9]+$`)
	imgTagPattern     = regexp.MustCompile(`<img\b[^>]*>`)
	imgValidSrc       = regexp.MustCompile(`\ssrc="` + mediaContentPath + `"`)
	htmlMediaIDSource = regexp.MustCompile(`<img\b[^>]*\ssrc="` + mediaContentPath + `"`)
)

// markdownExtensions dùng chung cho renderer và parser kiểm ảnh, để luật lúc lưu và HTML lúc phát hành đọc markdown
// theo cùng một cú pháp.
var markdownExtensions = []goldmark.Extender{extension.GFM}

// markdownParser chỉ dựng AST để kiểm nội dung. Parser goldmark khởi tạo một lần (sync.Once) và tạo context mới mỗi
// lần Parse nên dùng chung giữa các request được.
var markdownParser = goldmark.New(goldmark.WithExtensions(markdownExtensions...)).Parser()

// GoldmarkRenderer render GFM bằng goldmark (không bật unsafe nên HTML thô bị bỏ) rồi lọc lại bằng bluemonday.
// Policy dựng từ NewPolicy() với danh sách cho phép tường minh; không dùng UGCPolicy/AllowImages vì AllowImages
// cho mọi img src và Matching() thêm sau không thu hẹp được.
type GoldmarkRenderer struct {
	md     goldmark.Markdown
	policy *bluemonday.Policy
}

var _ MarkdownRenderer = (*GoldmarkRenderer)(nil)

// NewMarkdownRenderer dùng chung cho service (khi phát hành) và seed để HTML hai nơi giống hệt nhau.
func NewMarkdownRenderer() *GoldmarkRenderer {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "h1", "h2", "h3", "h4", "ul", "ol", "li", "strong", "em", "del", "code", "pre",
		"blockquote", "hr", "table", "thead", "tbody", "tr", "th", "td")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	// Cần cho src ảnh nội bộ dạng đường dẫn tương đối; scheme lạ (javascript:, data:) vẫn bị chặn.
	p.AllowRelativeURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowAttrs("class").Matching(codeClassPattern).OnElements("code")
	p.AllowAttrs("src").Matching(mediaSrcPattern).OnElements("img")
	p.AllowAttrs("alt").OnElements("img")
	return &GoldmarkRenderer{
		md:     goldmark.New(goldmark.WithExtensions(markdownExtensions...)),
		policy: p,
	}
}

// Render trả HTML đã lọc. Thẻ img mất src hợp lệ bị gỡ cả thẻ (bluemonday vẫn giữ <img alt> khi src bị lọc).
func (r *GoldmarkRenderer) Render(source string) (string, error) {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(source), &buf); err != nil {
		return "", fmt.Errorf("stages: render markdown: %w", err)
	}
	html := r.policy.Sanitize(buf.String())
	return imgTagPattern.ReplaceAllStringFunc(html, func(tag string) string {
		if imgValidSrc.MatchString(tag) {
			return tag
		}
		return ""
	}), nil
}

// foreignImages trả destination của mọi ảnh markdown không phải ảnh media nội bộ, theo thứ tự xuất hiện. Đi trên AST
// nên ảnh trong code block/inline code và thẻ <img> HTML thô (renderer bỏ HTML thô) không bị tính; destination là
// bytes thô chưa unescape nên biến thể escape của đường dẫn media cũng bị coi là ảnh ngoài.
func foreignImages(source string) []string {
	var out []string
	doc := markdownParser.Parse(text.NewReader([]byte(source)))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering {
			if dest := string(img.Destination); !mediaSrcPattern.MatchString(dest) {
				out = append(out, dest)
			}
		}
		return ast.WalkContinue, nil
	})
	return out
}

// mediaIDsInHTML trích id media của mọi ảnh nội bộ trong HTML đã render, bỏ trùng, giữ thứ tự xuất hiện.
func mediaIDsInHTML(html string) []uuid.UUID {
	matches := htmlMediaIDSource.FindAllStringSubmatch(html, -1)
	out := make([]uuid.UUID, 0, len(matches))
	seen := make(map[uuid.UUID]bool, len(matches))
	for _, m := range matches {
		id, err := uuid.Parse(m[1])
		if err != nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
