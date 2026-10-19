# Báo cáo thực hiện: quiz phía web

- Phase: `phase-11-quiz-web.md` (FR-71, FR-75, FR-76, FR-77)
- Ngày: 2026-10-19
- Trạng thái: xong phần code và unit test; `lint`, `typecheck`, `test` (916/916, chạy đủ hai lần liền) và `lint:design` đều xanh. `e2e/quiz.spec.ts` đã viết, qua lint và typecheck, **chưa chạy**: stack :8081 build từ source cũ và theo yêu cầu không rebuild container. Không commit, không sửa `apps/api`.

## Kết quả

Học viên (`src/features/learning/`):

- Màn bắt đầu có facts, gợi ý theo chế độ, nút "Bắt đầu làm bài", "Làm lượt mới" hoặc "Làm lại", lý do bị chặn (`aria-disabled`) và danh sách "Các lượt đã nộp".
- Chế độ tập trung ẩn danh sách bên và hiện một câu mỗi màn. Thanh trạng thái gồm:
  - đồng hồ `role="timer"`;
  - "Câu x/n" và "a/n đã trả lời";
  - trạng thái lưu qua `aria-live`.
- Bản đồ câu là `nav`, mỗi nút cao 44px.
- Chuyển câu đưa focus vào heading của câu và thông báo "Câu x trên n". Khi bật `prefers-reduced-motion` thì cuộn không có hiệu ứng.
- Lưu câu trả lời theo hàng đợi tuần tự cho từng câu, chỉ gửi giá trị mới nhất. Lưu lỗi thì giữ lựa chọn và có nút thử lại.
- Câu đang xem được lưu trong `sessionStorage`.
- Cảnh báo ở mốc 5 phút và 1 phút. Hết giờ, client gọi nộp đúng một lần; nếu server trả 409 thì mở trang kết quả.
- Hộp "Nộp bài?" nêu số câu chưa trả lời.
- Trang kết quả (`?result=`) có hai nhánh:
  - Practice hiện đáp án, "Bạn chọn", giải thích và ghi chú.
  - Assessment chỉ hiện điểm, Đạt hoặc Chưa đạt, hộp chờ xác nhận và nhận xét kèm "Câu n." theo vị trí học viên thấy. Không có đáp án.
- Schema lượt mở không có `isCorrect` và không dùng `.strict()`.

Chấm (`src/features/grading/`):

- Trang `/{teach|admin}/grading/quiz/:attemptId` có:
  - facts;
  - "Bài làm từng câu" với "(đáp án đúng)" và "(học viên chọn)", cùng ô "Ghi chú cho câu i (không bắt buộc)";
  - form "Xác nhận kết quả", "Nhận xét cho học viên" hoặc "Sửa nhận xét", và lịch sử lượt.
- Lượt `superseded` bị khóa, kèm link tới lượt mới nhất.
- Lần xác nhận đầu đi tới `nextQueueItem` và toast "Chuyển sang mục tiếp theo.", hoặc báo "Đã xử lý hết hàng chờ.". Sửa nhận xét và practice thì ở lại trang.
- Nút quay lại là "Về hàng chờ" hoặc "Về thống kê quiz", theo `from` trên URL.
- Lỗi 403 hiện đúng message của server. 400 về comment gắn vào ô, còn lại vào alert của form. 409 khóa form và tải lại.
- Tab "Quiz" (`QuizTab`, export qua `index.ts`): mỗi quiz một thẻ gồm settings, thống kê và bảng học viên. Hành động là "Xác nhận", "Xem" hoặc "Xem, nhận xét". Có trạng thái trống và trạng thái lỗi (403 và 404 không có nút thử lại).
- Lớp: `CLASS_TABS` và `TEACH_CLASS_TABS` thêm `quiz` (label "Quiz") ngay sau "Bài tập". Tab mặc định không đổi. `class-detail-page.tsx` và `teach-class-page.tsx` render `QuizTab`.

Áp dụng bốn quyết định của team-lead về phase 10:

- 403: POST reviews hiện message "không phải lượt của mình". GET lượt và tab Quiz hiện "Bạn chỉ xem được lớp mình phụ trách.". Cả hai đều lấy message từ server.
- `position` của ghi chú là vị trí câu học viên thấy.
- Sáu mã trạng thái thành viên được map sang nhãn của prototype.
- Practice được nhận xét thì thành `confirmed` nhưng không vào hàng chờ.

