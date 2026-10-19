# Phase 15: Cấp thêm lượt, thống kê câu hỏi, tổng kết toàn khóa, báo cáo triển khai

- Ngày: 2026-10-19
- Trạng thái: phần code và test đơn vị/tích hợp đã xong và chạy xanh. Bước e2e trong `quiz.spec.ts` đã viết nhưng **chưa chạy** (team-lead sẽ chạy).
- Không commit. Không có migration. Không đụng container.

## Kết quả

- FR-72: giảng viên hoặc admin cấp thêm 1–3 lượt cho một học viên ở một quiz kiểm tra (assessment), lý do bắt buộc.
  - API `POST /classes/:id/members/:mid/grants` với body `{lessonId, extraAttempts, reason}`, trả 201 và bản ghi grant.
  - 422 khi học liệu không phải quiz kiểm tra của lớp (kể cả quiz luyện tập) hoặc form sai (lý do trống sau trim, quá 500 ký tự tính theo rune, số lượt ngoài 1–3).
  - 409 khi lớp không ở trạng thái đang chạy hoặc học viên đã rời lớp; 403 khi giảng viên không phụ trách lớp; 404 khi thành viên không thuộc lớp.
  - Khóa `class_members` `FOR UPDATE` như lúc bắt đầu lượt, nên cấp lượt và bắt đầu lượt của cùng học viên chạy tuần tự.
  - Mỗi lần cấp ghi audit; lịch sử cấp lượt trả về trong `grants` của tab Quiz.
  - Học viên thấy giới hạn lượt mới (`limit` cộng thêm) và câu "đã được cấp thêm"; hết lượt rồi được cấp thì nút "Làm lượt mới" mở lại.
- FR-73: tab Quiz của lớp có khối "Tỉ lệ trả lời đúng từng câu" cho từng quiz.
  - Mỗi câu: "Câu n." + nội dung, thanh % (ẩn dưới 720px, con số vẫn hiện), và badge "Dưới 30%: xem lại câu hỏi" khi tỉ lệ dưới 30%.
  - Chưa có lượt nộp thì hiện "Chưa có lượt nộp nào.".
- FR-81: trang "Tổng kết khóa học" của học viên có bảng "Tổng kết từng chặng" trong thẻ "Toàn khóa".
  - Cột: Chặng, Trạng thái, Còn thiếu, Quiz bài tập, Homework. Thêm dòng "x/y chặng hoàn thành".
  - Không có tổng điểm toàn khóa (test khẳng định không có).

## File

### Backend

- `apps/api/internal/features/quizzes/`:
  - Mới: `grant.go`, `grant_service.go`, `grant_test.go`, `grant_integration_test.go`.
  - Sửa: `repository.go`, `repository_pg.go`, `dto.go`, `handler.go` (route grants), `review_service.go`.
  - Golden: `testdata/class-quizzes.json`, `testdata/question-stats.json`.
- `apps/api/internal/features/summary/`: entity, service, dto thêm phần `course`; test tích hợp; golden `learner-summary.json`.
- `apps/api/internal/platform/audit/`: `actions.go`, `actions_test.go`, `audit.go` (hành động cấp lượt).
- `apps/api/internal/features/reports/model.go` (chỉ sửa bằng Edit: `actionLabels`, `activityPayload`, `summarize`) và `model_test.go`.
- `docs/architecture.md`: mô tả endpoint grants, mã lỗi, audit `quiz.attempts_granted`, `grants[]` và `questionStats` của `GET /classes/{id}/quizzes`.

### Web

- `apps/web/src/features/grading/`:
  - `model/quiz-schemas.ts`: `attemptGrantSchema`, `questionStatsSchema`, trường `grants` / `questionStats`.
  - Mới: `model/grant-form.ts` (+ test): lựa chọn 1–3, schema form (trim, đếm theo code point, tối đa 500), câu toast.
  - `model/quiz-tab.ts` (+ test): `LOW_CORRECT_PERCENT`, `isLowCorrect`, `canGrantAttempts`.
  - `api/grading-api.ts`: `grant(classId, memberId, body)`. Form chặn lỗi phía client trước khi gửi; mọi lỗi từ server (409, 422, 403) hiện ở alert của dialog và dialog giữ mở.
  - `hooks/use-quiz-review.ts`: `useGrantAttempts` (meta silent, invalidate tab Quiz khi thành công).
  - Mới: `components/grant-dialog.tsx`, `components/grant-history.tsx`, `components/question-stats.tsx`.
  - `components/quiz-tab.tsx`: nút "Cấp thêm lượt" từng dòng học viên, khối thống kê câu hỏi và lịch sử cấp lượt.
  - `api/msw-handlers.ts`: handler POST grants cho test.
  - `components/quiz-tab.test.tsx`: test thống kê câu, hiển thị nút, luồng cấp lượt, lỗi 409, lớp đã kết thúc, lịch sử.
