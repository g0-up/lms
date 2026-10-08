---
phase: 4
title: "Phase 4: Trang soạn + luồng điều hướng + chặn rời trang"
status: completed
priority: P2
effort: "1 ngày"
dependencies: [2, 3]
---

# Phase 4: Trang soạn học liệu markdown

## Goal

Thêm trang soạn riêng (D1) cho học liệu markdown. Trang có tiêu đề, cờ "bắt buộc", tab Soạn thảo và tab Xem trước
(D4), lưu qua mutation hiện có và chặn rời trang khi chưa lưu. Dialog học liệu chuyển markdown sang trang này, còn
video giữ nguyên.

## Context & Requirements

- **Sở hữu file:** `apps/web/src/features/stages/{routes.tsx,pages/**,components/lesson-form-dialog.tsx,model/lesson-form.ts(+test),hooks/use-lesson-mutations.ts}`.
  Không sửa `src/app`. Route mới đăng ký qua `features/stages/routes.tsx` với path tương đối.
- Dữ liệu:
  - `useStageVersion(vid)` trả `{id, stageId, stageCode, stageName, versionNo, status, lessons[]}`.
  - Ở bản nháp, lesson có `markdownSource`; `markdownHtml` chỉ có khi phiên bản không còn là nháp.
  - `useLessonMutations(stageId, vid).save` nhận `{lessonId?, values}`, tự toast "Đã lưu học liệu." hoặc
    "Đã thêm học liệu.", invalidate version/detail/list và đặt `meta.silent` để form tự hiện lỗi.
- Trang chi tiết chặng đọc phiên bản từ search param `v` qua `selectVersion`. URL quay về là
  `/admin/stages/:stageId?v=:versionId`.
- Router test dùng `createMemoryRouter` (data router), nên `useBlocker` chạy được trong Vitest.
- Radix Tabs mặc định **unmount** nội dung tab không active. Tab Soạn thảo phải dùng `forceMount` và ẩn bằng
  `hidden`/`data-state`, nếu không editor bị mount lại (uncontrolled) và mất trạng thái undo/selection.

## Thiết kế

### Route (thêm vào `routes.tsx`, đều `lazy`)

| Path (tương đối khu admin) | Trang | `handle.title` |
|---|---|---|
| `stages/:stageId/versions/:versionId/lessons/new` | `lesson-editor-page` (tạo) | "Soạn học liệu" |
| `stages/:stageId/versions/:versionId/lessons/:lessonId/edit` | `lesson-editor-page` (sửa) | "Soạn học liệu" |

Một module `pages/lesson-editor-page.tsx` dùng cho cả hai route, phân biệt bằng việc có `lessonId` hay không. Chỉ
module này import `MarkdownEditor`, nên editor và CSS nằm trong chunk lazy riêng của route.

### Bố cục trang

```text
Crumbs: Chặng › {stageCode} v{n} › Soạn học liệu
PageHead: "Thêm học liệu markdown" | "Sửa: {title}"      [Hủy] [Lưu]
Alert lỗi (nếu có: 422 ảnh ngoài / vi phạm ảnh-link (liệt kê) / VERSION_IMMUTABLE + "Sao chép markdown" / quá lớn / parse error + nút "Mở chế độ Mã nguồn")
Field "Tiêu đề" (Input)          Checkbox "Bắt buộc (tính vào %)"
Tabs: [Soạn thảo] [Xem trước]
  Soạn thảo: <Field label="Nội dung"> <MarkdownEditor …/> </Field>   (forceMount, ẩn khi không active)
  Xem trước: Skeleton → <MarkdownContent html=…/> | Alert lỗi | EmptyState "Chưa có nội dung."
Dòng phụ: "{jsonBodyBytes(body)} / 64 KB"  (đổi màu cảnh báo khi > 90%; cập nhật theo onChange, có debounce)
```

### Form và model (`model/lesson-form.ts`)

- **`markdownLessonSchema`** (mới): `{title, required, markdownSource}`.
  - Bắt buộc có tiêu đề, dùng `LESSON_TITLE_REQUIRED`.
  - `markdownSource` qua `superRefine(markdownSourceError)` của phase 2 (rỗng, base64). Kích thước kiểm trên body
    JSON thật lúc submit bằng `lessonBodySizeError` (phase 2).
