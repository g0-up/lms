# Báo cáo triển khai: Soạn quiz và homework (web)

- Phase: [phase-05-quiz-homework-authoring-web.md](../phase-05-quiz-homework-authoring-web.md)
- Trạng thái: hoàn thành. Toàn bộ Vitest web xanh. Playwright đã viết và type-check, nhưng chưa chạy (controller chạy E2E).
- Ngày: 2026-10-19. Chưa commit.

## Kết quả

Form học liệu có select "Loại" với năm giá trị, nhãn lấy nguyên văn prototype: "Video", "Bài đọc (Markdown)", "Quiz học liệu (luyện tập, chấm tự động)", "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)", "Bài tập nộp repo hoặc file (giảng viên chấm)".

- Help text theo spec. Đổi giữa hai loại quiz chỉ gửi `quiz` (không gửi `type`) nên câu hỏi được giữ.
- Đổi sang loại khác lúc đang sửa thì hiện hộp xác nhận `Đổi loại "{title}"?` trước khi lưu.
- Quiz bài tập: các trường "Ngưỡng đạt (%)" (mặc định 70), "Số lượt làm bài" (mặc định 2, help "Tính lượt tốt nhất") và "Thời gian mỗi lượt (phút)" (mặc định 20).
- Cả hai loại quiz có checkbox "Đảo thứ tự câu hỏi và đáp án mỗi lượt", mặc định bật.
- Homework: "Ngưỡng đạt (%)", mặc định 70.
- Tạo quiz hoặc homework xong thì chuyển thẳng tới trang soạn của học liệu đó.

Trang soạn học liệu chọn phần soạn theo loại (`GradedLessonEditor` cho quiz và homework).

Quiz:

- Thẻ facts cấu hình theo chế độ, có nút "Sửa thiết lập".
- Danh sách "Câu hỏi {n} câu · {total} điểm". Mỗi dòng có "Đáp án đúng: …" và các nút lên, xuống, "Sửa câu i", "Xóa câu i".
- Dialog câu hỏi:
  - Nội dung và giải thích là textarea markdown, xem trước qua `/stages/markdown-preview`.
  - "Loại câu", "Điểm", và sáu ô phương án, mỗi ô có checkbox "Đúng".
  - Lỗi dùng copy của server. Lỗi nhóm phương án gắn vào `fieldset` qua `aria-describedby`.
  - Khi sửa, id phương án được giữ.

Homework:

- Thẻ facts: "Ngưỡng đạt" kèm "từ x/y", "Cách nộp", "Giới hạn file", "Định dạng".
- Ghi chú hạn nộp. Thẻ "Đề bài" dùng `MarkdownEditor` có sẵn với nhãn "Đề bài (Markdown)", nút "Lưu đề bài" và chặn rời trang khi chưa lưu.
- Bảng tiêu chí có dòng "Tổng".
- File kèm: tải lên với kind `document`, có thanh tiến độ; chip tải xuống trỏ `/api/v1/media/{id}/content`; nút "Gỡ file …".
- File sai đuôi bị chặn ngay ở client, lỗi gắn vào nút "Tải file lên" qua `aria-describedby`.

Trang chặng:

- Dòng quiz và homework có meta theo loại và link tới trang soạn ("Soạn câu hỏi: …" / "Soạn tiêu chí: …"; bản đã phát hành là "Xem chi tiết: …").
- Học liệu thiếu nội dung có badge "Chưa đủ để phát hành".
- Khối "Chưa phát hành được v{n}:" thêm các lý do của quiz và homework, khớp lời và thứ tự của `publishBlocker` phía server.

Bản đã phát hành mở trang soạn ở chế độ chỉ đọc: không còn nút sửa, ô nhập hay cột "Thao tác".

`upload-media` đã chuyển sang `src/shared/api/upload-media.ts`, kind gồm `video | image | document`. `putToStorage` gửi đúng `headers` trong upload ticket thay vì `file.type`.

Giảng viên mở đường dẫn trình soạn bị đưa về `/teach`, nhờ role middleware. Hành vi này giờ có test khóa lại.

## File

Trong danh sách Files của phase:

