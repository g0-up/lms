---
phase: 3
title: "Phase 3: Component MarkdownEditor: plugin, chặn định dạng, dialog ảnh, tiếng Việt, theme"
status: completed
priority: P2
effort: "1.5 ngày"
dependencies: [2]
---

# Phase 3: Component MarkdownEditor

## Goal

Một component `MarkdownEditor` dùng được với react-hook-form, bọc MDXEditor 4.3 với cấu hình chỉ-GFM. Phải đáp ứng:

- không tạo ra định dạng mà server bỏ;
- ảnh chỉ vào qua upload media, có alt (D3);
- nhãn tiếng Việt;
- theme theo token GoUp;
- không làm form "bẩn" khi chỉ mở bài cũ;
- báo lỗi parse rõ ràng và cho sửa trong chế độ mã nguồn.

## Context & Requirements

- **Sở hữu file:** `apps/web/src/features/stages/components/markdown-editor/**` (mới),
  `apps/web/src/features/stages/test/dom-polyfills.ts`.
- Dùng từ phase 2:
  - `uploadMedia("image", …)`;
  - `imageFileError`, `altFromFileName`, `mediaContentUrl` trong `model/image-file.ts`;
  - `MEDIA_SRC`, `SAFE_URL` trong `model/markdown-rules.ts`;
  - `proseClass` trong `shared/ui/markdown-content.tsx`.