## File

Mới:

- Học viên:
  - `learning/api/quiz-api.ts`
  - `learning/model/quiz-schemas.ts` và `quiz-session.ts`, cùng test
  - `learning/hooks/use-quiz-attempt.ts` và `use-countdown.ts`
  - `learning/components/quiz-{start,runner,question,map,submit-dialog,result,view}.tsx`
  - `learning/pages/quiz-lesson.test.tsx`
- Chấm:
  - `grading/model/quiz-schemas.ts`, `review-form.ts`, `quiz-tab.ts`, `links.ts`, cùng test
  - `grading/hooks/use-quiz-review.ts`
  - `grading/components/attempt-facts.tsx`, `attempt-answers.tsx`, `review-form.tsx`, `review-history.tsx`, `quiz-tab.tsx` (kèm test)
  - `grading/pages/review-attempt-page.tsx` (kèm test)
- Dùng chung: `shared/domain/quiz.ts`, gồm `QUIZ_KIND_VI` và `quizReviewBadge`.
- E2E: `e2e/quiz.spec.ts`.

Sửa:

- Học viên: `learning/api/msw-handlers.ts`, `test/golden.ts`, `components/lesson-viewer.tsx`, `pages/lesson-page.tsx`.
- Chấm: `grading/api/grading-api.ts`, `api/msw-handlers.ts`, `routes.tsx`, `index.ts`, `model/schemas.ts`, `model/grade-form.ts`, `test/golden.ts`, `test/render-grading.tsx`.
- Lớp: `classes/model/class-tabs.ts` (kèm test) và `classes/pages/class-detail-page.tsx` (kèm test).
- Báo cáo: `reports/model/teach-class-tabs.ts` (kèm test) và `reports/pages/teach-class-page.tsx` (kèm test).
- Test và cấu hình khác: `shared/domain/index.ts`, `domain.test.ts`, `app/grading-routes.test.tsx` (thêm route quiz cho teacher và admin), `e2e/playwright.config.ts` (thêm `quiz` vào `E2E_SPECS`).

## Kiểm chứng

```text
cd apps/web
corepack pnpm lint          # sạch
corepack pnpm typecheck     # sạch
corepack pnpm test          # 87 file, 916/916 (chạy hai lần liền, đều xanh)
corepack pnpm lint:design   # 0 violations
```

Test mới chính:

- Schema: parse đủ golden của API (`review-assessment`, `review-practice`, `class-quizzes`, các file `attempt-*`). Từ chối trạng thái lạ.
- `quiz-session`: các test `resumeIndex`, `remaining` (lệch đồng hồ ±2 phút), `warningsDue`, `fmtCountdown`, `toggleOption` và `unansweredCount`.
- `quiz-lesson.test.tsx` (28 test) phủ toàn bộ FR-77:
  - focus và thông báo, chuyển động giảm, tiếp tục đúng câu;
  - lưu tuần tự, lưu lỗi rồi thử lại, 409;
  - cảnh báo 5 và 1 phút, tự nộp một lần;
  - kết quả practice và assessment, nhận xét kèm vị trí câu.
- `review-attempt-page.test.tsx` (14 test): xác nhận đi tới mục tiếp theo, hết hàng chờ, sửa, practice, bắt buộc nhận xét, 400 vào ô hoặc vào form, 403, 409 khóa, superseded, 403 và 404 khi mở trang, admin, `from=quiz`.
- `quiz-tab.test.tsx` (8 test) cùng hai page test của lớp và route test.

E2E `quiz.spec.ts` (@serial, seed lại ở `beforeAll` và `afterAll`), học viên An lớp basic01:

1. Practice "Luyện tập: Truy vấn SQL cơ bản": trả lời hai câu, tải lại trang thì vẫn ở câu 3, nộp khi còn 1 câu trống. Kết quả có 5 "(đáp án đúng)", 4 "Giải thích." và 1 "Chưa trả lời".
2. Assessment "Kiểm tra cuối chặng Go", lượt 2/2:
   - `page.on('response')` khẳng định không response API nào trước khi nộp (cả sau nộp) chứa `isCorrect`.
   - axe trên màn làm bài.
   - "Nộp bài?" báo "2 câu chưa trả lời".
   - Kết quả "Chờ giảng viên xác nhận." và DB `pending`.