- `src/shared/api/upload-media.ts` (+ `upload-media.test.ts`). Đã xóa `stages/api/upload-media.ts` và `stages/api/media-api.ts`, sửa các chỗ import.
- `src/features/stages/model/`: `schemas.ts` (`quizSchema`, `questionSchema`, `optionSchema`, `homeworkSchema`, `criterionSchema`, `attachmentSchema`), `lesson-form.ts` (`LessonKind` năm giá trị, `toLessonBody`, `toLessonPatch`, `dropsContent`, `isGraded`), `stage-rules.ts` (`lessonPublishBlockers`).
- Mới: `model/question-form.ts`, `model/criterion-form.ts`, mỗi file kèm test.
- `api/stages-api.ts` (câu hỏi, tiêu chí, sắp xếp). `api/msw-handlers.ts`: handler câu hỏi và tiêu chí, PATCH gộp cấu hình quiz và homework, ticket `document`.
- Mới: `hooks/use-question-mutations.ts`, `hooks/use-criterion-mutations.ts` (kèm `useHomeworkPatch`).
- Mới: `components/quiz-settings-card.tsx`, `question-list.tsx`, `question-form-dialog.tsx`, `homework-brief-card.tsx`, `criteria-table.tsx`, `criterion-form-dialog.tsx`, `attachment-list.tsx`.
- `components/lesson-form-dialog.tsx`, `components/lesson-list.tsx`, `pages/lesson-editor-page.tsx`.
- `src/shared/ui/type-icon.tsx`: icon lucide `CircleHelp` cho quiz, `ClipboardList` cho homework.
- Mới: `e2e/quiz-homework-authoring.spec.ts`. `e2e/playwright.config.ts`: thêm `quiz-homework-authoring` vào regex `E2E_SPECS`.

Ngoài danh sách Files (giữ tối thiểu):

- Mới: `components/graded-lesson-editor.tsx` (+ test). Đây là phần soạn quiz/homework tách khỏi trang, theo mục Rủi ro của phase.
- Mới: `components/facts.tsx`, `components/violation-list.tsx`, dùng chung cho thẻ quiz, homework và trang soạn.
- Mới: `model/attachment-file.ts` (+ test): allowlist đuôi file và giới hạn dung lượng của document.
- Mới: `test/markdown-editor-double.tsx`: bản thay `MarkdownEditor` cho test jsdom, dùng chung cho `lesson-editor-page.test.tsx` và `graded-lesson-editor.test.tsx`.
- `components/unit-section.tsx` (link trang soạn, meta, badge) và `pages/stage-detail-page.tsx`, vì danh sách học liệu giờ nằm trong `unit-section`.
- `pages/lesson-editor-page.test.tsx` (dùng double chung), `pages/stage-detail-page.test.tsx`.
- `src/app/middleware.test.tsx`: test cho FR-74.
- `components/markdown-editor/markdown-editor.tsx` (+ test): prop tùy chọn `label` cho tên truy cập của vùng soạn. Mặc định vẫn là "Nội dung bài học" (lấy từ `editor-translation.ts`), nên trang soạn markdown không đổi. `homework-brief-card.tsx` truyền "Đề bài (Markdown)". Double `test/markdown-editor-double.tsx` cũng nhận prop này.
- Feature `reports` và `dashboard` (theo yêu cầu của team-lead, để Vitest toàn bộ xanh với golden mới):
  - `reports/model/schemas.ts`: enum `type` của học liệu trong member report mở rộng thành `video | markdown | quiz | homework`. Không thêm `.strict()`.
  - `reports/model/schemas.test.ts`: thêm hai test. Golden member report có đủ bốn loại học liệu; loại lạ (`survey`) bị từ chối.
  - `reports/components/report-panel.test.tsx`: số theo golden mới (Cường 56%, An 11/25, ba học liệu "Không bắt buộc"). Thêm assert icon "Quiz" và "Bài tập" trong drawer. Không học viên nào trong seed xong trọn một chặng, nên test highlight `text-ok` dùng bản golden có Database của Cường đặt 100% (có comment).
  - `dashboard/pages/dashboard-page.test.tsx`: basic01 "24% hoàn thành" (golden `dashboard.json`).

## Test

Vitest, phần của phase này:

- `model/question-form.test.ts` (10), `model/criterion-form.test.ts` (4), `model/attachment-file.test.ts` (4).
- `model/lesson-form.test.ts`, `model/schemas.test.ts` và `model/stage-rules.test.ts`: thêm case quiz và homework.
- `shared/api/upload-media.test.ts` (4): ticket `document`; file `.sql` có `file.type` rỗng vẫn PUT với `Content-Type` lấy từ ticket.
- `components/graded-lesson-editor.test.tsx` (21):
  - facts; validate câu hỏi (quá ít phương án, chưa có đáp án đúng, câu một đáp án có hai đáp án đúng);
  - body POST chính xác; lỗi đọc qua accessible description; sửa câu giữ id phương án; xem trước;
  - sắp xếp và xóa; lý do chặn khi có câu chưa đánh dấu đáp án đúng; "Sửa thiết lập";
  - đổi sang markdown; đề bài, tiêu chí, upload pdf (ticket, PUT, PATCH `attachmentMediaIds`), chặn csv, gỡ file;
  - chế độ chỉ đọc của cả hai loại.
- `pages/stage-detail-page.test.tsx` (59, thêm describe "quiz and homework"): năm nhãn loại; tạo quiz bài tập, quiz luyện tập và homework; link trang soạn và meta; badge và lý do chặn; bản đã phát hành; đổi giữa hai loại quiz không gửi `type`; đổi quiz sang homework phải xác nhận.
- `app/middleware.test.tsx`: giảng viên mở `/admin/stages/…/lessons/:id/edit` bị đưa về `/teach`.
- `components/markdown-editor/markdown-editor.test.tsx`: vùng soạn lấy tên từ `label` được truyền, và quay về "Nội dung bài học" khi bỏ `label`. `graded-lesson-editor.test.tsx` tìm đề bài theo "Đề bài (Markdown)".

Playwright `e2e/quiz-homework-authoring.spec.ts`, chạy @serial trên chặng E2EQH:

1. Admin tạo quiz bài tập (số lượt 3) và homework. Trang chặng hiện ba lý do chặn, nút phát hành bị khóa, hai badge. Đối chiếu `quiz_settings` trong DB.
2. Soạn hai câu: một câu một đáp án và một câu nhiều đáp án (2 điểm). Câu đầu thử lưu khi chưa có đáp án đúng thì bị chặn. Đối chiếu `questions` và `question_options`.
3. Soạn đề bài, hai tiêu chí (60 + 40) và một file kèm pdf qua storage thật. Đối chiếu `homework_settings`, `rubric_criteria`, `homework_attachments` và `media_files` (`document`, `application/pdf`, `ready`).
4. Câu thiếu đáp án đúng chặn phát hành. Sửa "câu 2", phát hành thành công, và trang soạn của bản đã phát hành ở chế độ chỉ đọc.
5. Giảng viên mở URL trình soạn thì về `/teach`.

## Lệnh và kết quả

Chạy trong `apps/web`:

| Lệnh | Kết quả |
|------|---------|
| `corepack pnpm lint` | sạch |
| `corepack pnpm typecheck` (`tsc -b --noEmit`, gồm cả `e2e/**`) | sạch |
| `corepack pnpm lint:design` | `design-audit: 0 violations` |
| `corepack pnpm test` | 65 file, 675 test, tất cả pass |
| `grep` import `stages/api/upload-media` | không còn chỗ nào |

**E2E chưa chạy.** Controller chạy (sau `make e2e-seed-reset`): `corepack pnpm -C apps/web test:e2e quiz-homework-authoring lesson-editor units reports` (đối số là bộ lọc file của Playwright).

Lần chạy trước, `corepack pnpm test` fail 5 file `reports`/`dashboard` (`ZodError: Invalid option: expected one of "markdown"|"video"`), vì golden reports mới có học liệu quiz/homework. Đã sửa như ghi ở mục File.

## Sai khác so với plan và prototype