- **`lessonFormSchema`** (dùng cho dialog): nhánh markdown **bỏ** `markdownSource` và refine của nó. Dialog chỉ còn
  thu `title` và `required` cho markdown.
- **`toLessonBody` / `toLessonPatch`**: nhận union `video output | markdownLesson output`.
  - `toLessonPatch(values, { keepSource })` bỏ `markdownSource` khỏi body khi `keepSource` đúng. Như vậy đổi tiêu đề
    bài cũ không gửi lại source, khớp với luật server chỉ kiểm ảnh khi có source mới.
  - `LessonPatch` nhánh markdown đổi thành `markdownSource?: string`.
- **Một nguồn sự thật cho nội dung** <!-- Updated: Red Team Session 1 - source of truth -->:
  - `contentTouched` (state + ref) bật khi editor gọi `onChange` — hợp đồng phase 3: mọi sửa của người dùng ở cả
    rich-text lẫn Mã nguồn, không gồm chuẩn hoá lúc mount.
  - Lúc submit: nếu `contentTouched` thì `source = editorRef.current.getMarkdown()` (không dùng giá trị field RHF), kiểm
    `markdownSourceError(source)`, `editorRef.current.violations()` và `lessonBodySizeError(body)`, rồi gửi; nếu không
    thì `keepSource = true` và bỏ `markdownSource` khỏi body.
  - **Không bao giờ so với dữ liệu query đang sống** (`lesson.markdownSource` có thể đổi khi refetch lúc focus cửa
    sổ, `staleTime` 30 s). Default của form chỉ nạp **một lần** khi version tải xong lần đầu; refetch không `reset`
    form, không đổi `key` editor.
  - Nhờ vậy mở rồi lưu không đổi byte nào (tiêu chí 7), và đổi tiêu đề không ghi đè source người khác vừa lưu.
- **Ghi đè đồng thời giữa hai admin** (last-write-wins khi **cả hai** cùng sửa nội dung): giữ như hiện nay, API không có
  token phiên bản cho lesson; ngoài phạm vi.
- **`SaveLessonInput`** thêm `keepSource?: boolean`. `use-lesson-mutations.ts` truyền xuống `toLessonPatch`.

### Luồng điều hướng

- **Dialog "Thêm học liệu", loại Markdown:**
  - Ẩn Textarea; nút submit đổi nhãn thành "Tiếp tục soạn".
  - Submit kiểm `title` rồi gọi `onContinueMarkdown({title, required})`.
  - Trang chi tiết đóng dialog rồi gọi
    `navigate("stages/:stageId/versions/:vid/lessons/new" (đường dẫn tuyệt đối /admin/...), { state: {title, required} })`.
- **Dialog loại Video:** giữ nguyên toàn bộ.
- **"Sửa" trên bài markdown** (`LessonList.onEdit` ở `stage-detail-page.tsx`): khi `lesson.type === "markdown"` thì
  `navigate(".../lessons/:lessonId/edit")`, không mở dialog. Bài video vẫn mở dialog.
- **Bỏ code chết:** xóa nhánh `markdownSource` và `MARKDOWN_HELP` trong `lesson-form-dialog.tsx`. Dialog sửa không
  còn bao giờ nhận lesson markdown.
- **Sau khi lưu thành công:**
  - Đặt `allowLeave.current = true` ngay trước `navigate`. **Chỉ dùng ref**: `form.reset()` không kịp, vì `useBlocker`
    cài hàm chặn mới trong effect sau render nên `navigate` cùng tick vẫn thấy `isDirty` cũ.
  - Gọi `navigate(\`/admin/stages/${stageId}?v=${versionId}\`, { replace: true })`. Toast đã có sẵn trong mutation.
- **"Hủy":** navigate về cùng URL; blocker sẽ hỏi nếu form đang dirty.
- **Tạo mới mà không có router state** (tải lại trang hoặc mở link trực tiếp): tiêu đề rỗng, `required` mặc định
  `true` như `lessonFormDefaults`.

### Trạng thái đặc biệt

