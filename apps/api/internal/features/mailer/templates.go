package mailer

import (
	"bytes"
	"embed"
	"fmt"
	htmltpl "html/template"
	texttpl "text/template"
	"time"
)

// AppName là tên ứng dụng hiển thị trong email.
const AppName = "GoUp LMS"

//go:embed templates/*.tmpl
var templateFS embed.FS

// subjects là tiêu đề email theo template; {{.ClassCode}} được render như text.
var subjects = map[TemplateName]string{
	TemplateInvite:        "[" + AppName + "] Lời mời vào lớp {{.ClassCode}}",
	TemplateAdded:         "[" + AppName + "] Bạn đã được thêm vào lớp {{.ClassCode}}",
	TemplateResend:        "[" + AppName + "] Mật khẩu tạm mới cho lớp {{.ClassCode}}",
	TemplatePasswordReset: "[" + AppName + "] Đặt lại mật khẩu",
}

// Renderer render email từ template nhúng; HTML qua html/template (tự escape), text và subject qua text/template.
type Renderer struct {
	html    map[TemplateName]*htmltpl.Template
	text    map[TemplateName]*texttpl.Template
	subject map[TemplateName]*texttpl.Template
}

// NewRenderer parse toàn bộ template; loc là múi giờ hiển thị hạn mật khẩu tạm (nil = UTC).
func NewRenderer(loc *time.Location) (*Renderer, error) {
	if loc == nil {
		loc = time.UTC
	}
	funcs := map[string]any{"vntime": func(v any) string { return formatTime(v, loc) }}
	r := &Renderer{
		html:    map[TemplateName]*htmltpl.Template{},
		text:    map[TemplateName]*texttpl.Template{},
		subject: map[TemplateName]*texttpl.Template{},
	}
	for name, subj := range subjects {
		h, err := htmltpl.New(string(name)+".html.tmpl").Funcs(funcs).Option("missingkey=error").
			ParseFS(templateFS, "templates/"+string(name)+".html.tmpl")
		if err != nil {
			return nil, fmt.Errorf("mailer: parse %s.html: %w", name, err)
		}
		t, err := texttpl.New(string(name)+".txt.tmpl").Funcs(funcs).Option("missingkey=error").
			ParseFS(templateFS, "templates/"+string(name)+".txt.tmpl")
		if err != nil {
			return nil, fmt.Errorf("mailer: parse %s.txt: %w", name, err)
		}
		s, err := texttpl.New(string(name) + ".subject").Option("missingkey=error").Parse(subj)
		if err != nil {
			return nil, fmt.Errorf("mailer: parse %s subject: %w", name, err)
		}
		r.html[name], r.text[name], r.subject[name] = h, t, s
	}
	return r, nil
}

// Render dựng email cho to từ template và data (payload đã ghép bí mật và AppName).
func (r *Renderer) Render(name TemplateName, to string, data map[string]any) (RenderedMail, error) {
	h, ok := r.html[name]
	if !ok {
		return RenderedMail{}, fmt.Errorf("%w: %q", ErrUnknownTemplate, name)
	}
	merged := make(map[string]any, len(data)+1)
	for k, v := range data {
		merged[k] = v
	}
	merged["AppName"] = AppName

	var subj, text, html bytes.Buffer
	if err := r.subject[name].Execute(&subj, merged); err != nil {
		return RenderedMail{}, fmt.Errorf("mailer: render %s subject: %w", name, err)
	}
	if err := r.text[name].Execute(&text, merged); err != nil {
		return RenderedMail{}, fmt.Errorf("mailer: render %s.txt: %w", name, err)
	}
	if err := h.Execute(&html, merged); err != nil {
		return RenderedMail{}, fmt.Errorf("mailer: render %s.html: %w", name, err)
	}
	return RenderedMail{To: to, Subject: subj.String(), Text: text.String(), HTML: html.String()}, nil
}

// formatTime hiển thị thời điểm theo loc dạng "15:04 ngày 02/01/2006". Payload jsonb lưu time dạng RFC 3339 nên
// nhận cả string lẫn time.Time; giá trị không parse được thì in nguyên văn.
func formatTime(v any, loc *time.Location) string {
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case *time.Time:
		if x == nil {
			return ""
		}
		t = *x
	case string:
		p, err := time.Parse(time.RFC3339Nano, x)
		if err != nil {
			return x
		}
		t = p
	default:
		return fmt.Sprint(v)
	}
	return t.In(loc).Format("15:04 ngày 02/01/2006")
}