- Sự thật đã kiểm chứng về MDXEditor 4.3.2 (báo cáo nghiên cứu §2–§9):
  - **Uncontrolled.** `markdown` chỉ được đọc lúc mount; muốn reset thì đổi `key`.
  - **`onChange(md, initialMarkdownNormalize)`** gọi một lần lúc mount khi chuẩn hoá làm đổi source.
  - **`suppressHtmlProcessing`.** Dùng để `3<4` và `<https://…>` không lỗi. HTML thô, footnote và reference link
    vẫn gây `onError`.
  - **Transform không chạy với nội dung nạp lúc mount.** Có chạy khi gõ, dán, chèn và `setMarkdown`.
  - **Lỗi upload.** Promise bị reject trong upload handler thành unhandled rejection, không có UI báo lỗi.
  - **Dán và kéo thả ảnh.** Dán ảnh chèn `altText: ""`. Kéo thả chuỗi URL thì chèn thẳng src.
  - **`validateUrl` bị bỏ qua** bởi link dialog và markdown shortcut (issue #880). Vì vậy chặn link bằng transform
    `LinkNode`.
  - **Popup z-index là 2**, trong khi app dùng Dialog z-50. Phải override `.mdxeditor-popup-container`.
  - **Contenteditable có `role="textbox"`.** Nhãn lấy từ `translation('contentArea.editableMarkdown')`. Không có
    prop `aria-labelledby`.
- App không có dark mode, nên không cần `dark-theme`.

## Cấu trúc file

```text
features/stages/components/markdown-editor/
  markdown-editor.tsx          # MarkdownEditor (export default cho lazy) + props
  editor-plugins.tsx           # buildPlugins({ uploadPasted, ImageDialog }) + toolbar
  gfm-guard.ts                 # gfmGuard(), hardBreak(), pastedAltPlugin/transform alt
  upload-image-dialog.tsx      # ImageDialog chỉ-upload, alt bắt buộc
  editor-translation.ts        # map key → tiếng Việt
  markdown-editor.css          # theme bằng var() token, z-index popup, toolbar 44px
  gfm-guard.test.tsx           # transform/command trên editor thật trong jsdom
  markdown-editor.test.tsx     # mount/normalize/onChange/onError/aria
  upload-image-dialog.test.tsx # alt bắt buộc, lỗi upload, progress
```

## API component

```ts
export interface MarkdownEditorProps {
  value: string;                       // chỉ đọc lúc mount; cha đổi `key` để nạp lại
  onChange: (markdown: string) => void; // mọi sửa của người dùng (rich-text lẫn Mã nguồn); KHÔNG gọi cho lần chuẩn hoá lúc mount
  onPendingUploadsChange?: (count: number) => void; // số ảnh dán/kéo thả đang upload
  onBlur?: () => void;
  id?: string;                          // gắn lên contenteditable để <Label htmlFor> hoạt động
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  onParseError?: (message: string) => void;
  readOnly?: boolean;
}
export const MarkdownEditor = forwardRef<MarkdownEditorHandle, MarkdownEditorProps>(…)
export interface MarkdownEditorHandle {
  getMarkdown(): string;        // nguồn duy nhất cho lưu và xem trước
  violations(): string[];       // ảnh src ≠ MEDIA_SRC và link ≠ SAFE_URL trên cây Lexical hiện tại (gồm nội dung cũ nạp lúc mount)
  showSource(): void;           // chuyển diffSourcePlugin sang "source" (dùng cho Alert lỗi parse)
  focus(): void;
}
```

`Field` (shared) clone child với `id`, `aria-describedby`, `aria-invalid`. Wrapper nhận các prop này rồi dùng
`useEffect` gắn chúng lên `.mdxeditor-root-contenteditable [contenteditable]` qua ref container, mỗi khi prop đổi.
Nhờ vậy `Field` dùng được mà không phải sửa `shared`.

## Steps

1. **Spike ~2 giờ (làm đầu tiên).** Hook scout chặn mọi đường dẫn `node_modules`, nên đọc mã nguồn bằng
   `npm pack @mdxeditor/editor@4.3.2` (và `lexical@<x>`) giải nén vào scratchpad, như báo cáo nghiên cứu đã làm; chạy
   thử bằng một test jsdom tạm. Trả lời bốn câu hỏi, ghi vào "Concerns" của báo cáo phase:
   <!-- Updated: Red Team Session 1 - spike scope -->
   - **(S1) `onChange` ở chế độ Mã nguồn và trạng thái lỗi parse.** Sửa trong source mode có gọi `onChange` không?
     Nếu không, wrapper subscribe cell giá trị của source editor (ví dụ `markdownSourceEditorValue$`, tên lấy từ dist)
     và gọi `onChange` cho mỗi lần sửa. Hợp đồng bắt buộc: **mọi** sửa của người dùng ở cả hai chế độ đều gọi
     `onChange`, và `getMarkdown()` trả nội dung đang thấy.
   - **(S2) Ngôn ngữ code block không có trong `codeBlockLanguages`** (seed có ```` ```jsx ````; server cho mọi
     `language-[a-z0-9]+`). 4.3.2 ném lỗi render ("No CodeBlockEditor registered…", issue #423) hay fallback, và có
     giữ nguyên `language` khi xuất không?
   - **(S3) Ảnh dán: MDXEditor chèn node trước hay sau khi upload resolve?** (có placeholder `blob:` không).
   - **(S4) Alt cho ảnh dán/kéo thả**, hai cách dưới.
   - **Cách A.** Trong `createRootEditorSubscription$`, đăng ký `registerNodeTransform(ImageNode, …)`. Nếu
     `getAltText() === ""` và `pastedAlt.has(getSrc())`, thì gọi `setAltText(alt)` nếu method đó tồn tại; nếu không,
     thay node bằng `$createImageNode({ src, altText, title })` (hàm này được export từ `@mdxeditor/editor`).
   - **Cách B (dự phòng).** Lexical chỉ có priority 0–4 (`CRITICAL = 4`); cùng priority thì listener đăng ký trước
     chạy trước, và `imagePlugin` đăng ký trước `gfmGuard`. Vì vậy: **không** truyền `imageUploadHandler` cho
     `imagePlugin` (handler paste/drop của nó trả `false` khi không có upload handler — kiểm trong spike), rồi
     `gfmGuard` đăng ký `PASTE_COMMAND`/`DROP_COMMAND` ở `COMMAND_PRIORITY_CRITICAL`. Khi payload có file ảnh, tự
     upload (qua `uploadPasted`) rồi publish `insertImage$({ src, altText: altFromFileName(name) })`, trả `true`. Khi
     payload là chuỗi URL thả vào, trả `true` để bỏ qua. Nếu spike cho thấy imagePlugin vẫn chặn trước, đặt một
     `realmPlugin` riêng **trước** `imagePlugin` trong mảng plugin.
   - Ghi kết quả spike vào phần "Concerns" của báo cáo phase.
2. **`gfm-guard.ts`** theo báo cáo nghiên cứu §11:
   - **`gfmGuard()`.** Một `realmPlugin` đăng ký trong `createRootEditorSubscription$`:
     - `FORMAT_TEXT_COMMAND` ở `COMMAND_PRIORITY_CRITICAL`, chặn `underline`, `subscript`, `superscript`,
       `highlight`.
     - Transform `TextNode`: bỏ bit `8 | 32 | 64 | 128` và xóa `style`.
     - Transform `ImageNode`: `remove()` khi `!MEDIA_SRC.test(getSrc())` — **một quy tắc duy nhất**, gồm cả src rỗng.
       Không có ngoại lệ `blob:`/placeholder: nếu spike S3 cho thấy MDXEditor chèn placeholder trước khi upload xong,
       dùng Cách B (chỉ chèn node sau khi có src media) thay vì nới quy tắc.
     - Transform `ListNode`: `check` thành `bullet`.
     - Transform `LinkNode`: khi `!SAFE_URL.test(getURL())`, đưa các con ra ngoài rồi `remove()`.
     - Transform `HeadingNode`: h5 và h6 thành h4.
     - Hàm cleanup gọi mọi hàm `off`.
   - **`hardBreak()`.** Một export visitor cho `$isLineBreakNode`, `priority: 100`, `appendToParent({ type: "break" })`.
   - **Alt cho ảnh dán.** Cài theo kết quả spike. Map `pastedAlt: Map<src, alt>` được tạo trong closure của từng
     instance editor, không đặt ở biến module, để hai editor không dùng chung.
   - **Import.** `TextNode`, `FORMAT_TEXT_COMMAND`, `COMMAND_PRIORITY_CRITICAL`, `$isLineBreakNode` từ `lexical`;
     `ListNode` từ `@lexical/list`; `LinkNode` từ `@lexical/link`; `HeadingNode`, `$createHeadingNode` từ
     `@lexical/rich-text`; `ImageNode`, `realmPlugin`, `createRootEditorSubscription$`, `addExportVisitor$` từ
     `@mdxeditor/editor`.
3. **`editor-plugins.tsx`**, hàm `buildPlugins({ uploadPasted, ImageDialog })`, theo đúng thứ tự sau:
   - Plugin: `headingsPlugin({ allowedHeadingLevels: [1, 2, 3, 4] })`, `listsPlugin()`, `quotePlugin()`,
     `thematicBreakPlugin()`, `linkPlugin({ validateUrl: (u) => SAFE_URL.test(u) })`, `linkDialogPlugin()`,
     `imagePlugin({ imageUploadHandler: uploadPasted, disableImageResize: true, ImageDialog })`, `tablePlugin()`,
     `codeBlockPlugin({ defaultCodeBlockLanguage: "" })`.
   - `codeMirrorPlugin({ codeBlockLanguages: { "": "Văn bản", ts: "TypeScript", tsx: "TSX", js: "JavaScript", jsx: "JSX", go: "Go", sql: "SQL", bash: "Bash", json: "JSON", html: "HTML", css: "CSS" } })`.
     Các khóa phải khớp `codeClassPattern` của server (`^language-[a-z0-9]+$`).
   - **Fallback cho ngôn ngữ lạ** (theo kết quả S2): nếu 4.3.2 ném lỗi hoặc làm mất `language`, thêm
     `codeBlockPlugin({ codeBlockEditorDescriptors: [fallbackCodeBlock] })` với `priority` thấp nhất, `match: () => true`,
     editor là CodeMirror không highlight (hoặc `<textarea>` mono) dùng `useCodeBlockEditorContext().setCode`, và
     **giữ nguyên** `language`/`meta`. Bài có `python`, `yaml`… mở được và lưu không đổi fence.
     <!-- Updated: Red Team Session 1 - unknown fence language -->
   - `diffSourcePlugin({ viewMode: "rich-text" })`.
   - `toolbarPlugin`, chứa `DiffSourceToggleWrapper options={["rich-text", "source"]}` bọc: `UndoRedo`,
     `BlockTypeSelect`, `BoldItalicUnderlineToggles options={["Bold", "Italic"]}`,
     `StrikeThroughSupSubToggles options={["Strikethrough"]}`, `CodeToggle`,
     `ListsToggle options={["bullet", "number"]}`, `CreateLink`, `InsertImage`, `InsertTable`,
     `InsertThematicBreak`, `InsertCodeBlock`, ngăn bằng `Separator`.
   - Ba plugin cuối: `gfmGuard()`, `hardBreak()`, và **`markdownShortcutPlugin()` ở cuối cùng**.
4. **`upload-image-dialog.tsx`** là `ImageDialog` tùy biến:
   - **State.** `const [state] = useCellValues(imageDialogState$)`; `saveImage = usePublisher(saveImage$)`;
     `close = usePublisher(closeImageDialog$)`.
   - **Giao diện.** Dùng `Dialog` và `Field` của `shared/ui`:
     - input file với `accept={IMAGE_CONTENT_TYPES.join(",")}`;
     - ô "Mô tả ảnh (alt)" bắt buộc, `maxLength 200`, tự điền `altFromFileName` khi chọn file và người dùng chưa gõ;
     - thanh tiến độ (`shared/ui/progress`), nút "Chèn ảnh" và "Hủy".
   - **Kiểm và upload.** Kiểm `imageFileError`, rồi gọi `uploadMedia("image", file, { onProgress, signal })`.
     Thành công thì `saveImage({ src: mediaContentUrl(media.id), altText: alt.trim() })` rồi `close()`.
     Lỗi thì giữ dialog mở và hiện `Alert` với thông điệp từ `isApiError` hoặc `UPLOAD_FAILED_MESSAGE`.
     Đóng dialog thì abort upload.
   - **Sửa ảnh.** Ở `state.type === "editing"`, chỉ cho sửa alt (giữ src) và không yêu cầu file mới.
   - **Phạm vi.** Dialog nằm trong editor ở trang riêng (D1), không lồng trong một Dialog khác, nên không gặp vấn đề
     focus trap.
   - **Kiểm chứng sau khi cài.** Kiểm `Dialog` của app hiện trên popup container của MDXEditor; nếu không thì truyền
     `overlayContainer` (xem báo cáo nghiên cứu §7).
5. **`uploadPasted(file)`**, handler cho dán và kéo thả, luôn resolve và không bao giờ reject:
   - **Giới hạn**: tối đa 5 ảnh đang chờ/đang tải mỗi editor; vượt thì resolve `""` và một toast duy nhất
     (`toast.error(…, { id: "paste-limit" })`) "Chỉ tải tối đa 5 ảnh mỗi lần.". Upload **tuần tự** (hàng đợi một
     luồng) để không đốt hạn mức `/media` 120 req/phút; 429 coi như lỗi upload.
   - Lỗi `imageFileError` thì `toast.error(msg)` rồi resolve `""`.
   - Hợp lệ thì upload. Thành công: ghi `pastedAlt.set(src, altFromFileName(file.name))` rồi resolve `src`.
     Lỗi: `toast.error(UPLOAD_FAILED_MESSAGE, { id: "paste-upload" })` rồi resolve `""`. Ảnh có src `""` bị transform
     `ImageNode` gỡ (quy tắc duy nhất ở bước 2).
   - **Đếm upload đang chạy**: tăng khi bắt đầu, giảm trong `finally`, báo qua `onPendingUploadsChange`. Trang (phase 4)
     khóa Lưu và giữ blocker khi > 0, để không lưu mất ảnh đang tải. <!-- Updated: Red Team Session 1 - in-flight uploads -->
6. **`editor-translation.ts`**:
   - Hàm `translate(key, defaultValue, interpolations)` dùng map tiếng Việt cho các key xuất hiện ở toolbar, dialog
     link, chọn block, bảng, code và diff-source.
   - `contentArea.editableMarkdown` dịch thành "Nội dung bài học".
   - Key không có trong map thì trả về `defaultValue`, có nội suy `{{x}}`.
   - Lấy danh sách key bằng cách grep `t('` trong dist của gói đã `npm pack` vào scratchpad (bước 1; hook chặn
     `node_modules`). Không đoán key.
7. **`markdown-editor.tsx`**:
   - **Plugin.** `useMemo` dựng plugins một lần cho mỗi instance.
   - **Cấu hình MDXEditor.**
     - `markdown={value}`, `suppressHtmlProcessing`, `translation={translate}`.
     - `contentEditableClassName={cn(proseClass, "min-h-[320px] max-w-none px-4 py-3")}` (`max-w-none` để editor
       rộng hết khung).
     - `className="goup-mdx"`, `readOnly`, `onBlur`.
   - **`onChange`.** `(md, initialNormalize) => { if (!initialNormalize) onChange(md) }`, cộng đường source mode theo
     kết quả S1.
   - **Handle.** `getMarkdown()` từ ref MDXEditor; `violations()` đọc `editor.getEditorState().read(…)` duyệt
     `$getRoot()` lấy `ImageNode` (`!MEDIA_SRC.test(src)` → `src`) và `LinkNode` (`!SAFE_URL.test(url)` → `url`);
     `showSource()` publish `viewMode$("source")`. Ở source mode, `violations()` trả `[]` (cây Lexical không phản ánh
     bản đang sửa) — server là chốt cuối.
   - **`onError`.** `({ error }) => onParseError?.(error)`. Phía cha (phase 4) hiện `Alert` "Nội dung có cú pháp
     editor chưa hỗ trợ (HTML thô, chú thích…). Chuyển sang chế độ Mã nguồn để sửa." và nút chuyển chế độ: publish
     `viewMode$("source")` qua một plugin nhỏ, hoặc dùng nút sẵn của `DiffSourceToggleWrapper`.
   - **Phạm vi import.** Chỉ file này import `@mdxeditor/editor/style.css` và `./markdown-editor.css`, để CSS chỉ
     nằm trong chunk lazy.
8. **`markdown-editor.css`**: theme `.goup-mdx, .goup-mdx.mdxeditor-popup-container`.
   - Biến màu: `--accentBase…`, `--baseBase…` lấy từ `var(--color-navy-700)`, `var(--color-line)`,
     `var(--color-surface-50)`, `var(--color-ink)` và các token khác. Không dùng hex thô.
   - Font: `--font-body: var(--font-sans)`, `--font-mono` lấy theo token mono nếu có, nếu không thì `ui-monospace`.
   - `.mdxeditor-popup-container { z-index: 60 }` để nằm trên Dialog z-50 của app khi dialog ảnh mở popup.
   - Toolbar: `flex-wrap`. Dưới 720px, nút toolbar có `min-width` và `min-height` 44px; nếu `lint:design` bắt
     kích thước gốc của MDXEditor thì ghi `design-audit-allow`.
   - Nút điều khiển trong code block (chọn ngôn ngữ, xóa) và bảng (thêm/xóa hàng/cột) cũng ≥ 44px dưới 720px. Nếu một
     control nội bộ không chỉnh được, ghi rõ ngoại lệ trong báo cáo phase để E42 (phase 5) loại trừ có chủ đích.
   - Khung editor có viền và focus ring như `Input`.
9. **`dom-polyfills.ts`**: thêm `Range.prototype.getBoundingClientRect` và `getClientRects` (no-op), giống báo cáo
   nghiên cứu §8.
10. **Unit test (jsdom)**, mỗi test có helper `await settle()` chờ khoảng 50 ms:
    - **`gfm-guard.test.tsx`.** Lấy `LexicalEditor` qua một `realmPlugin` test cùng `createRootEditorSubscription$`,
      rồi kiểm:
      - `dispatchCommand(FORMAT_TEXT_COMMAND, "underline")` không đổi markdown;
      - `setMarkdown("<u>x</u> ==y== H<sup>2</sup>")` cho kết quả không chứa `<u>`, `==` hay `<sup>`;
      - `setMarkdown("##### h5")` thành `#### h5`;
      - `setMarkdown("- [ ] a")` thành bullet không có `[ ]`;
      - `setMarkdown("[x](javascript:alert(1))")` chỉ còn chữ `x`;
      - `setMarkdown("![a](https://x/y.png)")` bị gỡ ảnh, còn ảnh media thì giữ;
      - hard break (`a\\\nb`) round-trip ổn định.
    - **`markdown-editor.test.tsx`.**
      - Mount với `"- a\n- b"` thì `onChange` không được gọi (trường hợp normalize).
      - Mount với `"# A"` (đã canonical) thì `onChange` không được gọi.
      - Mount với `"<div>x</div>"` thì `onParseError` được gọi.
      - Có `role="textbox"` với nhãn "Nội dung bài học", và `id`/`aria-describedby` được gắn.
      - Với **cả 7** bài seed (golden `apps/api/internal/seed/testdata/seed_markdown.json`, nạp qua `test/golden.ts`;
        goldens `stage_version*.json` chỉ chứa fixture ngắn, không phải nội dung seed): mount không ném lỗi, không gọi
        `onParseError`, không gọi `onChange`; fence `jsx` vẫn là `jsx` trong `getMarkdown()`.
      - Fence ngôn ngữ lạ (```` ```python ````) mount không ném lỗi và giữ `python` khi xuất.
      - Mount nội dung cũ có `![a](https://x/y.png)` và `[l](javascript:x)`: transform không chạy lúc mount, nhưng
        `violations()` trả cả hai.
      - Sửa trong chế độ Mã nguồn gọi `onChange` với nội dung mới (hợp đồng S1).
      - `uploadPasted` với upload treo: `onPendingUploadsChange` báo 1 rồi 0 sau khi resolve; ảnh thứ 6 trong hàng đợi
        bị từ chối với một toast.
    - **`upload-image-dialog.test.tsx`.**
      - Thiếu alt thì nút "Chèn ảnh" disabled hoặc báo lỗi.
      - File sai loại thì báo thông điệp.
      - Upload lỗi (MSW `/media/uploads` trả 500) thì hiện `Alert` và dialog vẫn mở.
      - Thành công thì `saveImage$` nhận src media và alt. Kiểm qua markdown của editor: có chứa `![alt](/api/v1/media/…/content)`.

## Verification

```bash
cd apps/web
pnpm test -- src/features/stages/components/markdown-editor
pnpm typecheck && pnpm lint && pnpm lint:design
```

Hành vi gõ phím, dán HTML thật, kéo thả và Ctrl+U trên trình duyệt được kiểm ở phase 5 (Playwright).

## Risk

- **Key translation đổi giữa các bản patch.** Đã ghim `~4.3.2`, và test aria-label bắt được khi nhãn chính đổi.
- **Transform `ImageNode` gỡ nhầm ảnh đang upload.** Chỉ xảy ra nếu MDXEditor chèn placeholder trước khi upload xong
  (spike S3). Khi đó chuyển sang Cách B (node chỉ được chèn khi đã có src media); **không** nới quy tắc gỡ ảnh.
- **CSS của MDXEditor đè prose.** CSS theme đặt sau `style.css`, và specificity được kiểm bằng ảnh chụp ở phase 5.

## Rollback

Xóa thư mục `components/markdown-editor/` và các polyfill vừa thêm. Chưa có trang nào dùng component này cho tới
phase 4.