- **Đang tải version:** `Skeleton`.
- **404 version, hoặc `version.stageId !== stageId`:** dùng `EmptyState` "Không tìm thấy phiên bản." kèm link về
  danh sách chặng.
- **Sửa mà `lessonId` không có trong `version.lessons`, hoặc lesson là video:** dùng `EmptyState`
  "Không tìm thấy học liệu markdown." kèm link về chặng.
- **`version.status !== "draft"`:** `Alert` "Phiên bản đã phát hành hoặc lưu trữ, không thể sửa. Nhân bản thành bản
  nháp để chỉnh." kèm link về chặng. Không render editor và không có nút Lưu.
- **Lưu nhận `VERSION_IMMUTABLE`:** hiện `VERSION_IMMUTABLE_MESSAGE`, khóa nút Lưu, chuyển editor sang `readOnly`
  (vẫn đọc được) và thêm nút "Sao chép markdown" (`navigator.clipboard.writeText(getMarkdown())`, toast "Đã sao
  chép.") để không mất công soạn. Mutation đã tự refresh. Không tự đổi `key` editor khi version refetch về published.
- **Có ảnh dán/kéo thả đang upload** (`onPendingUploadsChange > 0`): nút Lưu disabled với nhãn "Đang tải ảnh…".
- **`violations()` không rỗng lúc submit:** không gửi; `Alert` "Gỡ các ảnh/liên kết không hợp lệ trước khi lưu:" kèm
  danh sách (tối đa 5 mục + "và N mục khác"). Bắt cả bài cũ mà transform không chạy lúc mount.

### Tab Xem trước

- Khi chuyển sang tab, chụp `source = editorRef.current.getMarkdown()` — cùng getter với lúc lưu.
- Gọi `useQuery({ queryKey: ["stage-markdown-preview", source], queryFn: ({ signal }) => stagesApi.previewMarkdown(source, signal), enabled: tab === "preview" && source.trim() !== "", staleTime: 60_000, gcTime: 60_000 })`.
  - Root key **riêng** (không bắt đầu bằng `stageKeys.list = ["stages"]`), để `invalidateQueries(["stages"])` sau mỗi
    lần lưu không refetch preview và không làm chậm `onSuccess`. <!-- Updated: Red Team Session 1 - query key -->
  - Không đặt `meta`: query client không toast lỗi query (chỉ mutation có `mutationMeta`).
  - Key theo source, nên mở lại tab khi không sửa gì sẽ dùng cache.
- Lỗi 422 (ảnh ngoài) hiện thông điệp server trong `Alert`.
- Render bằng `MarkdownContent`, tức cùng component và cùng class prose với trang học viên.

### Chặn rời trang khi chưa lưu

- `dirty = formState.isDirty || contentTouched || pendingUploads > 0`.
- Gọi `useBlocker(({ currentLocation, nextLocation }) => dirtyRef.current && !allowLeave.current && currentLocation.pathname !== nextLocation.pathname && !AUTH_PATHS.has(nextLocation.pathname))`
  với `AUTH_PATHS = new Set([LOGIN_PATH, FIRST_LOGIN_PATH])` (import từ `@/features/auth`).
  - **Không chặn chuyển hướng xác thực**: khi 401, `query-client.ts` đã `resetSession` (xóa cache) rồi điều hướng tới
    `/login`. Chặn ở đây sẽ giữ nội dung của phiên cũ trên màn hình (phá bất biến của `session-isolation.test.tsx`) và
    kẹt người dùng trong vòng 401. Không lưu nháp vào storage (rò nội dung giữa người dùng).
    <!-- Updated: Red Team Session 1 - auth redirect -->
  - Đọc `dirty` qua ref để hàm chặn không dùng closure cũ.
  Khi `blocker.state === "blocked"` thì hiện `ConfirmDialog`:
  - tiêu đề "Rời trang?", nội dung "Nội dung chưa lưu sẽ mất.";
  - nút "Rời trang" (danger) gọi `blocker.proceed()`, "Ở lại" gọi `blocker.reset()`.
- Thêm `useEffect` đăng ký `beforeunload` (`preventDefault` cộng `returnValue = ""`) khi `dirty`.

### Lỗi parse editor

- Khi `onParseError` được gọi, hiện `Alert` kèm nút "Mở chế độ Mã nguồn" gọi `editorRef.current.showSource()`.
- Không chặn lưu: sửa trong chế độ Mã nguồn gọi `onChange` (hợp đồng S1 phase 3) nên `contentTouched` bật và submit
  gửi `getMarkdown()`. Không sửa gì thì source cũ được giữ nguyên (`keepSource`).

## Files

| File | Thay đổi |
|---|---|
| `src/features/stages/routes.tsx` | Thêm 2 route lazy. |
| `src/features/stages/pages/lesson-editor-page.tsx` (mới) | Trang soạn như trên (export `Component` hoặc `default` theo quy ước `lazy` của các route hiện có). |
| `src/features/stages/pages/lesson-editor-page.test.tsx` (mới) | Test trang. |
| `src/features/stages/components/lesson-form-dialog.tsx` | Bỏ nhánh Textarea markdown; thêm prop `onContinueMarkdown`; nút "Tiếp tục soạn". |
| `src/features/stages/pages/stage-detail-page.tsx` | `onEdit` rẽ nhánh theo loại; nối `onContinueMarkdown` với `navigate`. |
| `src/features/stages/pages/stage-detail-page.test.tsx` | Cập nhật test markdown (~dòng 424) **và** "explains VERSION_IMMUTABLE inside the dialog" (~dòng 448, đang bấm "Sửa" bài markdown). |
| `src/features/stages/test/*` (world/MSW; không đụng `golden.ts` của phase 2, `dom-polyfills.ts` của phase 3) | Thêm một bài video vào draft world (hoặc override trong test) để dialog sửa video còn fixture cho VERSION_IMMUTABLE; golden draft hiện chỉ có bài markdown. |
| `src/features/stages/model/lesson-form.ts` (+ `.test.ts`) | `markdownLessonSchema`, union cho body/patch, `keepSource`. |
| `src/features/stages/hooks/use-lesson-mutations.ts` | `SaveLessonInput.keepSource`. |
| `src/features/stages/test/render-stages.tsx` | Helper `editorUrl(stageId, vid, lessonId?)`. |

## Steps

1. Model: viết `markdownLessonSchema`, rút gọn nhánh markdown của `lessonFormSchema`, đổi chữ ký
   `toLessonBody`/`toLessonPatch`, cập nhật `lesson-form.test.ts`:
   - `keepSource` bỏ `markdownSource` khỏi body;
   - source rỗng hoặc có `data:image/` bị chặn; body JSON quá 64 KB bị `lessonBodySizeError` chặn.
2. `use-lesson-mutations.ts`: truyền `keepSource`.
3. Dialog: thêm `onContinueMarkdown`, bỏ Textarea và `MARKDOWN_HELP`. Loại học liệu vẫn disabled khi sửa (hiện chỉ
   còn bài video được sửa qua dialog).
4. `stage-detail-page.tsx`: `useNavigate`, rẽ nhánh `onEdit`, nối `onContinueMarkdown`. Đường dẫn tạo bằng một
   helper thuần trong `model` (ví dụ `lessonEditorPath(stageId, vid, lessonId?)`) để page, test và dialog dùng chung.
5. `lesson-editor-page.tsx` theo thiết kế:
   - `react-hook-form` + `zodResolver(markdownLessonSchema)`.
   - `Controller` cho `markdownSource` render `MarkdownEditor`, với `key={lessonId ?? "new"}` và
     `value={field.value}`, `onChange={field.onChange}`, `onBlur={field.onBlur}`.
   - Submit theo "Một nguồn sự thật": `save.mutateAsync({ lessonId, values: { ...values, markdownSource: source }, keepSource: !contentTouched })`.
   - Map lỗi: `VERSION_IMMUTABLE` dùng thông điệp riêng; 422 dùng `error.message`; 400 có `details.body` dùng
     `details.body`; còn lại dùng `error.message`.
6. Test trang (`lesson-editor-page.test.tsx`), với Vitest + MSW + `renderStages(editorUrl(…))`:
   - **Test double cho editor.** `vi.mock("../components/markdown-editor/markdown-editor", …)` thay `MarkdownEditor`
     bằng `<textarea>` có cùng props và handle: gọi `onChange` khi gõ, gắn `id` và aria, `getMarkdown` trả value,
     `violations()` trả ảnh/link vi phạm bằng regex đơn giản trên value, `showSource()` no-op; một nút ẩn trong
     double gọi `onPendingUploadsChange(1)` để test khóa Lưu. Ghi rõ trong
     comment rằng hành vi editor thật đã được kiểm ở test của phase 3 và E2E phase 5. Đây là test double, không phải
     dữ liệu giả trong app.
   - Mở bài markdown golden: tiêu đề và nội dung được nạp, nút Lưu bấm được.
   - Lưu khi chỉ đổi tiêu đề: body PATCH là `{title, required}`, **không có** `markdownSource`.
   - Sửa nội dung rồi lưu: body có `markdownSource`, sau đó điều hướng về `/admin/stages/:id?v=:vid` và có toast.
   - Tạo mới từ router state: tiêu đề và cờ bắt buộc được điền sẵn; POST có `type: "markdown"`.
   - Server trả 422 ảnh ngoài: `Alert` hiện đúng thông điệp và vẫn ở lại trang.
   - Tab Xem trước: gọi `POST /stages/markdown-preview` với source hiện tại và render HTML golden. Đổi tab qua lại
     không mất nội dung, nhờ editor được `forceMount`.
   - Dirty rồi bấm "Hủy": `ConfirmDialog` hiện ra. "Ở lại" giữ trang, "Rời trang" thì điều hướng.
   - Phiên bản published: không có editor, không có nút Lưu, có `Alert` hướng dẫn nhân bản.
   - Lưu thành công: điều hướng **không** hiện `ConfirmDialog`.
   - Dirty rồi một request trả 401: về `/login` **không** hiện `ConfirmDialog`.
   - Version refetch trả `markdownSource` khác (giả lập admin khác), người dùng chỉ đổi tiêu đề: PATCH không có
     `markdownSource`.
   - Nội dung có ảnh ngoài (bài cũ): Lưu hiện danh sách vi phạm, không gửi request.
   - Đang upload ảnh (`onPendingUploadsChange(1)`): nút Lưu disabled.
   - `VERSION_IMMUTABLE` khi lưu: editor readOnly, nút "Sao chép markdown" ghi clipboard.
   - Body JSON > 64 KB (nhiều xuống dòng/ngoặc kép): báo lỗi, không gửi.
   - `lessonId` không tồn tại: hiện `EmptyState`.
7. Cập nhật `stage-detail-page.test.tsx`:
   - Thay test "edits a markdown lesson without changing its type" bằng:
     - "Sửa" trên bài markdown điều hướng tới trang soạn;
     - thêm bài loại Markdown, sau "Tiếp tục soạn", tới trang soạn có state.
   - Chuyển assert VERSION_IMMUTABLE của bài markdown sang `lesson-editor-page.test.tsx`; test dialog VERSION_IMMUTABLE
     đổi sang bài video (fixture mới trong world/MSW của `test/`).
   - Giữ nguyên các test video.

## Verification

```bash
cd apps/web
pnpm test -- src/features/stages
pnpm typecheck && pnpm lint && pnpm lint:design
pnpm build && ls dist/assets | grep -i lesson-editor   # chunk riêng cho trang soạn
```

## Risk

- **`useBlocker` chặn cả lần điều hướng sau khi lưu.** Dùng ref `allowLeave` đặt trước `navigate` (không dùng
  `form.reset`), có test riêng.
- **Mất router state khi tải lại trang tạo mới.** Chấp nhận được: form rỗng và người dùng nhập lại tiêu đề.
- **Tab ẩn editor bằng `hidden` làm popup của MDXEditor tính sai vị trí khi hiện lại.** Popup chỉ mở khi tab Soạn
  thảo active nên không xảy ra; kiểm thêm trong E2E.
- **Người dùng quen sửa markdown trong dialog.** Thay đổi có chủ đích (D1); E2E E02 (video) không bị ảnh hưởng.

## Rollback

Revert các file của phase này. Dialog quay lại dùng Textarea. Phase 1–3 không phụ thuộc phase 4.
