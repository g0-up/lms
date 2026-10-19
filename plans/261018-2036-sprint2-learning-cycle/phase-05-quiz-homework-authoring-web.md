---
phase: 5
title: "Soạn quiz và homework: web"
status: completed
priority: P0
effort: "2.5d"
dependencies: [3, 4]
---

# Phase 05: Soạn quiz và homework (web)

## Mục tiêu

Form học liệu có một select "Loại" với năm giá trị. Trang soạn học liệu có:

- thẻ cấu hình quiz, trình soạn câu hỏi;
- thẻ đề bài homework, bảng tiêu chí chấm, tải file kèm.

Giảng viên mở đường dẫn trình soạn bị đưa về `/teach`.

## Context

- Spec FR-60, FR-70, FR-74. Prototype `lessonForm` 1730-1754, `pageLessonEditor` 1094-1136, `questionForm` 1137-1146, `criterionForm` 1147 ([scout-prototype](reports/scout-prototype.md) §2).
- Web: `stages/model/lesson-form.ts`, `components/lesson-form-dialog.tsx`, `pages/lesson-editor-page.tsx`, `api/upload-media.ts` ([scout-web](reports/scout-web.md)).

## Files

- Chuyển `src/features/stages/api/upload-media.ts` sang `src/shared/api/upload-media.ts` để không phải import chéo feature, vì `learning` cũng dùng ở phase 08. Kind nhận `"video" | "image" | "document"`. Sửa các chỗ import và giữ test.
- Sửa `src/features/stages/`:
  - `model/schemas.ts`: `lessonTypeSchema` thêm `quiz` và `homework`; thêm `quizSchema`, `questionSchema`, `homeworkSchema`, `criterionSchema`.
  - `model/lesson-form.ts`: giá trị form `kind` gồm `video | markdown | quiz:practice | quiz:assessment | homework`, có `toLessonBody` và `toLessonPatch`; đổi giữa hai loại quiz giữ câu hỏi.
  - Thêm `model/question-form.ts` và `model/criterion-form.ts`, mỗi file kèm test.
  - `api/stages-api.ts`, `api/msw-handlers.ts`.
  - Thêm `hooks/use-question-mutations.ts` và `hooks/use-criterion-mutations.ts`.
  - Thêm component:
    - `quiz-settings-card.tsx`: facts và ghi chú theo chế độ.
    - `question-list.tsx` và `question-form-dialog.tsx` (tối đa 6 phương án, checkbox "Đúng").
    - `homework-brief-card.tsx`: dùng `markdown-editor` có sẵn cho "Đề bài (Markdown)".
    - `criteria-table.tsx` (có dòng "Tổng"), `criterion-form-dialog.tsx`.
    - `attachment-list.tsx`: tải lên kind `document` và hiện chip tải xuống.
  - `pages/lesson-editor-page.tsx` chọn phần soạn theo loại.
  - `components/lesson-list.tsx`: meta theo loại (số câu, tổng điểm, "N tiêu chí") và badge "Chưa đủ để phát hành".
  - `model/stage-rules.ts`: các lý do chặn phát hành của quiz và homework, khớp server.
- Sửa `src/shared/ui/type-icon.tsx`: icon cho quiz và homework (lucide, không emoji).
- Thêm `e2e/quiz-homework-authoring.spec.ts`.

## Các bước

1. Đổi schema theo golden `stages/testdata/lesson_quiz.json`, `lesson_homework.json` của phase 04.
2. Chuyển `stages/api/upload-media.ts` sang shared (việc nền, [plan.md](plan.md) §4 mục 10). `putToStorage` hiện đặt `Content-Type` bằng `file.type` (`upload-media.ts:35`); đổi để gửi đúng `headers` trong upload ticket, vì server ký theo type chuẩn của nó. Test: file `.sql` có `file.type` rỗng vẫn gửi header từ ticket. Chạy `pnpm lint` để boundary rule xác nhận không còn import chéo.
3. Form học liệu:
   - Select "Loại" với label nguyên văn từ prototype, và help "Đổi giữa hai loại quiz giữ nguyên câu hỏi. Đổi sang loại khác sẽ bỏ nội dung của loại cũ.".
   - Khi đổi sang loại khác lúc đang sửa, hiện `confirm-dialog` trước khi lưu.
   - Trường quiz assessment chỉ hiện khi chọn quiz bài tập: "Ngưỡng đạt (%)", "Số lượt làm bài" (help "Tính lượt tốt nhất"), "Thời gian mỗi lượt (phút)", và checkbox "Đảo thứ tự câu hỏi và đáp án mỗi lượt".
   - Homework: "Ngưỡng đạt (%)" mặc định 70.
4. Trình soạn câu hỏi:
   - Tiêu đề danh sách "Câu hỏi {n} câu · {total} điểm".
   - Mỗi dòng có nút lên, xuống, "Sửa câu i", "Xóa câu i".
   - Lỗi validate dùng copy của prototype và hiện trên field qua RHF + zod.
   - Phương án là input text. Nội dung câu hỏi và giải thích dùng `textarea` markdown, có xem trước qua `/stages/markdown-preview` sẵn có.
5. Homework:
   - Thẻ facts ("Ngưỡng đạt", "Cách nộp", "Giới hạn file", "Định dạng").
   - Ghi chú "Hạn nộp đặt riêng cho từng lớp trong trang lớp, không đặt ở đây."
   - Bảng tiêu chí; tải file kèm có thanh tiến độ, dùng lại `progress-bar`.
6. Chế độ chỉ đọc khi phiên bản là published: ẩn mọi nút sửa, như học liệu hiện tại.
7. Test FR-74 phía web: test router hoặc middleware khẳng định teacher mở `/admin/stages/.../lessons/:id/edit` bị chuyển về `/teach`. MVP đã có hành vi này nhờ role middleware; phase này thêm test để khóa nó lại.
8. Playwright `quiz-homework-authoring.spec.ts`:
   - Admin tạo quiz bài tập có hai câu (một single, một multi) và một homework có hai tiêu chí cùng một file kèm pdf.
   - Thử phát hành khi một câu chưa có đáp án đúng: bị chặn, thấy đúng lý do.
   - Sửa câu đó, phát hành thành công.
   - Teacher mở URL trình soạn, thấy trang `/teach`.

## Kiểm chứng

```bash
pnpm -C apps/web lint typecheck test lint:design
E2E_SPECS='quiz-homework-authoring|lesson-editor|units' pnpm -C apps/web test:e2e
```

## Tiêu chí hoàn thành

- [x] FR-60, FR-70 và FR-74 phía UI có test.
- [x] Không còn import `@/features/stages/api/upload-media` từ ngoài `stages`.
- [x] Mọi field có label. Lỗi được đọc bởi trình đọc màn hình (`aria-describedby`), giống form hiện có.

## Rủi ro

- `lesson-editor-page.tsx` đang chỉ phục vụ markdown và video. Tách từng phần soạn thành component theo loại để file trang không phình quá lớn.
