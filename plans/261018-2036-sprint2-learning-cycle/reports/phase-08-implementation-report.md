# Báo cáo thực hiện: homework và chấm bài (web)

- Phase: `phase-08-homework-grading-web.md` (FR-61 đến FR-64, phía web)
- Ngày: 2026-10-19
- Trạng thái: xong, kèm lưu ý. Lint, typecheck, test và lint:design đều xanh. Spec Playwright đã viết nhưng **chưa chạy**, vì stack e2e (:8081) còn image cũ và controller chưa build lại. Không commit.

## Kết quả

Phase làm đủ bốn màn hình.

**Trang homework của học viên** nằm trong `features/learning`:

- Đề bài và form nộp gồm repo, commit và tối đa 5 file. File đi qua `POST /me/uploads` rồi `/complete`.
- Help của repo theo OQ-1: "Repo phải để public để giảng viên mở được."
- Kết quả lần chấm mới nhất có điểm từng tiêu chí.
- Khi khóa nộp, trang hiện lý do: đã Đạt, hoặc lý do server trả về.
- Lịch sử nộp có nhãn "Đã thay bằng lần nộp sau".

**Hàng chờ chấm** ở `/teach/grading` và `/admin/grading`:

- Câu đếm tính trên toàn hàng chờ, cùng badge "N mục quá 3 ngày".
- Bộ lọc gồm loại, lớp, bài và "chỉ quá 3 ngày". Bộ lọc ghi lên URL và đổi thành tham số API.
- Có danh sách "Đã chấm và xác nhận gần đây".
- Trạng thái trống và lỗi có nút thử lại.

**Trang chấm** ở `grading/:submissionId`:

- Facts, đề bài và lịch sử chấm, gồm điểm từng tiêu chí của mỗi lần.
- Form chấm có tổng điểm trực tiếp (`aria-live`) và ngưỡng Đạt/Cần làm lại.
- Lần chấm đầu chuyển sang `nextQueueItem` với toast "Chuyển sang bài tiếp theo.", hoặc về hàng chờ với "Đã chấm hết hàng chờ.". Sửa điểm thì ở lại trang.
- Lỗi 409 hiện alert. Lỗi 422 gắn vào đúng trường.
- 403/404 hiện toast và quay về hàng chờ.
- Bản nộp cũ bị khóa, kèm link tới bản mới.

**Tab "Bài tập" của lớp**:

- Có cả ở trang lớp của Admin và của giảng viên. Tab mặc định của giảng viên vẫn là `report`.
- Mỗi homework là một thẻ gồm hạn nộp, thống kê và bảng học viên.
- Dialog đặt, đổi hoặc bỏ hạn nộp dùng `datetime-local` theo giờ Việt Nam và gửi ISO có `+07:00`.
- Lớp đã kết thúc: nút có `aria-disabled` và tooltip lý do.

**Thay đổi chung:**

- NAV thêm "Chấm bài" cho giảng viên và Admin, không có badge.
- Router thêm các route grading của hai khu vực.
- Ghi chú tiến độ của báo cáo đổi theo prototype: "Video và bài đọc do học viên tự xác nhận; quiz và bài tập tính theo kết quả".

## File

**Mới:**

- `apps/web/src/features/grading/`:
  - `index.ts` và `routes.tsx`;
  - `api/`, `hooks/`;
  - `model/`: `schemas`, `grade-form`, `queue-filter`, `links`, `wait`, `deadline-time`, `deadline-form`, cùng test;
  - `components/`: `homework-tab`, `deadline-dialog`, `grade-form`, `grade-history`, `queue-filter-bar`, `queue-table`, `recent-list`, `submission-facts`;
  - `pages/`: `grading-page` và `grade-submission-page`, cùng test;
  - `test/`: `golden.ts` và `render-grading.tsx`.
- `apps/web/src/app/grading-routes.test.tsx`.
- `apps/web/e2e/homework.spec.ts`.

**Sửa:**

- `app/router.tsx`, `shared/layout/nav.tsx`, `app/app-shell.test.tsx`.
- `classes/model/class-tabs.ts` và test, `classes/pages/class-detail-page.tsx` và test.
- `reports/model/teach-class-tabs.ts` (mới) và test, `reports/pages/teach-class-page.tsx` và test.
- `reports/model/schemas.ts` và test, `reports/components/report-panel.tsx` và test, `reports/components/student-detail-drawer.tsx`.
- Trang homework của học viên trong `features/learning`.
- `e2e/a11y.spec.ts`, `e2e/playwright.config.ts` (thêm `homework` vào `E2E_SPECS`), `e2e/reports.spec.ts` (câu ghi chú tiến độ).