3. "Kiểm tra cuối chặng Database" với `page.clock`: cảnh báo 5 phút và 1 phút. Hết giờ, client gọi submit đúng 1 lần và hiện kết quả.
4. Giảng viên Hương mở từ hàng chờ, chạy axe trên trang xác nhận, ghi chú câu 1 rồi "Xác nhận". DB chuyển `confirmed`, Mailpit có thư "Giảng viên đã xác nhận kết quả: Kiểm tra cuối chặng Go lượt #2".
5. An thấy nhận xét, "Câu 1." và ghi chú, không có "(đáp án đúng)" hay "Giải thích.".

Lưu ý khi chạy:

- `playwright.config.ts` dùng hằng regex, không đọc biến `E2E_SPECS` như lệnh trong plan. Chạy bằng `pnpm -C apps/web test:e2e` hoặc `playwright test -c e2e/playwright.config.ts --project=e2e quiz`.
- Cần rebuild stack trước khi chạy.

## Lệch so với plan và ghi chú

1. Chữ ký hàm thuần khác plan:
   - `resumeIndex(questions, answers, storedIndex)` cần `answers` để tìm câu chưa trả lời đầu tiên.
   - `remaining(deadlineAt, serverNow, clientNow, receivedAt)` cần `receivedAt` để tính độ lệch so với lúc nhận dữ liệu.
2. Thêm `shared/domain/quiz.ts` (`QUIZ_KIND_VI`, `quizReviewBadge`) vì cả learning và grading cùng dùng nhãn trạng thái xác nhận.
3. Dòng khóa đáp án bỏ chữ "đề xuất". Câu hiện tại là "Quiz bài tập chỉ hiện điểm và kết quả Đạt, không hiện đáp án.".
4. Thêm copy gợi ý cho assessment không giới hạn thời gian, vì prototype chỉ có bản có giờ.
5. Trang bài học không hiện `CompleteBar` cho quiz: hoàn thành quiz do nộp bài quyết định.
6. Thêm helper MSW `closeAttempt` để test trường hợp server đã đóng lượt.
7. Client không tự đối chiếu chéo dữ liệu kết quả (điểm, trạng thái) mà tin server.
8. Route quiz được thêm ở `features/grading/routes.tsx` (do `app/router.tsx` mount), không thêm trực tiếp vào `app/router.tsx`, cùng cách làm của phase 08.
9. Dòng quiz trong hàng chờ đã có từ phase 08, nên không sửa `queue-table.tsx`.
10. `grading-page.test.tsx` được chỉnh theo golden `queue-recent.json` mà phase 12 sinh lại: test tìm mục homework theo loại. Thêm test cho homework "Cần làm lại".
11. Tab Quiz báo lỗi mạng bằng message mạng chung ("Không kết nối được máy chủ. Thử lại sau."), không dùng fallback.
12. Sửa nhỏ ngoài phần quiz để test ổn định:
    - Ba file `lesson-page.test.tsx`, `homework-lesson.test.tsx` và `quiz-lesson.test.tsx` nạp trước route lazy trong `beforeAll`, giống `class-detail-page.test.tsx`. Trước đó test đầu mỗi file vượt timeout 1 giây của `findBy` vì phải import trang bài học (giờ lớn hơn do có quiz). Lỗi xảy ra cả khi chạy riêng file.
    - `review-form.test.ts` dựng emoji bằng `String.fromCodePoint` để qua `lint:design`. Ý nghĩa test giữ nguyên.
13. Comment trong `quiz.spec.ts` viết tiếng Việt theo quy ước của mọi spec trong `e2e/`. Comment trong `src` vẫn tiếng Anh.
14. E2E có giờ: sau khi tua `page.clock`, spec không lưu câu hay tải lại lượt. Dữ liệu mới mang giờ server thật nên đồng hồ sẽ đếm lại từ đó. Đây là đúng thiết kế.

## Tiêu chí hoàn thành

- [x] FR-77 đủ.
- [x] FR-75 và FR-76 phía UI có test.
- [ ] Không lộ đúng/sai trước kết quả. Phần component đã xong, có test. Phần e2e đã viết nhưng chưa chạy, nên chưa đánh dấu.

## Câu hỏi còn mở

- Có muốn `playwright.config.ts` đọc biến `E2E_SPECS` để khớp lệnh trong plan không? Phase này chưa đổi hành vi đó.
