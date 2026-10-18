# Spec Sprint 2 – Chu trình học tập theo chặng

- **Ngày:** 08/10/2026
- **Tác giả:** Nguyễn Văn Thược
- **Trạng thái:** Draft – chờ review
- **Cập nhật 13/10/2026:** hai loại quiz (quiz học liệu, quiz bài tập), chỉ Admin soạn quiz, giảng viên xác nhận và nhận xét lượt quiz, làm quiz từng câu một (FR-74 → FR-77). OQ-8 đã chốt: giảng viên không soạn quiz.
- **Cập nhật 13/10/2026 (2):** thêm cấp **bài học** giữa chặng và học liệu (Chặng → Bài học → Học liệu), mỗi chặng kết thúc bằng bài học "Kiểm tra cuối chặng" (FR-78).
- **Phạm vi:** Luyện tập → Homework → Đánh giá → Tổng kết cho từng chặng
- **Phụ thuộc:** Spec MVP ([`spec-lms-mvp.md`](http://spec-lms-mvp.md)) – Sprint 1

> Quy ước: các con số đánh dấu **[đề xuất]** là giá trị mặc định do người viết spec đưa ra, cần xác nhận trước khi chốt. Mã FR tiếp nối spec MVP (MVP dùng FR-01 → FR-50).

---

## 1. Vấn đề

Sau MVP, tiến độ học viên chỉ dựa vào việc **học viên tự tích "Đã học xong"**. Giảng viên và Admin không biết học viên đã thực sự làm được bài hay chưa. Với khóa lập trình (Database, Golang, React…), học viên học chủ yếu qua luyện tập và làm bài tập, nhưng hệ thống chưa có chỗ để giao bài, nộp bài, chấm và phản hồi. Phần này hiện phải làm thủ công ngoài hệ thống (chat, email, bảng tính), nên không có dữ liệu tập trung để trả lời câu hỏi *"học viên X còn thiếu gì ở chặng nào"*.

Nếu không giải quyết, báo cáo tiến độ MVP chỉ phản ánh mức độ học viên tự khai. Giảng viên phải tổng hợp kết quả bằng tay cho từng lớp, và chất lượng phản hồi phụ thuộc vào từng người.

## 2. Mục tiêu


| #   | Mục tiêu                                                                                | Loại               | Cách đo                                                                          |
| --- | --------------------------------------------------------------------------------------- | ------------------ | -------------------------------------------------------------------------------- |
| G1  | Mọi homework được giao, nộp và chấm trong hệ thống, không qua kênh ngoài                | Vận hành           | % homework của lớp pilot có bài nộp và điểm trong hệ thống: 100%                 |
| G2  | Học viên nhận phản hồi homework kịp thời                                                | Học viên           | Thời gian trung bình từ lúc nộp đến lúc chấm ≤ 3 ngày **\[đề xuất\]**            |
| G3  | Học viên tự kiểm tra kiến thức và nhận kết quả ngay sau mỗi chặng                       | Học viên           | % học viên làm ít nhất 1 lượt luyện tập ở mỗi chặng ≥ 80% **\[đề xuất\]**        |
| G4  | Giảng viên trả lời được "học viên X còn thiếu gì" trên một màn hình, không tổng hợp tay | Vận hành           | Có / không, kiểm chứng ở lớp pilot                                               |
| G5  | Kết quả hoàn thành dựa trên bài làm, không chỉ tự khai                                  | Chất lượng dữ liệu | % học liệu bắt buộc có trạng thái do hệ thống hoặc giảng viên xác định: theo dõi |


## 3. Ngoài phạm vi


| Hạng mục                                           | Lý do                                                                                                |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Chấm code tự động (chạy test trên bài nộp)         | Cần sandbox thực thi code an toàn, quy mô một dự án riêng. Bài code đi qua Homework, giảng viên chấm |
| Điểm tổng toàn khóa có trọng số, xếp loại          | Đã chốt: Tổng kết chỉ cho biết đã hoặc chưa hoàn thành                                               |
| Chặn học chặng sau khi chưa hoàn thành chặng trước | Đã chốt: Tổng kết không chặn                                                                         |
| Ngân hàng câu hỏi dùng chung, rút đề ngẫu nhiên    | Câu hỏi gắn với quiz trong chặng là đủ cho sprint này                                                |
| Câu hỏi tự luận trong quiz                         | Bài tự luận đi qua Homework                                                                          |
| Chống gian lận nâng cao (camera, khóa trình duyệt) | Chi phí cao, giá trị thấp với chương trình miễn phí                                                  |
| Học viên chấm chéo bài nhau                        | Chưa có nhu cầu                                                                                      |
| Giảng viên sửa điểm quiz khi xác nhận              | Điểm quiz do hệ thống chấm; xử lý đáp án sai thuộc OQ-7                                              |
| Quiz riêng hoặc hạn làm quiz theo từng lớp         | Quiz thuộc phiên bản chặng, mọi lớp dùng chung; hạn làm quiz theo lớp ghi ở OQ-9                     |


## 4. Quyết định đã chốt


| Chủ đề                 | Quyết định                                                                                   |
| ---------------------- | -------------------------------------------------------------------------------------------- |
| Hình thức nộp homework | Link repo và file. Học viên nộp một trong hai hoặc cả hai                                    |
| Chấm homework          | Nhiệm vụ bắt buộc của giảng viên. Hệ thống theo dõi thời gian chấm                           |
| Mục đích tổng kết      | Cho biết học viên đã hoặc chưa hoàn thành gì. Không chặn, không tính điểm tổng               |
| Luyện tập              | Quiz trắc nghiệm chấm tự động                                                                |
| Luyện tập và Đánh giá  | Dùng chung một quiz engine, khác cấu hình                                                    |
| Versioning             | Đề bài, câu hỏi, tiêu chí chấm, ngưỡng đạt thuộc phiên bản chặng, bất biến sau khi phát hành |
| Hai loại quiz          | **Quiz học liệu** (chế độ Luyện tập) và **Quiz bài tập** (chế độ Đánh giá). Xem bảng thuật ngữ ở mục 5 |
| Soạn quiz              | Chỉ Admin soạn cả hai loại quiz trong phiên bản chặng nháp và phát hành. Giảng viên không tạo, sửa, xóa hay sắp xếp quiz |
| Chấm quiz              | Hệ thống chấm ngay khi nộp; điểm có hiệu lực ngay, không chờ giảng viên                      |
| Xác nhận quiz bài tập  | Mỗi lượt nộp vào hàng chờ chấm ở trạng thái "Chờ xác nhận". Giảng viên xem từng câu, viết nhận xét và xác nhận. Không chặn hoàn thành |
| Nhận xét quiz học liệu | Giảng viên xem được mọi lượt và nhận xét khi cần. Không vào hàng chờ                         |
| Làm quiz               | Học viên làm từng câu một ở cả hai loại quiz                                                 |
| Bài học                | Cấp nội dung giữa chặng và học liệu: Chặng → Bài học → Học liệu. Mỗi học liệu thuộc đúng một bài học. Bài học hoàn thành khi mọi học liệu bắt buộc trong đó hoàn thành; % chặng và % khóa vẫn tính theo học liệu (FR-78) |


## 5. Mô hình chặng

```
Phiên bản chặng
 ├─ Bài học 1..n (có thứ tự, giữ unit_key qua các phiên bản)
 │   ├─ Học liệu nội dung (video, markdown)   → học viên tự tích
 │   ├─ Quiz học liệu (quiz, chế độ practice)   → hệ thống chấm, giảng viên nhận xét khi cần
 │   ├─ Homework      (đề bài + tiêu chí chấm)  → giảng viên chấm
 │   └─ Quiz bài tập  (quiz, chế độ assessment) → hệ thống chấm, giảng viên xác nhận
 ├─ Bài học cuối "Kiểm tra cuối chặng"          → thường gồm homework và quiz bài tập
 └─ Tổng kết   (không phải học liệu)           → bảng trạng thái + nhận xét giảng viên

```

- `lessons.type` mở rộng thành {`video`, `markdown`, `quiz`, `homework`}. Tổng kết là dữ liệu tổng hợp, không phải học liệu.
- Một chặng có 0 hoặc nhiều học liệu mỗi loại. Admin sắp xếp theo thứ tự chu trình; hệ thống không bắt buộc làm theo thứ tự.
- Học liệu được nhóm thành **bài học** (FR-78). Thứ tự học liệu trong chặng là thứ tự bài học, rồi thứ tự học liệu trong bài học. Trên giao diện, "Bài học" là cấp nhóm; từng video, bài đọc, quiz, homework vẫn gọi là "học liệu".
- Lớp học cùng một phiên bản chặng được chấm theo cùng một chuẩn.

**Thuật ngữ quiz**


| Trên giao diện                                  | Trong spec               | Giá trị kỹ thuật                                                           |
| ----------------------------------------------- | ------------------------ | -------------------------------------------------------------------------- |
| Quiz học liệu                                   | Luyện tập                | `quiz_settings.mode = practice`                                            |
| Quiz bài tập                                    | Đánh giá, bài đánh giá   | `quiz_settings.mode = assessment`                                          |
| Chờ xác nhận / Đã xác nhận / Đã có lượt mới hơn | Trạng thái xác nhận lượt | `quiz_attempts.review_status = pending / confirmed / superseded`           |
| Đã nhận xét                                     | Nhận xét quiz học liệu   | `review_status = confirmed` trên lượt practice; lượt practice không có `pending` |
| Chấm bài                                        | Hàng đợi chấm (FR-64)    | Gồm bản nộp homework chờ chấm và lượt quiz bài tập chờ xác nhận            |


**Định nghĩa "hoàn thành" theo loại học liệu**


| Loại             | Hoàn thành khi                                            | Ai xác định |
| ---------------- | --------------------------------------------------------- | ----------- |
| Video, markdown  | Học viên tích "Đã học xong"                               | Học viên    |
| Quiz học liệu    | Nộp ít nhất 1 lượt **\[đề xuất\]**                        | Hệ thống    |
| Quiz bài tập     | Điểm của lượt cao nhất ≥ ngưỡng đạt. Không chờ xác nhận   | Hệ thống    |
| Homework         | Bài nộp gần nhất được chấm **Đạt** **\[đề xuất – OQ-2\]** | Giảng viên  |


Thay đổi so với MVP: nút "Đã học xong" (FR-32) chỉ áp dụng cho video và markdown. Công thức % tiến độ (FR-33) giữ nguyên, "hoàn thành" theo bảng trên.

---

## 6. User stories

### Giảng viên

1. Là Giảng viên, tôi muốn có danh sách bài đang chờ chấm của mọi lớp mình phụ trách, bài cũ nhất lên đầu, để không bỏ sót bài nào.
2. Là Giảng viên, tôi muốn chấm homework theo từng tiêu chí và viết nhận xét, để học viên biết chính xác mình sai ở đâu.
3. Là Giảng viên, tôi muốn đặt hạn nộp homework cho lớp của mình, để học viên theo kịp lịch lớp.
4. Là Giảng viên, tôi muốn xem học viên nào đã hoặc chưa hoàn thành từng chặng và còn thiếu mục nào, để nhắc đúng người, đúng việc.
5. Là Giảng viên, tôi muốn viết nhận xét tổng kết cho từng học viên ở mỗi chặng, để học viên có đánh giá tổng thể.
6. Là Giảng viên, tôi muốn cấp thêm lượt làm bài đánh giá cho một học viên gặp sự cố, để xử lý ngoại lệ mà không phải sửa nội dung.
7. Là Giảng viên, tôi muốn xem tỉ lệ trả lời đúng của từng câu hỏi trong lớp, để phát hiện câu hỏi có vấn đề hoặc phần kiến thức cả lớp còn yếu.
8. Là Giảng viên, tôi muốn xem học viên trả lời đúng hay sai từng câu ở mỗi lượt quiz bài tập, viết nhận xét chung và ghi chú từng câu rồi xác nhận, để học viên biết vì sao mình sai.
9. Là Giảng viên, tôi muốn xem lượt quiz học liệu của học viên và gửi nhận xét khi cần, mà không phải xử lý chúng trong hàng chờ.

### Học viên

10. Là Học viên, tôi muốn nộp homework bằng link repo hoặc file, để nộp theo cách phù hợp với bài.
11. Là Học viên, tôi muốn nộp lại khi bài bị đánh giá "Cần làm lại", để sửa và hoàn thành bài.
12. Là Học viên, tôi muốn nhận email khi bài được chấm và xem điểm từng tiêu chí cùng nhận xét, để biết cần cải thiện gì.
13. Là Học viên, tôi muốn làm quiz học liệu không giới hạn lần và xem đáp án kèm giải thích, để tự ôn trước khi làm quiz bài tập.
14. Là Học viên, tôi muốn làm quiz bài tập có giới hạn thời gian và biết ngay kết quả, để biết mình đã đạt chặng hay chưa.
15. Là Học viên, tôi muốn làm quiz từng câu một và chuyển nhanh giữa các câu qua bản đồ câu hỏi, để tập trung và không bỏ sót câu.
16. Là Học viên, tôi muốn nhận email và thấy nhận xét của giảng viên ngay trong trang kết quả quiz, để biết cần ôn lại phần nào.
17. Là Học viên, tôi muốn xem tổng kết của mình ở từng chặng, để biết còn thiếu mục nào.
18. Là Học viên, tôi muốn thấy chặng chia thành các bài học, biết mình đang ở bài học nào và đã xong bài học nào, để học theo từng phần nhỏ.

### Admin

19. Là Admin, tôi muốn soạn đề homework kèm tiêu chí chấm, soạn quiz học liệu và quiz bài tập trong phiên bản chặng nháp, để mọi lớp dùng chung chuẩn đánh giá.
20. Là Admin, tôi muốn xem tồn đọng chấm bài theo từng giảng viên, để can thiệp khi có giảng viên chấm trễ.
21. Là Admin, tôi muốn nhóm học liệu thành bài học, sắp xếp bài học và chuyển học liệu giữa các bài học, để biết chính xác mỗi quiz nằm ở đâu trong chặng.

### Trường hợp biên

- Học viên nộp lại khi bài trước còn đang chờ chấm.
- Học viên nộp sau hạn.
- Học viên mất mạng hoặc đóng tab giữa lúc làm bài đánh giá.
- Hết giờ làm bài khi học viên chưa bấm nộp.
- Học viên đã dùng hết lượt làm bài đánh giá.
- Phát hiện đáp án sai trong bài đánh giá khi lớp đang chạy.
- Giảng viên sửa điểm sau khi đã chấm.
- Học viên nộp lượt quiz bài tập mới khi lượt trước còn chờ xác nhận.
- Giảng viên sửa nhận xét quiz sau khi đã xác nhận.
- Giảng viên mở đường dẫn trình soạn quiz.
- Admin xóa bài học còn học liệu.
- Admin phát hành chặng có bài học chưa có học liệu.
- Link repo hợp lệ nhưng giảng viên không truy cập được (repo private).
- Học viên ở trạng thái bỏ học (`dropped`) cố nộp bài.

---

## 7. Yêu cầu

### 7.1 P0 – Bắt buộc

#### A. Homework

**FR-60 – Soạn homework** (trong phiên bản chặng nháp)

- \[ \] Tiêu đề, đề bài dạng markdown, file đính kèm (đề bài, dữ liệu mẫu).
- \[ \] Bộ tiêu chí chấm: danh sách tiêu chí, mỗi tiêu chí có mô tả và điểm tối đa. Tổng điểm = tổng điểm tối đa các tiêu chí.
- \[ \] Ngưỡng đạt theo % tổng điểm, mặc định 70% **\[đề xuất\]**.
- \[ \] Cờ bắt buộc.
- \[ \] Điều kiện phát hành chặng: mỗi homework có đề bài và ít nhất 1 tiêu chí.
- \[ \] Không sửa được homework trong phiên bản chặng đã phát hành.

**FR-61 – Hạn nộp theo lớp**

- \[ \] Admin hoặc giảng viên phụ trách đặt hạn nộp cho từng homework ở cấp lớp, khi lớp ở trạng thái `draft` hoặc `active`.
- \[ \] Chưa đặt hạn nghĩa là không có hạn.
- \[ \] Nộp sau hạn vẫn được nhận và gắn nhãn "Nộp muộn" **\[đề xuất\]**.
- \[ \] Mọi lần đổi hạn được ghi nhật ký thao tác.

**FR-62 – Nộp bài**

Vòng đời bài nộp:

```
Chưa nộp ──nộp──> Chờ chấm ──chấm (≥ ngưỡng)──> Đạt
                    ↑    │
             nộp lại│    │chấm (< ngưỡng)
                    │    ↓
                  Cần làm lại

```

- \[ \] Bắt buộc có ít nhất một trong hai: link repo hoặc file.
- \[ \] **Link repo:** URL https hợp lệ của GitHub, GitLab hoặc Bitbucket **\[đề xuất\]**, kèm trường commit hash hoặc tag (không bắt buộc). Lý do: repo vẫn thay đổi được sau khi nộp; commit hash cho biết chính xác phiên bản được chấm.
- \[ \] **File:** tối đa 5 file, mỗi file ≤ 20 MB; định dạng cho phép: zip, pdf, md, txt, sql, png, jpg **\[đề xuất – OQ-6\]**. Lưu trên kho lưu trữ riêng tư.
- \[ \] Mỗi lần nộp tạo một bản nộp mới, bất biến, ghi thời điểm nộp.
- Given bài đang "Chờ chấm", When học viên nộp lại, Then bản trước chuyển thành "Đã thay thế" và rời khỏi hàng đợi chấm, bản mới vào hàng đợi.
- Given bài đã "Đạt", When học viên mở homework, Then không có nút nộp lại.
- Given học viên ở trạng thái `dropped` hoặc lớp không `active`, When học viên gọi API nộp bài, Then backend từ chối.

**FR-63 – Chấm bài**

- \[ \] Giảng viên chỉ chấm được bài của lớp mình phụ trách; Admin chấm được mọi lớp.
- \[ \] Giảng viên chấm bản nộp mới nhất; các bản cũ chỉ xem.
- \[ \] Nhập điểm từng tiêu chí (từ 0 đến điểm tối đa), nhận xét chung (bắt buộc **\[đề xuất\]**), nhận xét từng tiêu chí (không bắt buộc).
- \[ \] Kết quả tự tính: tổng điểm ≥ ngưỡng → Đạt; ngược lại → Cần làm lại.
- \[ \] Sửa điểm sau khi chấm được phép; lưu lịch sử người sửa, thời điểm, điểm cũ và mới.
- \[ \] Học viên nhận email khi bài được chấm; xem được điểm từng tiêu chí và nhận xét.
- \[ \] File học viên nộp chỉ cho tải xuống, không mở trực tiếp trong trình duyệt (tránh thực thi nội dung độc hại).

**FR-64 – Hàng đợi chấm**

- \[ \] Giảng viên có màn hình danh sách bài "Chờ chấm" của mọi lớp mình phụ trách, sắp theo thời gian nộp, bài cũ nhất lên trước.
- \[ \] Mỗi dòng hiển thị: học viên, lớp, chặng, homework, thời điểm nộp, số ngày đã chờ, nhãn nộp muộn.
- \[ \] Lọc theo lớp và theo homework.
- \[ \] Hàng đợi gồm cả lượt quiz bài tập chờ xác nhận (FR-75), cùng thứ tự theo thời điểm nộp. Mỗi dòng có nhãn loại (Homework / Quiz bài tập); lọc thêm theo loại. Lọc "Bài" gồm homework và quiz bài tập.

#### B. Quiz engine (Luyện tập và Đánh giá)

**FR-70 – Soạn quiz** (trong phiên bản chặng nháp; chỉ Admin soạn, theo FR-74)

- \[ \] Form thêm và sửa học liệu chọn loại trong một danh sách: Video, Bài đọc, Quiz học liệu, Quiz bài tập, Bài tập (homework). Đổi giữa hai loại quiz giữ nguyên câu hỏi; đổi sang loại khác bỏ nội dung của loại cũ.
- \[ \] Form có trường "Bài học" bắt buộc (FR-78). Quiz được thêm vào cuối bài học đã chọn.
- \[ \] Loại câu hỏi: một đáp án đúng; nhiều đáp án đúng. Nội dung câu hỏi và giải thích dạng markdown (hỗ trợ code block).
- \[ \] Điểm mỗi câu, mặc định 1.
- \[ \] Câu nhiều đáp án chỉ được điểm khi chọn đúng toàn bộ, không tính điểm một phần **\[đề xuất\]**.
- \[ \] Cấu hình theo chế độ:


| Cấu hình                       | Quiz học liệu – Luyện tập     | Quiz bài tập – Đánh giá                     |
| ------------------------------ | ----------------------------- | ------------------------------------------- |
| Số lượt làm                    | Không giới hạn                | 2 **\[đề xuất – OQ-3\]**                    |
| Giới hạn thời gian             | Không                         | Có, Admin đặt                               |
| Trộn thứ tự câu hỏi và đáp án  | Có                            | Có                                          |
| Tính vào trạng thái hoàn thành | Có (đã làm ≥ 1 lượt)          | Có (đạt ngưỡng)                             |
| Ngưỡng đạt                     | Không áp dụng                 | 70% **\[đề xuất\]**                         |
| Sau khi nộp, học viên thấy     | Điểm, đáp án đúng, giải thích | Điểm và Đạt/Chưa đạt **\[đề xuất – OQ-4\]** |
| Điểm ghi nhận                  | Không áp dụng                 | Lượt cao nhất **\[đề xuất – OQ-3\]**        |
| Giảng viên                     | Nhận xét khi cần (FR-76)      | Xác nhận từng lượt nộp (FR-75)              |


- \[ \] Điều kiện phát hành chặng: mỗi quiz có ít nhất 1 câu; mỗi câu có ít nhất 1 đáp án đúng.

**FR-71 – Làm bài**

- \[ \] Khi bắt đầu lượt, server ghi thời điểm bắt đầu và thời điểm hết giờ.
- \[ \] Đáp án đúng **không bao giờ** được gửi xuống trình duyệt trước khi nộp; việc chấm diễn ra hoàn toàn ở server.
- \[ \] Câu trả lời được tự lưu khi học viên chọn.
- Given học viên mất mạng hoặc đóng tab, When quay lại trong thời gian làm bài, Then tiếp tục đúng lượt đang làm với các câu trả lời đã lưu.
- Given đã hết giờ mà học viên chưa nộp, When server xử lý, Then tự nộp với các câu trả lời đã lưu. Yêu cầu nộp sau giờ hết hạn cộng 30 giây ân hạn bị từ chối **\[đề xuất\]**.
- Given học viên đã dùng hết lượt bài đánh giá, When bấm làm bài, Then không tạo được lượt mới và hiển thị số lượt đã dùng.
- \[ \] Mỗi học viên chỉ có tối đa một lượt đang làm cho mỗi quiz (ràng buộc ở database).
- \[ \] Chỉ làm được khi lớp `active` và học viên đang học.

**FR-74 – Quyền soạn quiz**

- \[ \] Chỉ Admin tạo, sửa, xóa và sắp xếp quiz (cả quiz học liệu lẫn quiz bài tập) và câu hỏi, trong trình soạn chặng (FR-70). Vị trí quiz do Admin đặt: chọn bài học, rồi sắp xếp trong bài học (FR-78).
- \[ \] Giảng viên không có màn hình soạn quiz. Giảng viên chỉ làm việc với lượt làm bài: xác nhận và nhận xét lượt quiz bài tập (FR-75), nhận xét lượt quiz học liệu khi cần (FR-76), xem thống kê câu hỏi và cấp thêm lượt.
- Given giảng viên gọi API tạo, sửa, xóa hoặc sắp xếp học liệu hay câu hỏi, When backend xử lý, Then từ chối vì không đủ quyền.
- Given giảng viên mở đường dẫn trình soạn quiz của Admin, When trang tải, Then chuyển về trang chủ giảng viên.

**FR-75 – Xác nhận quiz bài tập**

Vòng đời xác nhận một lượt quiz bài tập:

```
Nộp ──> Chờ xác nhận ──giảng viên xác nhận──> Đã xác nhận ──sửa nhận xét──> Đã xác nhận (lưu lịch sử)
             │
             └──học viên nộp lượt mới──> Đã có lượt mới hơn (rời hàng chờ)
```

- \[ \] Điểm tự động có hiệu lực ngay khi nộp (tiến độ, tổng kết, Đạt/Chưa đạt). Xác nhận không đổi điểm và không chặn hoàn thành.
- \[ \] Trang xác nhận hiển thị: thông tin lượt (học viên, lớp, bài, giờ bắt đầu, giờ nộp, thời gian làm, điểm tự động, lượt thứ mấy); từng câu theo thứ tự học viên thấy, đáp án học viên chọn, đáp án đúng, đúng hay sai; lịch sử các lượt đã nộp.
- \[ \] Nhận xét chung bắt buộc; ghi chú từng câu không bắt buộc. Giảng viên không sửa điểm (OQ-7).
- \[ \] Sau khi xác nhận, hệ thống chuyển sang mục tiếp theo trong hàng chờ.
- \[ \] Học viên nhận email "Giảng viên đã xác nhận kết quả" và thấy nhận xét chung cùng ghi chú từng câu trong trang kết quả. Ghi chú hiển thị kèm nội dung câu hỏi, không lộ đáp án (giữ quy tắc FR-70 về hiển thị đáp án).
- \[ \] Thời hạn xử lý 3 ngày (FR-65), nhắc giảng viên (FR-66) và tồn đọng (FR-67) áp dụng cho lượt chờ xác nhận như bài chờ chấm.
- Given lượt trước còn "Chờ xác nhận", When học viên nộp lượt mới, Then lượt trước chuyển "Đã có lượt mới hơn", rời hàng chờ, và trang của lượt đó không cho xác nhận mà dẫn tới lượt mới nhất.
- Given lượt đã xác nhận, When giảng viên lưu nhận xét mới, Then nhận xét cũ vào lịch sử (người, thời điểm, nội dung) và học viên nhận email cập nhật.
- Given giảng viên không phụ trách lớp, When mở hoặc gửi xác nhận, Then bị từ chối. Admin xác nhận được mọi lớp.

**FR-76 – Nhận xét quiz học liệu**

- \[ \] Từ thống kê quiz của lớp, giảng viên mở được mọi lượt quiz học liệu đã nộp, xem từng câu như FR-75.
- \[ \] Nhận xét là tùy chọn; lượt quiz học liệu không vào hàng chờ và không tính thời hạn xử lý.
- \[ \] Khi giảng viên gửi nhận xét, học viên nhận email và thấy nhận xét chung cùng ghi chú dưới từng câu trong trang kết quả (đã có đáp án và giải thích).

**FR-77 – Làm quiz từng câu một**

- \[ \] Áp dụng cho cả quiz học liệu và quiz bài tập. Mỗi màn hình hiển thị một câu, kèm "Câu x/n", số câu đã trả lời, trạng thái tự lưu và đồng hồ (nếu có giới hạn thời gian).
- \[ \] Nút "Câu trước" và "Câu tiếp"; ở câu cuối, "Câu tiếp" đổi thành "Nộp bài". Nút "Nộp bài" phụ luôn có trên thanh trạng thái để nộp sớm.
- \[ \] Bản đồ câu hỏi: mỗi câu một nút ≥ 44px, đánh dấu câu đã trả lời và câu đang xem; bấm để chuyển thẳng tới câu đó.
- \[ \] Trong lúc làm bài, ẩn danh sách học liệu bên cạnh để học viên tập trung.
- \[ \] Khi quay lại lượt đang làm, mở câu đang xem nếu vẫn trong phiên trình duyệt đó; nếu không, mở câu chưa trả lời đầu tiên.
- \[ \] Chuyển câu đưa focus bàn phím tới câu mới và thông báo cho trình đọc màn hình ("Câu x trên n"); tôn trọng chế độ giảm chuyển động.
- Given còn câu chưa trả lời, When học viên nộp bài, Then hộp xác nhận nêu số câu chưa trả lời.

#### B2. Cấu trúc chặng

**FR-78 – Bài học**

Một phiên bản chặng gồm các bài học có thứ tự; mỗi bài học gồm các học liệu có thứ tự. Cấp bài học thay đổi mô hình chặng của MVP (FR-11, FR-30).

- \[ \] Mỗi bài học có: tiêu đề (không trùng trong cùng phiên bản chặng), `unit_key`, thứ tự. Bài học không có cờ bắt buộc riêng; tính bắt buộc nằm ở học liệu.
- \[ \] Mỗi học liệu thuộc đúng một bài học của cùng phiên bản chặng.
- \[ \] Trong phiên bản nháp, Admin thêm, đổi tên, xóa và sắp xếp bài học; thêm học liệu vào một bài học; chuyển học liệu sang bài học khác (học liệu vào cuối bài học mới). Sắp xếp học liệu đi qua ranh giới bài học: đưa học liệu đầu bài học lên thì nó vào cuối bài học trước; đưa học liệu cuối bài học xuống thì nó vào đầu bài học sau.
- \[ \] Xóa bài học xóa luôn các học liệu trong đó, sau hộp xác nhận nêu số học liệu sẽ bị xóa **\[đề xuất – OQ-12\]**.
- \[ \] Mỗi chặng kết thúc bằng bài học **"Kiểm tra cuối chặng"**, chứa homework và quiz bài tập của chặng. Đây là quy ước nội dung, hệ thống không chặn phát hành nếu thiếu **\[đề xuất\]**.
- \[ \] Điều kiện phát hành chặng (bổ sung FR-12): có ít nhất 1 bài học; mọi bài học có ít nhất 1 học liệu.
- \[ \] Bài học **hoàn thành** khi mọi học liệu bắt buộc trong đó hoàn thành (theo định nghĩa ở mục 5). Bài học chỉ có học liệu không bắt buộc không có trạng thái hoàn thành và hiển thị "Không bắt buộc" **\[đề xuất – OQ-13\]**. Trạng thái bài học là số liệu tính toán, không lưu.
- \[ \] % chặng và % khóa (FR-33) vẫn tính theo học liệu bắt buộc, không theo số bài học.
- \[ \] Lộ trình học viên (FR-30): trong mỗi chặng, học liệu nhóm theo bài học. Mỗi bài học hiển thị "Bài i.j · tiêu đề", số học liệu bắt buộc đã xong trên tổng và nhãn "Hoàn thành" khi hoàn thành. Thẻ chặng ghi số bài học, số học liệu và số học liệu bắt buộc đã xong.
- \[ \] Mỗi học liệu mở trên trang riêng. Đường dẫn vị trí (breadcrumb): Lớp của tôi / lớp / chặng / bài học / học liệu. Danh sách bên cạnh nhóm học liệu theo bài học.
- \[ \] Nút "Học liệu trước" và "Học liệu tiếp" đi theo thứ tự học liệu trong bài học. Ở học liệu cuối của bài học, nút tiếp đổi thành "Sang bài học tiếp" và mở học liệu đầu của bài học sau; ở học liệu đầu, nút trước mở học liệu cuối của bài học trước.
- \[ \] Tổng kết chặng (FR-80) ghi bài học của từng học liệu.
- Given bài học chưa có học liệu, When Admin phát hành chặng, Then bị chặn với thông báo nêu tên bài học đó.
- Given Admin đặt tên bài học trùng một bài học khác trong cùng phiên bản, When lưu, Then bị từ chối.
- Given học viên ở học liệu cuối của bài học, When bấm "Sang bài học tiếp", Then mở học liệu đầu của bài học kế tiếp trong cùng chặng.

#### C. Tổng kết

**FR-80 – Tổng kết chặng của học viên**

- \[ \] Liệt kê từng học liệu bắt buộc với trạng thái Hoàn thành / Chưa hoàn thành và chi tiết:
  - Homework: Chưa nộp / Chờ chấm / Cần làm lại / Đạt; điểm; số lần nộp; nộp muộn.
  - Quiz bài tập: điểm cao nhất; số lượt đã dùng trên tổng số lượt; trạng thái xác nhận của lượt gần nhất.
  - Quiz học liệu: số lượt đã làm; điểm lượt gần nhất.
  - Video, markdown: đã tích hay chưa.
- \[ \] Trạng thái chặng: **Hoàn thành** khi mọi học liệu bắt buộc đã hoàn thành; ngược lại là **Chưa hoàn thành**, kèm danh sách mục còn thiếu.
- \[ \] Nhận xét của giảng viên cho học viên ở chặng đó: văn bản tự do, sửa được, ghi người viết và thời điểm.
- \[ \] Học viên xem được tổng kết của chính mình, gồm cả nhận xét.

**FR-82 – Báo cáo tổng kết lớp**

- \[ \] Ma trận học viên × chặng; mỗi ô hiển thị Hoàn thành / Chưa hoàn thành và số mục còn thiếu.
- \[ \] Lọc: chưa hoàn thành chặng X; có homework "Cần làm lại"; có homework quá hạn chưa nộp; chưa có nhận xét của giảng viên.
- \[ \] Trạng thái "Chưa hoàn thành" không khóa chặng sau và không ảnh hưởng quyền học.
- \[ \] Giảng viên chỉ xem lớp mình phụ trách.

#### D. Thay đổi trên nền MVP

**FR-90 – Mở rộng versioning**

- \[ \] Nhân bản chặng (FR-13) sao chép sâu thêm: cấu hình quiz, câu hỏi, đáp án, đề homework, tiêu chí chấm. File đính kèm chỉ sao chép tham chiếu.
- \[ \] Câu hỏi giữ `question_key` qua các phiên bản, giống `lesson_key`.
- \[ \] Nhân bản chặng sao chép bài học (FR-78): bài học mới có id mới, giữ `unit_key`, tiêu đề và thứ tự; mỗi học liệu được sao chép trỏ tới bài học tương ứng trong phiên bản mới.
- \[ \] Bài học của phiên bản đã phát hành bất biến như học liệu.
- \[ \] Câu hỏi, đáp án, tiêu chí của phiên bản đã phát hành được bảo vệ bất biến ở tầng service và database, cùng cơ chế với học liệu.

**FR-91 – Tiến độ**

- \[ \] Nút "Đã học xong" chỉ hiển thị với video và markdown.
- \[ \] % chặng và % khóa (FR-33) tính theo định nghĩa hoàn thành ở mục 5.
- \[ \] Trạng thái hoàn thành bài học (FR-78) chỉ để hiển thị; không thay đổi công thức %.

**FR-92 – Nhật ký thao tác bổ sung**

- \[ \] Ghi nhật ký cho: đổi hạn nộp, chấm điểm, sửa điểm, cấp thêm lượt làm bài, sửa nhận xét tổng kết, xác nhận quiz bài tập, nhận xét quiz học liệu, sửa nhận xét quiz.

### 7.2 P1 – Làm ngay sau P0


| Mã    | Yêu cầu                        | Tiêu chí chấp nhận                                                                                                                               |
| ----- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| FR-65 | Theo dõi thời hạn chấm         | Thời hạn chấm hoặc xác nhận 3 ngày kể từ khi nộp **\[đề xuất – OQ-5\]**; bài và lượt quiz quá hạn được đánh dấu nổi bật trong hàng đợi             |
| FR-66 | Nhắc giảng viên chấm trễ       | Email gửi giảng viên mỗi ngày khi có bài quá hạn chấm **\[đề xuất\]**; không gửi khi không có bài quá hạn                                        |
| FR-67 | Tồn đọng chấm cho Admin        | Theo từng giảng viên: số mục chờ (bài và lượt quiz bài tập), số mục quá hạn, thời gian xử lý trung bình                                          |
| FR-68 | Nhắc hạn nộp cho học viên      | Email trước hạn 24 giờ cho học viên chưa nộp **\[đề xuất\]**; không gửi nếu homework không có hạn                                                |
| FR-72 | Cấp thêm lượt làm bài đánh giá | Giảng viên hoặc Admin cấp thêm lượt cho từng học viên, bắt buộc nhập lý do, có ghi lịch sử                                                       |
| FR-73 | Thống kê câu hỏi               | Theo lớp: tỉ lệ trả lời đúng từng câu; đánh dấu câu có tỉ lệ đúng &lt; 30%                                                                       |
| FR-81 | Tổng kết khóa của học viên     | Gộp các chặng: số chặng hoàn thành / tổng số chặng, mục còn thiếu theo chặng, điểm đánh giá và homework từng chặng. Không có điểm tổng toàn khóa |


Lý do để ở P1: thiếu các mục này thì quy trình vẫn chạy được (giảng viên vẫn thấy hàng đợi, ngoại lệ có thể xử lý thủ công), nhưng vận hành nhiều lớp song song sẽ khó.

### 7.3 P2 – Định hướng thiết kế, chưa làm


| Hạng mục                        | Ảnh hưởng thiết kế ở sprint này                                                                               |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Chấm code tự động               | `submissions` lưu `repo_url` + `commit_ref` để sau này một worker chấm tự động có thể đọc đúng phiên bản code |
| Ngân hàng câu hỏi dùng chung    | `question_key` cho phép gom câu hỏi tương đương qua các phiên bản                                             |
| Điểm tổng toàn khóa có trọng số | Điểm thành phần đã lưu riêng từng homework và từng quiz, tính thêm được mà không đổi cấu trúc                 |
| Mở chặng tuần tự                | Trạng thái hoàn thành chặng (FR-80) dùng lại được làm điều kiện mở khóa                                       |
| Câu hỏi tự luận trong quiz      | `questions.type` là trường mở rộng được                                                                       |


---

## 8. Chỉ số thành công

Đo trên lớp pilot đầu tiên chạy Sprint 2. Nguồn số liệu: database của hệ thống (truy vấn trên `submissions`, `submission_grades`, `quiz_attempts`).

### Leading – đánh giá sau 2 tuần chạy lớp pilot


| Chỉ số                                                     | Thành công         | Kỳ vọng cao |
| ---------------------------------------------------------- | ------------------ | ----------- |
| % học viên có ít nhất 1 bài homework được nộp              | ≥ 85%              | ≥ 95%       |
| Thời gian trung bình từ lúc nộp đến lúc chấm               | ≤ 3 ngày           | ≤ 1 ngày    |
| % bài được chấm trong thời hạn                             | ≥ 90%              | ≥ 98%       |
| % học viên làm ít nhất 1 lượt luyện tập ở mỗi chặng đã mở  | ≥ 80%              | ≥ 90%       |
| Số lỗi làm bài đánh giá (mất bài, sai giờ, không nộp được) | 0 lỗi nghiêm trọng | 0           |


### Lagging – đánh giá khi lớp pilot kết thúc


| Chỉ số                                     | Mục tiêu                                            |
| ------------------------------------------ | --------------------------------------------------- |
| % học viên hoàn thành từng chặng           | Lấy lớp pilot làm mốc cho các lớp sau               |
| Số lần nộp lại trung bình mỗi homework     | Theo dõi; cao bất thường là dấu hiệu đề bài chưa rõ |
| Số câu hỏi đánh giá có tỉ lệ đúng &lt; 30% | Toàn bộ được rà soát trước khi chạy lớp tiếp theo   |
| Số homework phải xử lý ngoài hệ thống      | 0                                                   |


---

## 9. Câu hỏi mở


| #    | Câu hỏi                                                                                                                   | Người trả lời       | Chặn                |
| ---- | ------------------------------------------------------------------------------------------------------------------------- | ------------------- | ------------------- |
| OQ-1 | Repo private: học viên phải để public hay thêm giảng viên làm collaborator? Để public thì học viên khác xem được lời giải | Nghiệp vụ           | **Chặn FR-62**      |
| OQ-2 | Homework tính hoàn thành khi được chấm Đạt hay chỉ cần nộp?                                                               | Nghiệp vụ           | **Chặn FR-80**      |
| OQ-3 | Điểm đánh giá lấy lượt cao nhất hay lượt cuối? Số lượt mặc định                                                           | Nghiệp vụ           | Không               |
| OQ-4 | Học viên có được xem đáp án bài đánh giá không, và khi nào (sau khi hết lượt hay khi lớp kết thúc)?                       | Nghiệp vụ           | Không               |
| OQ-5 | Thời hạn chấm 3 ngày tính theo ngày lịch hay ngày làm việc? Ai nhận cảnh báo khi giảng viên chấm trễ?                     | Nghiệp vụ           | Không (P1)          |
| OQ-6 | Giới hạn file nộp (dung lượng, định dạng) và có cần quét mã độc không?                                                    | Kỹ thuật, Bảo mật   | Không               |
| OQ-7 | Bài đánh giá có đáp án sai khi lớp đang chạy: chỉ cấp thêm lượt, hay cho phép loại câu đó khỏi tính điểm cho lớp?         | Nghiệp vụ, Kỹ thuật | Không               |
| OQ-8 | Giảng viên có quyền soạn đề homework và quiz không, hay chỉ Admin? (liên quan Q2 của spec MVP). **Đã chốt 13/10/2026 phần quiz:** chỉ Admin soạn cả hai loại quiz; giảng viên chỉ xác nhận và nhận xét lượt làm bài (FR-74). Homework vẫn do Admin soạn cho tới khi nghiệp vụ trả lời phần còn lại | Nghiệp vụ           | Không (quiz đã chốt) |
| OQ-9 | Có cần hạn làm quiz bài tập theo từng lớp như hạn nộp homework (FR-61) không?                                             | Nghiệp vụ           | Không               |
| OQ-10 | ~~Giảng viên được soạn quiz ở mọi chặng hay chỉ ở chặng thuộc khóa học mình đang dạy?~~ **Không còn áp dụng 13/10/2026:** giảng viên không soạn quiz (FR-74) | Nghiệp vụ           | Không               |
| OQ-11 | ~~Giảng viên có được tự nhân bản chặng để có bản nháp, hay luôn nhờ Admin?~~ **Không còn áp dụng 13/10/2026:** chỉ Admin nhân bản và soạn chặng (FR-74) | Nghiệp vụ           | Không               |
| OQ-12 | Xóa bài học còn học liệu: xóa luôn học liệu (sau xác nhận) hay từ chối cho tới khi bài học trống? | Nghiệp vụ           | Không               |
| OQ-13 | Bài học chỉ có học liệu không bắt buộc có cần trạng thái hoàn thành (ví dụ khi đã mở hoặc tích mọi học liệu) không? | Nghiệp vụ           | Không               |
| OQ-14 | Giảng viên có cần báo cáo tiến độ theo bài học (ngoài theo chặng) không? | Nghiệp vụ           | Không               |
| OQ-15 | Dữ liệu hiện có: khi thêm cấp bài học, mỗi phiên bản chặng đã phát hành được gán một bài học mặc định chứa mọi học liệu, hay Admin chia lại qua bản nháp mới? | Nghiệp vụ, Kỹ thuật | **Chặn migration FR-78** |


---

## 10. Kế hoạch triển khai

### Phụ thuộc

- **Sprint 1 (MVP) hoàn thành:** học liệu, versioning chặng và khóa học, lớp học, tiến độ, email.
- **Kho lưu trữ file** (Q5 của spec MVP): cần cho file nộp homework.
- **Dịch vụ email** (Q6 của spec MVP): cần cho email bài đã được chấm.
- **Câu hỏi chặn** OQ-1, OQ-2 phải có câu trả lời trước khi bắt đầu. OQ-8 đã chốt phần quiz.

### Thứ tự trong sprint


| Thứ tự | Nội dung                                                        | Kết quả kiểm chứng                                          |
| ------ | --------------------------------------------------------------- | ----------------------------------------------------------- |
| 1      | Bài học P0: FR-78, FR-90 phần bài học                           | Admin nhóm học liệu thành bài học; học viên học theo bài học |
| 2      | Homework P0: FR-60 → FR-64, FR-90 phần homework                 | Học viên nộp bài và nhận điểm, nhận xét                     |
| 3      | Quiz engine P0: FR-70, FR-71, FR-74 → FR-77, FR-90 phần quiz    | Học viên làm quiz từng câu; giảng viên xác nhận quiz         |
| 4      | Tổng kết P0: FR-80, FR-82, FR-91, FR-92                         | Giảng viên xem được học viên còn thiếu gì trên một màn hình |
| 5      | P1 theo thứ tự: FR-65, FR-72, FR-66, FR-67, FR-68, FR-73, FR-81 | —                                                           |


Bài học làm đầu tiên vì homework và quiz đều cần biết mình thuộc bài học nào. Homework làm tiếp theo vì giá trị cao nhất với khóa lập trình và không phụ thuộc quiz engine. Tổng kết làm cuối vì chỉ tổng hợp dữ liệu của hai phần trước.

### Rủi ro về quy mô

Chưa ước lượng vì chưa có thông tin về số người trong nhóm và độ dài sprint. Phạm vi P0 gồm 3 khối độc lập (Homework, Quiz engine, Tổng kết), nên nhiều khả năng vượt một sprint 2 tuần với nhóm nhỏ. **Điểm cắt nếu không đủ năng lực:** giữ Homework và Tổng kết ở Sprint 2, chuyển Quiz engine sang Sprint 3. Tổng kết vẫn chạy được với dữ liệu homework và học liệu nội dung.

---

## Phụ lục A – Bảng dữ liệu bổ sung


| Bảng                     | Trường chính                                                                                                      |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------- |
| `stage_units`            | stage\_version, unit\_key, tiêu đề, thứ tự (bài học, FR-78)                                                        |
| `lessons`                | `type` thêm `quiz`, `homework`; thêm `unit_id` (bắt buộc, trỏ `stage_units`)                                        |
| `quiz_settings`          | lesson, chế độ (practice / assessment), số lượt, giới hạn thời gian, trộn câu, ngưỡng đạt, chế độ hiển thị đáp án |
| `questions`              | lesson, question\_key, loại, nội dung, giải thích, điểm, thứ tự                                                   |
| `question_options`       | question, nội dung, là đáp án đúng, thứ tự                                                                        |
| `quiz_attempts`          | class\_member, lesson, số thứ tự lượt, started\_at, deadline\_at, submitted\_at, điểm, trạng thái, review\_status (null / pending / confirmed / superseded) |
| `quiz_attempt_reviews`   | attempt, người nhận xét, nhận xét chung, created\_at. Mỗi lần lưu là một dòng mới; dòng mới nhất là nhận xét hiện hành, các dòng cũ là lịch sử |
| `quiz_review_notes`      | review, question, ghi chú                                                                                         |
| `quiz_attempt_answers`   | attempt, question, các lựa chọn, đúng hay sai                                                                     |
| `quiz_attempt_grants`    | class\_member, lesson, số lượt cấp thêm, người cấp, lý do (P1)                                                    |
| `homework_settings`      | lesson, đề bài, ngưỡng đạt                                                                                        |
| `rubric_criteria`        | lesson, mô tả, điểm tối đa, thứ tự                                                                                |
| `class_lesson_deadlines` | class, lesson, hạn nộp                                                                                            |
| `submissions`            | class\_member, lesson, số thứ tự lần nộp, repo\_url, commit\_ref, submitted\_at, nộp muộn, trạng thái             |
| `submission_files`       | submission, media\_file                                                                                           |
| `submission_grades`      | submission, người chấm, tổng điểm, kết quả, nhận xét, graded\_at                                                  |
| `grade_criterion_scores` | grade, tiêu chí, điểm, nhận xét                                                                                   |
| `stage_reviews`          | class\_member, stage\_version, nhận xét, người viết, updated\_at                                                  |


Ràng buộc chính:

- `UNIQUE(class_member_id, lesson_id, attempt_no)` cho `quiz_attempts` và `submissions`.
- Partial unique index: mỗi (class_member, lesson) chỉ có một lượt quiz ở trạng thái đang làm.
- `UNIQUE(class_member_id, stage_version_id)` cho `stage_reviews`.
- `quiz_attempts.review_status = pending` chỉ cho lượt quiz bài tập đã nộp; partial unique index: mỗi (class_member, lesson) tối đa một lượt `pending`. Nộp lượt mới chuyển lượt `pending` cũ sang `superseded` trong cùng transaction.
- `UNIQUE(review_id, question_id)` cho `quiz_review_notes`.
- `UNIQUE(stage_version_id, unit_key)`, `UNIQUE(stage_version_id, title)` và `UNIQUE(stage_version_id, sort_order)` cho `stage_units`.
- `lessons.unit_id` phải thuộc cùng phiên bản chặng với học liệu (khóa ngoại kép `(unit_id, stage_version_id)`). Thứ tự học liệu duy nhất trong mỗi bài học.
- Bảng `lessons` của app hiện tại (`apps/`, `docs/database.md`) chưa có `unit_id`; cần migration tạo `stage_units`, gán học liệu hiện có vào bài học theo câu trả lời OQ-15, rồi đặt `unit_id` là bắt buộc.
- `UNIQUE(class_id, lesson_id)` cho `class_lesson_deadlines`.