**Sửa nhỏ ngoài danh sách file:**

- `shared/ui`: `badge` (`SubmissionBadge`), `tab-bar`, `file-chip`.
- `shared/lib/format`, `shared/api/upload-media`, `shared/domain` (homework: `shortTitle`, `GRADING_SLA_DAYS`).
- `learning/pages/homework-lesson.test.tsx`: sửa lỗi type `RequestHandler[]`.
- `dashboard/pages/dashboard-page.test.tsx`: `% hoàn thành` giờ lấy từ fixture. Golden `dashboard.json` vừa được backend delegate sinh lại nên giá trị cứng 24% đã sai.
- `reports/components/report-panel.test.tsx`: phần trăm của Cường, câu "x/y học liệu bắt buộc" và số "Không bắt buộc" giờ suy từ fixture, cũng vì golden báo cáo vừa đổi.

## Kiểm chứng

Lệnh: `cd apps/web && corepack pnpm lint && corepack pnpm typecheck && corepack pnpm test && corepack pnpm lint:design`

- lint và typecheck sạch.
- vitest: 79 file, 801 test đều qua.
- design-audit: 0 vi phạm.

Ở lần chạy đầu, `learning/pages/lesson-page.test.tsx` ("shows a markdown lesson…") fail một lần trong suite đầy đủ. Test này qua khi chạy riêng và qua ở lần chạy đầy đủ thứ hai, nên có vẻ là timeout khi máy tải nặng. Tôi không sửa test này.

Test Vitest mới bao phủ:

- schema (bất biến trên golden hàng chờ, không dùng số cứng);
- validate form nộp bài và form hạn nộp;
- rubric, tổng điểm và ngưỡng của form chấm;
- bộ lọc hàng chờ, cả URL lẫn API;
- routing và role guard: giảng viên mở `/admin/grading` bị đưa về `/teach`, học viên về `/learn`, khách về `/login?next=…`;
- tab "Bài tập" trên trang lớp của Admin và của giảng viên, với link chấm đúng khu vực.

**Playwright: chưa chạy.**

- `homework.spec.ts` (@serial, seed lại ở beforeAll/afterAll) đi trọn luồng của bước 6:
  - An nộp repo + README.md;
  - An nộp lại lần #2 kèm ảnh khi đang chờ chấm, bản #1 thành "Đã thay bằng lần nộp sau" (kiểm cả DB);
  - Hương đặt hạn nộp ở tab "Bài tập" (kiểm `due_at` trong DB);
  - tải file nộp, phải ra `content-disposition: attachment`;
  - Hương chấm 50/100 Cần làm lại từ hàng chờ;
  - An nộp lại lần #3;
  - Hương chấm 90/100 Đạt;
  - An thấy điểm từng tiêu chí và không còn nút nộp;
  - Mailpit có đúng hai thư "Bài đã được chấm".
- `a11y.spec.ts` thêm test axe cho ba route: `/teach/grading`, trang homework chưa nộp (go-hw) và trang homework đã chấm Đạt (ds-hw). Phần chạy axe được tách thành helper dùng chung với test cũ.

## Lệch so với plan hoặc prototype

- Help của hạn nộp ghi "Theo giờ Việt Nam (GMT+7)…". Bỏ câu nhắc email nhắc hạn, vì sprint này không có email đó.
- Bảng học viên trong tab Bài tập không có cột email: API lớp không trả email.
- Hàng "Quá 3 ngày chưa chấm" trong tab Bài tập dùng `waitingDays >= 3` của API, theo đồng hồ server. Hàng chờ dùng `overdue` và `waitedHours` của API. "Quá hạn" chưa nộp được suy ra ở client từ `deadline`.
- Mục quiz trong hàng chờ và danh sách gần đây trỏ tới `grading/quiz/:id`. Route này chưa có và sẽ 404 cho tới phase xem lượt quiz.
- Dòng trong hàng chờ không bấm được cả dòng. Chỉ link thao tác ("Chấm"/"Xác nhận") là điểm bấm, để tránh lồng tương tác.
- Thẻ backlog trên dashboard Admin không nằm trong phase này.
- Câu ghi chú trong drawer học viên dùng "tự xác nhận" thay cho "tự tích" của prototype, cho khớp với badge ở bảng.
- Golden JSON của backend (grading, reports, dashboard) đổi trong lúc làm, nên các test web liên quan đã chuyển sang suy giá trị từ fixture.

## Câu hỏi còn mở

- Cần build lại stack e2e để chạy `E2E_SPECS='homework|a11y'`. Spec cần backend có các endpoint homework, grading, deadline và mailer "Bài đã được chấm".
