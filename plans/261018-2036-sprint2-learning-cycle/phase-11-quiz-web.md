---
phase: 11
title: "Quiz: web"
status: completed
priority: P0
effort: "3d"
dependencies: [8, 10]
---

# Phase 11: Quiz (web)

## Mục tiêu

Phase này làm ba phần:

- Học viên: màn bắt đầu quiz; chế độ tập trung làm từng câu với bản đồ câu, đồng hồ, tự lưu và xác nhận nộp; trang kết quả `?result=`.
- Giảng viên và Admin: trang xác nhận hoặc nhận xét lượt từ hàng chờ, và tab "Quiz" của lớp.
- Hàng chờ của phase 08 hiện thêm dòng quiz bài tập.

## Context

- Spec FR-71, FR-75, FR-76, FR-77.
- Prototype ([scout-prototype](reports/scout-prototype.md) §3, §5, §6): `quizView` 1605-1669, `startQuizTimer` 1670-1680, `quizGoto` 1683-1693, xác nhận nộp 1821-1823, `pageReviewAttempt` 1476-1521, `renderQuizTab` 1294-1316.
- API và golden của phase 09 và 10.

## Files

- Học viên (`src/features/learning/`):
  - `api/quiz-api.ts` và `msw-handlers.ts`.
  - `model/quiz-schemas.ts`: schema lượt mở **không có** trường `isCorrect`. Không dùng `.strict()`: zod mặc định bỏ trường lạ, nên schema strict chỉ làm vỡ UI khi server thêm trường vô hại. Việc chặn lộ đáp án nằm ở server (allow-list DTO của phase 09) và ở kiểm tra response trong e2e.
  - `model/quiz-session.ts` cùng test, gồm các hàm thuần:
    - `resumeIndex(questions, storedIndex)`: câu đang xem nếu còn trong `sessionStorage`, ngược lại câu chưa trả lời đầu tiên (FR-77).
    - `remaining(deadlineAt, serverNow, clientNow)`: tính theo độ lệch đồng hồ.
    - `warningsDue(remainingMs, fired)`: các mốc cảnh báo 5 phút và 1 phút.
    - `unansweredCount`.
  - `hooks/use-quiz-attempt.ts`: start, get, save answer có hàng đợi tuần tự theo câu, submit.
  - `hooks/use-countdown.ts`.
  - `components/quiz-start.tsx`: facts, gợi ý, nút "Bắt đầu làm bài", "Làm lượt mới" hoặc "Làm lại", lý do bị chặn, "Các lượt đã nộp".
  - `components/quiz-runner.tsx`: thanh trạng thái `role="timer"`, "Câu x/n", "a/n đã trả lời", trạng thái lưu `aria-live`, và nút "Nộp bài".
  - `components/quiz-question.tsx`: fieldset với radio (`single`) hoặc checkbox (`multi`).
  - `components/quiz-map.tsx`: `nav aria-label="Chuyển nhanh đến câu hỏi"`, nút ≥ 44px.
  - `components/quiz-submit-dialog.tsx`.
  - `components/quiz-result.tsx`: practice và assessment là hai nhánh riêng; hộp nhận xét của giảng viên.
  - Sửa `lesson-viewer.tsx` và `pages/lesson-page.tsx`: chế độ tập trung ẩn danh sách bên (`is-focus`) khi có lượt mở; đọc `?result=`.
- Chấm (`src/features/grading/`):
  - `pages/review-attempt-page.tsx`.
  - `components/attempt-facts.tsx`, `attempt-answers.tsx` (ghi chú từng câu), `review-form.tsx`, `review-history.tsx`.
  - `components/quiz-tab.tsx` (export `QuizTab` qua `index.ts`, cùng lý do như `HomeworkTab` ở phase 08).
  - `queue-table.tsx`: dòng quiz "Lượt #n · x% · tự nộp khi hết giờ", nhãn loại và nút "Xác nhận".
- Lớp: `classes/model/class-tabs.ts` và `reports/model/teach-class-tabs.ts` thêm `quiz` (label "Quiz"), render qua `TabBar` dùng chung của phase 08. Mặc định tab không đổi.
- App: `src/app/router.tsx` thêm `/admin/grading/quiz/:attemptId` và `/teach/grading/quiz/:attemptId`.
- Thêm `e2e/quiz.spec.ts`.

## Các bước

1. Viết schema và các hàm thuần cùng test trước. Test `remaining` khi đồng hồ máy lệch ±2 phút.
2. Màn bắt đầu: copy nguyên văn prototype cho facts, gợi ý theo chế độ, "Bạn đã đạt quiz bài tập này với x%.", và "Bạn đã dùng hết N lượt". Nút bị chặn dùng `aria-disabled` kèm lý do hiện được.
3. Chế độ tập trung:
   - Mỗi màn một câu. "Câu trước" và "Câu tiếp"; ở câu cuối, nút tiếp thành "Nộp bài". Nút "Nộp bài" trên thanh trạng thái luôn có.
   - Chuyển câu đưa focus tới heading của câu và đọc "Câu x trên n" qua vùng `aria-live`. Cuộn bằng `behavior: "auto"` khi `prefers-reduced-motion`.
   - Tự lưu khi chọn, hiện "Đã lưu hh:mm". Lỗi lưu hiện thông báo và giữ lựa chọn để thử lại. Hết giờ phía server (409) thì chuyển sang kết quả.
   - Thông báo assessment: "Lượt {no}/{lim}. Hết giờ, hệ thống tự nộp với các câu đã lưu. Rời trang không dừng đồng hồ."
   - Đồng hồ cảnh báo ở 5 phút và 1 phút ("Còn n phút. Câu trả lời đã chọn được lưu tự động."). Ở 0, client gọi submit một lần; nếu server đã tự nộp thì chỉ chuyển sang kết quả với thông báo "Hết giờ: hệ thống đã tự nộp bài với các câu đã lưu.".
   - Lưu chỉ số câu đang xem vào `sessionStorage` theo `attemptId` và bọc trong `try/catch`.