- `apps/web/src/features/learning/pages/quiz-lesson.test.tsx`: test học viên được cấp thêm thì mở lại "Làm lượt mới".
- `apps/web/src/features/summary/`:
  - `model/schemas.ts` (+ test): `courseSummarySchema` và các schema điểm.
  - Mới: `model/course-summary.ts` (+ test), `components/course-table.tsx`.
  - `pages/learn-summary-page.tsx` (+ test): dòng số chặng hoàn thành và bảng.
- `apps/web/src/features/classes/pages/class-detail-page.test.tsx`: đổi id `…0014` thành `…0018`. Golden `class-quizzes.json` có thêm trường mới đứng trước trong thứ tự key nên id sinh ra trong fixture bị dịch; assertion giữ nguyên, chỉ đổi id.

### E2E (chưa chạy)

- `apps/web/e2e/quiz.spec.ts`: test cuối chuỗi serial "giảng viên cấp thêm 1 lượt cho học viên đã hết lượt, học viên thấy Làm lượt mới".
  - Giảng viên mở `/teach/classes/:id?tab=quiz`, tìm dòng Lê Văn Cường (seed đã hết lượt ở quiz Go, 2/2), mở dialog, nhập lý do, gửi.
  - Kiểm tra toast, dòng thành 2/3, mục lịch sử và lý do, và truy vấn đọc `quiz_attempt_grants` trả `[{extra_attempts: 1, reason}]`.
  - Cường đăng nhập qua `loginContext(email, seedPassword())`, màn bắt đầu hiện "2/3 đã dùng · đã được cấp thêm · tính lượt tốt nhất" và nút "Làm lượt mới" bấm được.

## Kiểm tra

- Web (`apps/web`): typecheck sạch, eslint sạch, design-audit 0 vi phạm, `corepack pnpm test` 98 file / 1015 test xanh.
- API (`apps/api`): `make test` xanh toàn bộ package; golangci-lint v2.14.0 0 issue; ranh giới import (`go list -deps`) sạch.
- Playwright: chưa chạy theo chỉ đạo.

## Sự cố trong lúc làm

- DB test dùng chung từng ở trạng thái dirty; quyền ép version bị từ chối nên tôi báo lại, không lách. Sau đó tự hết, `make test` chạy xanh.
- `TestReviewRacesSubmit` (quizzes) fail một lần rồi chạy lại xanh; ghi nhận là có thể flaky, chưa sửa.
- Ba test `quiz-tab` ban đầu fail vì một học viên xuất hiện ở nhiều quiz nên có nhiều nút "Cấp thêm lượt cho …"; đã sửa bằng cách tìm nút trong đúng thẻ quiz, không nới assertion.

## Khác với plan / quyết định cần biết

- Backend:
  - Sửa `review_service.go`: tổng quan quiz của lớp có `grants` và `questionStats`, giới hạn lượt ở chi tiết lượt và tab Quiz tính cả lượt được cấp. `Grant` là method của `ReviewService` (`grant_service.go`).
  - `GrantDTO` có thêm `lessonId` để web gắn lịch sử vào đúng quiz.
  - `granted` là số nguyên (tổng lượt được cấp), không phải bool.
  - Điểm homework trong bảng toàn khóa có `total` và `max` null khi bản nộp mới nhất chưa được chấm.
  - Học viên đã rời lớp thì trả 409.
  - `router.go` không đổi; route thêm trong `handler.go` của quizzes.
  - Mẫu số tỉ lệ đúng từng câu tính cả lượt đã bị thay thế (superseded).
  - Test race dùng thứ tự cố định để kết quả xác định.
- Web:
  - Nút cấp lượt hiện cho học viên chưa đạt (chưa làm, chưa đạt, hết lượt), theo prototype; nút nổi bật (outline) khi hết lượt.
  - Lớp không đang chạy: nút `aria-disabled` + `title`, bấm thì toast "Lớp không ở trạng thái đang chạy, không cấp thêm lượt được.".
  - Cột Trạng thái của bảng toàn khóa ghép với `stages` theo `stageVersionId` để hiện "Có bài quá hạn" / "Có bài cần làm lại".
  - Chặng có nhiều quiz hoặc homework thì mỗi dòng điểm có tiền tố tên ngắn của học liệu.
  - Quiz luyện tập cũng hiện thống kê từng câu (như prototype), nhưng không có nút cấp lượt.
  - E2E dùng Cường (đã hết lượt trong seed) thay vì An.

## Câu hỏi mở

- Mẫu số tỉ lệ đúng có nên bỏ lượt đã bị thay thế không? Hiện đang tính cả.
- `TestReviewRacesSubmit` có cần theo dõi thêm nếu fail lại không?