- Quiz luyện tập không có ngưỡng đạt (server và CHECK của `quiz_settings` cấm), nên form và facts ẩn "Ngưỡng đạt" khi chọn quiz học liệu.
- Đề bài homework được soạn trên trang soạn bằng `MarkdownEditor`, không nằm trong dialog học liệu. Dialog chỉ có ngưỡng đạt.
- Thanh tiến độ tải file dùng `Progress` (aria "Đã tải lên n%"), giống form video hiện có. Repo không có component `progress-bar`.
- Lý do "có n câu chưa đánh dấu đáp án đúng" không tạo được qua API, vì server từ chối câu không có đáp án đúng.
  - Vitest phủ lý do này bằng fixture.
  - E2E dựng trạng thái đó bằng một câu `UPDATE question_options` trực tiếp trong DB (có comment giải thích), rồi kiểm lý do chặn và luồng sửa qua giao diện.
  - E2E cũng kiểm các lý do tạo được thật: "chưa có câu hỏi", "chưa có đề bài", "chưa có tiêu chí chấm".
- Bỏ badge "đề xuất" của prototype, vì spec không có dữ liệu cho nó.
- Nhãn nút sắp xếp là "Chuyển câu n lên/xuống" và "Chuyển tiêu chí n lên/xuống", để mỗi nút có tên riêng.
- Breadcrumb trang soạn dùng mã chặng, giống trang soạn markdown hiện có.
- MSW: handler PATCH học liệu gộp cấu hình quiz và homework; POST câu hỏi và tiêu chí trả 201.

## Việc thêm: cập nhật `e2e/reports.spec.ts` theo seed mới

Seed backend giờ có hạn nộp, bài nộp và điểm, nên % tiến độ của lớp basic01 thay đổi. Tôi lấy số từ golden `apps/api/internal/features/reports/testdata/class-report.json` và `member-report.json`, không chép từ tin nhắn.

- Hai golden này được sinh bởi `repository_pg_test.go` trên DB đã chạy `seed.Run(..., seed.Options{})`. `make e2e-seed-reset` chạy cùng seed đó.
- Tôi chỉ đọc `apps/api`, không sửa.

% toàn khóa của từng học viên (đủ / tổng bắt buộc):

| Học viên | % |
|----------|---|
| Cường | 56 (14/25) |
| An | 44 (11/25) |
| Bích | 28 (7/25) |
| Hà | 16 (4/25) |
| Phong | 16 (4/25) |
| Khang | 8 (2/25) |
| Dũng | 0 (0/25) |

Golden ghi `belowCount: 6` ở ngưỡng 50 (so sánh `<`). Con số này khớp với danh sách sáu học viên dưới 50% bên dưới.

Các chỗ đã sửa trong spec:

- Thêm hằng `BICH`.
- `below=30`: thêm Bích, thành [Bích, Dũng, Hà, Khang, Phong].
- `below=50`: thêm An và Bích, thành [An, Bích, Dũng, Hà, Khang, Phong].
- Comment học viên bằng %: "Hà, Phong cùng 16%".
- Sheet của An: "44% hoàn thành" (thay "71%").

Các assert giữ nguyên vì golden vẫn đúng với chúng:

- Thứ tự sắp xếp: Cường đầu `pct-desc`, Dũng đầu `pct`.
- `notlogged`, `dropped`.
- Heading "1. Database" và "5. …" trong sheet. Golden vẫn có năm chặng, chặng đầu là Database.

Kiểm tra: `corepack pnpm typecheck` sạch, `eslint e2e/reports.spec.ts` sạch. Spec này **chưa chạy**.

Schema `reports` phía web đã nhận học liệu quiz/homework, nên trang báo cáo không còn lỗi parse golden mới. `e2e/reports.spec.ts` đã lệch prettier từ trước; tôi không format lại để diff gọn. Tương tự, `reports/model/schemas.ts` và `report-panel.test.tsx` chỉ sửa đúng các dòng cần, không format lại cả file.

## Câu hỏi mở

Không còn. Hai câu trước đã xử lý:

1. Tên truy cập của vùng soạn đề bài: `MarkdownEditor` nhận `label`, trang homework truyền "Đề bài (Markdown)". E2E `quiz-homework-authoring.spec.ts` tìm theo tên mới; `lesson-editor.spec.ts` giữ "Nội dung bài học".
2. Schema `reports` phía web: phase này đã sửa theo yêu cầu của team-lead (xem mục File).