4. Hộp "Nộp bài?" nêu số câu chưa trả lời với copy prototype. Sau khi nộp, điều hướng tới `?result=<attemptId>`.
5. Trang kết quả:
   - Practice: từng câu "Đúng · x điểm" / "Sai · 0 điểm" / "Chưa trả lời · 0 điểm", badge "Bạn chọn", khối "Giải thích." và "Ghi chú của giảng viên.".
   - Assessment: điểm, badge "Đạt" hoặc "Chưa đạt · cần N%", câu "Quiz bài tập chỉ hiện điểm và kết quả Đạt, không hiện đáp án đề xuất", hộp "Chờ giảng viên xác nhận." khi `pending`, và nhận xét "Giảng viên đã xác nhận · {tên}" kèm "Câu i. {nội dung}" và ghi chú.
   - Cảnh báo tự nộp; dòng "Kết quả tính vào tiến độ là lượt tốt nhất: x%. Còn n lượt."
6. Trang xác nhận:
   - Facts; "Bài làm từng câu" với "(đáp án đúng)" và "(học viên chọn)"; ô "Ghi chú cho câu i (không bắt buộc)".
   - Form với tiêu đề "Xác nhận kết quả", "Nhận xét cho học viên" hoặc "Sửa nhận xét", và nút "Xác nhận", "Gửi nhận xét" hoặc "Lưu nhận xét mới".
   - Lượt `superseded`: fieldset khóa, alert "Học viên đã nộp lượt #n; lượt này không cần xác nhận nữa." và link tới lượt mới nhất.
   - Lần xác nhận đầu đi tới `nextQueueItem` với toast "Chuyển sang mục tiếp theo."; sửa và practice ở lại trang. Nút quay lại là "Về hàng chờ" hoặc "Về thống kê quiz", theo `from` trên URL.
7. Tab "Quiz": mỗi quiz một thẻ với header, thống kê và bảng học viên (cột theo prototype), cùng hành động "Xác nhận", "Xem" hoặc "Xem, nhận xét". Phần thống kê câu hỏi và "Cấp thêm lượt" để lại chỗ cho phase 15.
8. Playwright `quiz.spec.ts`:
   - Học viên làm quiz luyện tập: chọn, tải lại trang, tiếp tục đúng câu đang xem, nộp với 1 câu trống, thấy đáp án.
   - Học viên làm quiz bài tập; kiểm qua `page.on('response')` rằng không response nào trước khi nộp chứa `isCorrect`.
   - Đồng hồ: `page.clock` chỉ tua đồng hồ trình duyệt, không tua server. E2E chỉ kiểm phía client: cảnh báo 5 phút và 1 phút, và khi hết giờ client gọi nộp và hiện kết quả. Tự nộp phía server (lazy close và `CloseExpired`) được test integration ở phase 09.
   - Hộp "Nộp bài?" hiện đúng số câu chưa trả lời (FR-77).
   - Giảng viên xác nhận từ hàng chờ; học viên thấy nhận xét, không thấy đáp án; Mailpit có email.
   - Axe trên màn làm bài và trang xác nhận.

## Kiểm chứng

```bash
pnpm -C apps/web lint typecheck test lint:design
E2E_SPECS='quiz|homework|a11y' pnpm -C apps/web test:e2e
```

## Tiêu chí hoàn thành

- [x] FR-77 đủ: một câu mỗi màn, "Câu x/n", số câu đã trả lời, trạng thái lưu, đồng hồ, bản đồ ≥ 44px, ẩn danh sách bên, tiếp tục đúng câu, focus và thông báo, giảm chuyển động, xác nhận nộp.
- [x] FR-75 và FR-76 phía UI có test.
- [x] Không component nào của học viên đọc trường đúng/sai trước khi có kết quả; e2e khẳng định không response nào trước khi nộp chứa `isCorrect`. (Schema lượt mở không có `isCorrect`, có test. `quiz.spec.ts` chạy xanh 10/10 trên stack mới; đề bài đến từ response POST bắt đầu lượt, client đưa thẳng vào cache nên không có GET lượt trước khi nộp.)

## Rủi ro

- Lưu đáp án dồn dập khi học viên đổi lựa chọn nhanh: hàng đợi tuần tự theo câu, chỉ gửi giá trị mới nhất.
- Hết giờ đúng lúc đang lưu: server là bên quyết định. Client xử lý 409 bằng cách mở trang kết quả.
