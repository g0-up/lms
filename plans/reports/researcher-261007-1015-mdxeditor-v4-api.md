# Research: `@mdxeditor/editor@4.3.2` cho lesson editor (GFM, upload media)

Ngày: 2026-10-07 (Asia/Saigon). Cách làm: `npm pack @mdxeditor/editor@4.3.2`, đọc `package.json`, `README.md`, `dist/index.d.ts` (3252 dòng) và mã JS trong `dist/`, rồi kiểm chứng hành vi bằng harness vitest 5 + jsdom 30 + React 19.3 trong scratchpad (không cài vào project). Tarball không có CHANGELOG, nên release notes lấy từ GitHub Releases API. Các đường dẫn `dist/...` bên dưới là đường dẫn trong tarball.

## Kết luận (TL;DR)

**Nên dùng**, với điều kiện cấu hình hạn chế và bọc thêm hai realm plugin nhỏ (`gfmGuard`, `hardBreak`). Thư viện đáp ứng đủ tập GFM dự án cần, đang được bảo trì tích cực (MIT, release gần nhất 4.3.2 ngày 2026-10-02) và chạy được với React 19.

Bốn điểm phải xử lý chủ động:
1. Plugin core luôn bật underline, sup, sub và highlight (`==x==`). Ctrl+U và paste HTML vẫn tạo ra các định dạng này, nên cần chặn bằng command và node transform.
2. `validateUrl` chỉ áp dụng cho `TOGGLE_LINK_COMMAND`. Link dialog và markdown shortcut đi vòng qua nó (maintainer xác nhận ở issue #880).
3. Hard break (Shift+Enter) bị export thành soft `\n`, tức là mất `<br>`.
4. Lỗi upload bị rethrow thành unhandled rejection và không có UI báo lỗi.

Server (goldmark + bluemonday) vẫn là lớp bảo vệ có thẩm quyền. Editor chỉ là lớp UX.

## 1. Peer deps, CSS, ESM/SSR, bundle

- `peerDependencies`: `react` và `react-dom` `">= 18 || >= 19"`. `engines.node >=16`. Package là `"type":"module"`, export `"."` → `./dist/index.js` và `"./style.css"`. **Chỉ có ESM, không có bản CJS** (`package.json`).
- Bắt buộc `import '@mdxeditor/editor/style.css'` (`README.md`). File CSS nặng 63.7 KB raw, sau khi Vite minify còn 49.7 KB (8.9 KB gzip).
- Dependency đáng chú ý:
  - Lexical `^0.48.0`, trong khi Lexical latest là 0.52, nên đừng thêm `lexical` vào project với version khác.
  - CodeMirror 6.
  - Các package `@radix-ui/react-*` riêng lẻ (dialog, popover, select, toolbar, tooltip…). Đây là bản cài riêng, không phải umbrella package `radix-ui` mà project dùng.
  - `react-hook-form ^7.56` là runtime dep. Bản này dedupe được với `^7.89` của project.
- SSR: mã dùng `document` khi mount. App là Vite SPA nên không có vấn đề. Nếu sau này có SSR thì phải render client-only.
- Bundle đo bằng Vite 8 lib mode, minify, external react:

| Entry | raw | gzip |
|---|---|---|
| `MDXEditor` + `headingsPlugin` (lõi) | 611 KB | **162 KB** |
| Cấu hình đề xuất (table, link, image, codeBlock/codeMirror, diffSource, toolbar) | 1.88 MB | **528 KB** (JS tĩnh) |

  CodeMirror language-data tách thành khoảng 110 chunk lazy. **Khuyến nghị**: `React.lazy` cho route hoặc component editor để màn learner không phải tải editor.

## 2. Props, ref methods, controlled hay uncontrolled

Signature: `MDXEditor: ForwardRefExoticComponent<MDXEditorProps & RefAttributes<MDXEditorMethods>>` (`dist/index.d.ts`).

- Props chính: `markdown: string` (bắt buộc, **chỉ đọc lúc mount**), `onChange?(markdown, initialMarkdownNormalize: boolean)`, `onError?({error, source})`, `plugins?: RealmPlugin[]`, `readOnly?`, `placeholder?: ReactNode`, `className?`, `contentEditableClassName?`, `autoFocus?`, `onBlur?`, `overlayContainer?: HTMLElement | null` (mặc định `document.body`), `suppressHtmlProcessing?`, `trim?` (mặc định true), `translation?: (key, defaultValue, interpolations?) => string`, `iconComponentFor?`, `lexicalTheme?`, `spellCheck?`.
- `toMarkdownOptions?` **thay thế** mặc định `{listItemIndent:'one'}`, không merge (`dist/MDXEditor.js`).
- Ref (`MDXEditorMethods`): `getMarkdown(): string`, `setMarkdown(v)`, `insertMarkdown(v)`, `focus(cb?, {defaultSelection?: 'rootStart'|'rootEnd', preventScroll?})`, `getContentEditableHTML()`, `getSelectionMarkdown()` (thêm ở 4.1.0).
- **Component này là uncontrolled** (đã kiểm chứng): rerender với prop `markdown` mới không có tác dụng. `setMarkdown` **không** gọi `onChange` (mute qua `muteChange$`) và bỏ qua nếu giá trị sau trim không đổi (`dist/plugins/core/index.js`). Để reset nội dung, đổi `key` (ví dụ `key={lessonId + ':' + version}`) hoặc gọi `ref.setMarkdown`.
- `getMarkdown()` trả về chuỗi đã trim, không có newline cuối. Ở chế độ source hoặc diff, nó trả về giá trị của source editor.

## 3. Plugin, toolbar, và cách chặn định dạng ngoài GFM

| Plugin | Options (exact) | Ghi chú |
|---|---|---|
| `headingsPlugin` | `{allowedHeadingLevels?: readonly HEADING_LEVEL[]}` | Chỉ lọc dropdown và phím tắt Ctrl/Cmd+Alt+N. **Import vẫn giữ h5/h6.** |
| `listsPlugin` | không có | Luôn bật task list, có shortcut `[ ] `, độ sâu tối đa 7. |
| `quotePlugin`, `thematicBreakPlugin`, `markdownShortcutPlugin` | không có | `markdownShortcutPlugin` chọn transformer từ `activePlugins$` lúc init, **nên phải đặt cuối mảng**. |
| `linkPlugin` | `{validateUrl?, disableAutoLink?}` | |
| `linkDialogPlugin` | `{LinkDialog?, linkAutocompleteSuggestions?, onClickLinkCallback?, onReadOnlyClickLinkCallback?, showLinkTitleField?}` | |
| `imagePlugin` | `{imageUploadHandler?, imageAutocompleteSuggestions?, disableImageResize?, disableImageSettingsButton?, allowSetImageDimensions?, imagePreviewHandler?, ImageDialog?, EditImageToolbar?, imagePlaceholder?}` | |
| `tablePlugin` | options của `mdast-util-gfm-table` | |
| `codeBlockPlugin` | `{codeBlockEditorDescriptors?, defaultCodeBlockLanguage?}` | |
| `codeMirrorPlugin` | `{codeBlockLanguages, codeMirrorExtensions?, autoLoadLanguageSupport?}` | |
| `diffSourcePlugin` | `{viewMode?, diffMarkdown?, codeMirrorExtensions?, readOnlyDiff?}` | |
| `toolbarPlugin` | `{toolbarContents, toolbarClassName?, toolbarPosition?}` | |

Toolbar components có option lọc: `BoldItalicUnderlineToggles options={['Bold','Italic']}`, `StrikeThroughSupSubToggles options={['Strikethrough']}`, `ListsToggle options={['bullet','number']}`. Các component còn lại: `BlockTypeSelect`, `CreateLink`, `InsertImage`, `InsertTable`, `InsertThematicBreak`, `InsertCodeBlock`, `CodeToggle`, `UndoRedo`, `DiffSourceToggleWrapper`, `Separator`.

**Những gì không biểu diễn được trong GFM hoặc nằm ngoài allowlist, kèm bằng chứng:**
- Underline, sup và sub export thành `<u>`, `<sup>`, `<sub>`; highlight thành `==x==`; text có style thành `<span style>` (`dist/plugins/core/LexicalTextVisitor.js`). Ctrl+U (đã thử trong jsdom) tạo ra `<u>`. Paste HTML chứa `<u>`, `<sup>`, `<mark>` hoặc style span đều được import.
- Task list sẽ bị bluemonday bỏ `<input>`, chỉ còn chữ. h5/h6 có thể vào qua paste hoặc import.
- Ẩn nút trên toolbar **không đủ**. Cần plugin `gfmGuard` (xem §11), dùng `FORMAT_TEXT_COMMAND` ở `COMMAND_PRIORITY_CRITICAL` kèm transform `TextNode`, `ListNode` và `HeadingNode`. Plugin này đã được kiểm chứng: Ctrl+U bị chặn và HTML paste ra plain text sạch.

## 4. Markdown hoặc HTML không hỗ trợ, và `suppressHtmlProcessing`

- Import dùng mdast visitors. Node không có visitor sẽ ném `UnrecognizedMarkdownConstructError` (`dist/importMarkdownToLexical.js`). Lỗi được bắt, set `markdownProcessingError$` và gọi `onError`.
- Khi đang lỗi, editor **trắng** (trừ khi dùng `diffSourcePlugin`, plugin này hiện thông báo lỗi kèm gợi ý sửa ở source mode). Gõ phím không gọi `onChange`, `getMarkdown()` trả lại nguyên source, và `setMarkdown(hợp lệ)` khôi phục được editor.
- **Mặc định (HTML/MDX bật)**, core đăng ký mdxJsx/mdxMd, và mdxMd tắt autolink, indented code và html. Hệ quả: `3<4`, `<https://x.com>`, footnote và reference link đều gây **onError**; indented code âm thầm thành paragraph; HTML comment âm thầm bị xoá; `<div>` và `<kbd>` được giữ dưới dạng `GenericHTMLNode`.
- **`suppressHtmlProcessing: true`** bỏ mdxJsx, mdxMd, comment và `MdastHTMLVisitor`. Hệ quả: `<` được escape thành `\<` và không còn lỗi; autolink thành `[url](url)`; indented code thành fence; raw `<div>` và comment gây onError; `<u>` vẫn import được do formatting visitor đọc html.
- **Khuyến nghị bật `suppressHtmlProcessing`**, vì server cũng không cho raw HTML và lesson thường có `<`, `>` trong văn bản. Đổi lại, HTML thô trong dữ liệu cũ sẽ gây lỗi. Đó là hành vi mong muốn, vì người dùng có thể sửa ở diff/source mode.
- Footnote và reference link gây lỗi ở cả hai chế độ, nên nội dung nhập từ ngoài cần được chuẩn hoá trước.

## 5. Fidelity, round-trip và `onChange` lúc load

- Input ở dạng canonical round-trip y hệt và **không** gọi `onChange` lúc load.
- Nếu serializer chuẩn hoá khác input, `onChange` gọi **một lần lúc mount** với `initialMarkdownNormalize=true`. Các phép chuẩn hoá đã quan sát:

| Input | Output |
|---|---|
| `_em_` | `*em*` |
| `__s__` | `**s**` |
| `-` (bullet) | `*` |
| `---` | `***` |
| `~~~js` | ```` ```js ```` |
| `\|:-\|-:\|` | `\| :- \| -: \|` (table được pad) |
| `hello\n` | `hello` (trim) |

- Escape: ký tự gõ vào `*c*` thành `\*c\*`, `[d]` thành `\[d]`, `<f>` thành `\<f>`. `snake_case`, `2*3*4` và dấu `#` giữa dòng giữ nguyên.
- Hard break (Shift+Enter, hoặc `\`+newline khi import) export thành `\n`, nên goldmark render ra soft break. Plugin `hardBreak` sửa thành `\\\n` và đã kiểm chứng round-trip ổn định.
- Code fence không có language export ra ```` ``` ```` trơn. Image title được giữ.

## 6. Upload ảnh

- Contract: `ImageUploadHandler = ((image: File) => Promise<string>) | null`. Chuỗi resolve ra là `src`.
- Lỗi: cả bốn đường (`insertImage$`, `saveImage$`, paste, drop) đều `.catch(e => { throw e })`, dẫn tới **unhandled rejection**; dialog vẫn mở, không có UI báo lỗi (`dist/plugins/image/index.js`). Handler phải tự bắt lỗi và toast bằng `sonner`; trong unit test, unhandled rejection làm vitest báo lỗi.
- Paste (`COMMAND_PRIORITY_CRITICAL`): payload chỉ toàn ảnh thì upload từng file rồi insert với `altText: ""`. Payload hỗn hợp rơi xuống paste HTML của Lexical, và `ImageNode.importDOM` giữ nguyên **src ngoài** (đã thấy `![e](https://evil.example/x.png)`).
- Drop: item `kind === 'string'` được insert **thẳng chuỗi src**, không qua upload.
- Chặn URL ngoài: không có option nào làm việc này. Cần transform `ImageNode` để xoá node khi src không khớp `^/api/v1/media/[0-9a-f-]+/content$` (đã kiểm chứng với paste).
- **Transform không chạy với nội dung nạp lúc mount** vì node không dirty. Đã kiểm chứng: ảnh ngoài và checklist nạp lúc mount vẫn còn. Ngược lại, `setMarkdown` có kích hoạt transform, và `getMarkdown()` phản ánh kết quả đã lọc dù `onChange` bị mute. Do đó cần kiểm tra lúc submit (regex/zod) và dựa vào bluemonday làm lớp có thẩm quyền.
- Chế độ chỉ upload: dialog mặc định có ô nhập URL và autocomplete. **Khuyến nghị** truyền `ImageDialog` custom: đọc `imageDialogState$` (`{type:'inactive'|'new'|'editing'}`), tự gọi presigned-PUT, rồi publish `saveImage$({src, altText})` và `closeImageDialog$()`. Cách này kiểm soát lỗi trọn vẹn và bắt buộc nhập alt. Đặt `disableImageResize: true` vì width/height sẽ bị bluemonday bỏ.

## 7. Theming, dark mode, z-index, Radix Dialog

- Các biến CSS khai báo trên class `_editorRoot_*`, gắn vào cả `.mdxeditor` lẫn `.mdxeditor-popup-container`: `--accentBase…--accentTextContrast` (12 bước), `--basePageBg`, `--baseBase…--baseTextContrast`, `--spacing-*`, `--radius-*`, `--font-body`, `--font-mono`, `--text-base|sm|xs`, `--error-color`.
- Dark mode: thêm class `dark-theme` (https://mdxeditor.dev/editor/docs/theming). `className` được áp cho cả popup container, nên truyền `className={isDark ? 'dark-theme' : ''}`. Issue #921 (open) cho biết `BlockTypeSelect` chưa hỗ trợ dark đầy đủ.
- Nội dung soạn thảo không có style prose. Dùng `contentEditableClassName="prose …"` hoặc CSS Tailwind 4 riêng. Các class public: `.mdxeditor-root-contenteditable`, `.mdxeditor-toolbar`, `.mdxeditor-full-height` (từ 4.2.0).
- z-index: popup container có `position:relative; z-index:2`, dialog overlay 51, dialog content 52. Maintainer khuyên override `.mdxeditor-popup-container { z-index: … }` (issue #305).
- **Editor nằm trong Radix Dialog của app**: popup mặc định gắn vào `document.body`, nằm ngoài vùng focus trap và `pointer-events` của modal; editor lại dùng instance `@radix-ui/react-dialog` riêng, không chia sẻ DismissableLayer context với `radix-ui` của app. Cách xử lý: truyền `overlayContainer={contentEl}` (DOM node của `Dialog.Content`, lấy qua callback ref và state). Trường hợp `<dialog>` showModal đã được sửa từ 3.30.1 (issue #752). Tương tác với Radix Dialog **chưa kiểm chứng** trên browser thật, cần một Playwright test; đơn giản nhất là soạn lesson trên trang riêng, không đặt trong modal.

## 8. Test trong jsdom/vitest

- Render và round-trip chạy được trong jsdom 30, với `css: false`. Link dialog hoặc selection cần polyfill:

```js
// vitest setup
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} }
Range.prototype.getBoundingClientRect ??= () => ({ x: 0, y: 0, top: 0, left: 0, bottom: 0, right: 0, width: 0, height: 0, toJSON() {} })
Range.prototype.getClientRects ??= () => ({ length: 0, item: () => null, [Symbol.iterator]: function* () {} })
```

- Cần `await` khoảng 50 ms sau render vì realm và Lexical cập nhật bất đồng bộ. Để thao tác Lexical trực tiếp trong test, lấy `LexicalEditor` qua `realmPlugin` + `createRootEditorSubscription$`.
- **Khuyến nghị**: unit test các plugin gfmGuard/hardBreak và hàm `uploadImage` bằng harness trên; trong test form, mock `LessonEditor` bằng `<textarea>`; gõ phím, paste, upload và dialog thì test bằng Playwright (upstream cũng test bằng Playwright).

## 9. Accessibility

- ContentEditable có `role="textbox"` và `aria-label` = `t('contentArea.editableMarkdown','editable markdown')` (`dist/MDXEditor.js`). Không có prop `aria-labelledby`. Đổi nhãn qua `translation`, kiêm luôn i18n tiếng Việt.
- Từ 4.3.0, mọi dialog đều có accessible name và description (issue #792 đã đóng).
- Toolbar dùng Radix Toolbar (roving focus) và tooltip.
- Phím tắt heading Ctrl/Cmd+Alt+1..4 được lọc theo `allowedHeadingLevels` (4.1.1).
- Chưa audit bằng axe trên browser.

## 10. License và mức độ bảo trì

- License MIT, tác giả Petyo Ivanov. Repo `mdx-editor/editor`: khoảng 3.7k sao, 90 issue mở, push gần nhất 2026-10-02, không bị archive.
- Release dày: 3.x ra liên tục đến 3.55.0 (2026-04-19). 4.0.0 (2026-05-09) chỉ có một breaking change là bỏ Sandpack. 4.1.0 chuyển sang Lexical 0.48.
- Rủi ro: bus factor (phần lớn commit từ một maintainer) và Lexical lệch version. Nên pin `~4.3.2` và nâng có kiểm soát.

## 11. Cấu hình đề xuất

```tsx
import '@mdxeditor/editor/style.css'
import {
  MDXEditor, type MDXEditorMethods, realmPlugin, createRootEditorSubscription$, addExportVisitor$,
  headingsPlugin, listsPlugin, quotePlugin, thematicBreakPlugin, linkPlugin, linkDialogPlugin, imagePlugin,
  tablePlugin, codeBlockPlugin, codeMirrorPlugin, diffSourcePlugin, toolbarPlugin, markdownShortcutPlugin,
  ImageNode, UndoRedo, BlockTypeSelect, BoldItalicUnderlineToggles, StrikeThroughSupSubToggles, ListsToggle,
  CreateLink, InsertImage, InsertTable, InsertThematicBreak, InsertCodeBlock, DiffSourceToggleWrapper, Separator,
} from '@mdxeditor/editor'
import { FORMAT_TEXT_COMMAND, COMMAND_PRIORITY_CRITICAL, TextNode, $isLineBreakNode } from 'lexical'
import { ListNode } from '@lexical/list'
import { LinkNode } from '@lexical/link'
import { HeadingNode, $createHeadingNode } from '@lexical/rich-text'

const MEDIA_SRC = /^\/api\/v1\/media\/[0-9a-f-]{36}\/content$/
const SAFE_URL = /^(https?:|mailto:|\/(?!\/)|#)/i
const STRIP = 8 | 32 | 64 | 128 // IS_UNDERLINE | IS_SUBSCRIPT | IS_SUPERSCRIPT | IS_HIGHLIGHT
const BLOCKED = new Set(['underline', 'subscript', 'superscript', 'highlight'])

const gfmGuard = realmPlugin({ init: (r) => r.pub(createRootEditorSubscription$, (e) => {
  const offs = [
    e.registerCommand(FORMAT_TEXT_COMMAND, (f) => BLOCKED.has(f), COMMAND_PRIORITY_CRITICAL),
    e.registerNodeTransform(TextNode, (n) => { if (n.getFormat() & STRIP) n.setFormat(n.getFormat() & ~STRIP); if (n.getStyle()) n.setStyle('') }),
    e.registerNodeTransform(ImageNode, (n) => { if (!MEDIA_SRC.test(n.getSrc())) n.remove() }),
    e.registerNodeTransform(ListNode, (n) => { if (n.getListType() === 'check') n.setListType('bullet') }),
    e.registerNodeTransform(LinkNode, (n) => { if (!SAFE_URL.test(n.getURL())) { for (const c of n.getChildren()) n.insertBefore(c); n.remove() } }),
    e.registerNodeTransform(HeadingNode, (n) => { if (n.getTag() === 'h5' || n.getTag() === 'h6') { const h = $createHeadingNode('h4'); h.append(...n.getChildren()); n.replace(h) } }),
  ]
  return () => offs.forEach((off) => off())
}) })
const hardBreak = realmPlugin({ init: (r) => r.pub(addExportVisitor$, {
  testLexicalNode: $isLineBreakNode, priority: 100,
  visitLexicalNode: ({ mdastParent, actions }) => actions.appendToParent(mdastParent, { type: 'break' }),
}) })

export const lessonPlugins = (upload: (f: File) => Promise<string>) => [
  headingsPlugin({ allowedHeadingLevels: [1, 2, 3, 4] }), listsPlugin(), quotePlugin(), thematicBreakPlugin(),
  linkPlugin({ validateUrl: (u) => SAFE_URL.test(u) }), linkDialogPlugin(),
  imagePlugin({ imageUploadHandler: upload, disableImageResize: true, ImageDialog: UploadOnlyImageDialog }),
  tablePlugin(), codeBlockPlugin({ defaultCodeBlockLanguage: '' }),
  codeMirrorPlugin({ codeBlockLanguages: { '': 'Plain', ts: 'TypeScript', go: 'Go', sql: 'SQL', bash: 'Bash' } }),
  diffSourcePlugin({ viewMode: 'rich-text' }),
  toolbarPlugin({ toolbarContents: () => (
    <DiffSourceToggleWrapper options={['rich-text', 'source']}>
      <UndoRedo /><Separator /><BlockTypeSelect />
      <BoldItalicUnderlineToggles options={['Bold', 'Italic']} /><StrikeThroughSupSubToggles options={['Strikethrough']} />
      <ListsToggle options={['bullet', 'number']} /><Separator />
      <CreateLink /><InsertImage /><InsertTable /><InsertThematicBreak /><InsertCodeBlock />
    </DiffSourceToggleWrapper>) }),
  gfmGuard(), hardBreak(),
  markdownShortcutPlugin(), // must stay last
]
```

Ghi chú về cấu hình trên:
- Hai realm plugin đã được kiểm chứng trong jsdom. Riêng transform `ListNode` (check → bullet) đã thử qua `setMarkdown`.
- `upload` phải tự `try/catch`, gọi `toast.error` rồi rethrow. Trong dialog custom, dùng `const [state] = useCellValues(imageDialogState$)` và `usePublisher(saveImage$)`.
- Tích hợp với React Hook Form:

```tsx
<Controller name="body" control={control} render={({ field }) => (
  <MDXEditor key={lessonKey} ref={editorRef} markdown={field.value} suppressHtmlProcessing
    plugins={plugins} overlayContainer={dialogContentEl /* only if inside a modal */}
    className={isDark ? 'dark-theme' : undefined} contentEditableClassName="prose max-w-none"
    translation={(k, d) => (k === 'contentArea.editableMarkdown' ? 'Nội dung bài học' : d)}
    onChange={(md, isInitialNormalize) => { if (!isInitialNormalize) field.onChange(md) }}
    onError={({ error }) => setError('body', { message: error })} onBlur={field.onBlur} />
)} />
```

- Bỏ qua lần normalize ban đầu để form không bị dirty giả.
- Lúc submit, chạy regex/zod kiểm tra src ảnh và URL link. Server vẫn sanitize lần cuối.

## Ma trận đánh đổi (quyết định chính)

| Lựa chọn | Độ an toàn so với allowlist | Độ phức tạp | Rủi ro |
|---|---|---|---|
| **A. MDXEditor + `suppressHtmlProcessing` + gfmGuard/hardBreak (đề xuất)** | Cao (chặn ở UI, server là authority) | Trung bình (khoảng 40 dòng plugin) | Dựa vào API Lexical nội bộ, cần pin version |
| B. MDXEditor mặc định, chỉ lọc toolbar | Thấp: Ctrl+U, paste `<u>`/ảnh ngoài/`javascript:` lọt qua | Thấp | Lệch so với render server, gây lỗi `3<4` |
| C. Textarea + preview (không dùng WYSIWYG) | Cao nhất | Thấp nhất, khoảng 0 KB | UX kém cho giảng viên |

Xếp hạng: **A > C > B**. Chọn C nếu không chấp nhận được khoảng 528 KB gzip, dù đã lazy-load.

## Giới hạn của nghiên cứu

- Tương tác bàn phím, paste thật, drag-drop và lồng trong Radix Dialog mới thử trong jsdom hoặc suy ra từ mã, chưa chạy trên Chromium.
- Không đọc trực tiếp mã nguồn Lexical, vì hook chặn đường dẫn `node_modules`. Hành vi Lexical được suy ra từ thực nghiệm.
- Kích thước bundle đo bằng Vite lib mode với external react. Số liệu thực tế còn phụ thuộc vào chunking của app.
- Chưa chạy axe và chưa thử trên mobile. Issue #951 và #327 cho thấy dialog trên Android còn lỗi.

## Câu hỏi chưa giải quyết

1. Nội dung lesson cũ hoặc nhập từ ngoài có chứa footnote, reference link hoặc raw HTML không? Nếu có thì cần migrate, vì chúng sẽ gây onError.
2. Có cho phép ảnh không có alt không? Paste và drop chèn `altText: ""`. Nếu alt là bắt buộc thì phải chặn paste ảnh hoặc ép mở dialog.
3. Editor đặt trên trang riêng hay trong modal? Câu trả lời quyết định có cần `overlayContainer` và Playwright test cho trường hợp lồng dialog hay không.

Status: DONE_WITH_CONCERNS
Summary: Đã trả lời đủ 10 câu hỏi, dựa trên tarball 4.3.2 và thử nghiệm trong jsdom. Đề xuất dùng MDXEditor với `suppressHtmlProcessing`, plugin gfmGuard/hardBreak, ImageDialog chỉ-upload và lazy-load.
Concerns/Blockers: Lỗi upload là unhandled rejection. `validateUrl` bị dialog đi vòng qua. Transform không chạy cho nội dung lúc mount. Bundle khoảng 528 KB gzip. Phần Radix Dialog lồng nhau và paste/drop thật chưa được kiểm chứng trên browser.
