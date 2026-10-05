# Spec MVP – LMS đào tạo học viên

- **Ngày:** 03/10/2026
- **Tác giả:** Nguyễn Văn Thược
- **Trạng thái:** Draft – chờ review
- **Phạm vi:** MVP (giai đoạn 1)

> Quy ước: các con số đánh dấu **[đề xuất]** là giá trị mặc định do người viết spec đưa ra, cần xác nhận trước khi chốt.

---

## 1. Bối cảnh và vấn đề

Đơn vị tự vận hành chương trình đào tạo **miễn phí** cho **học viên bên ngoài**, không có quan hệ lao động với công ty. Hệ thống dùng nội bộ, không bán dạng SaaS.

Cần một hệ thống để:
- Quản lý học viên theo lớp (basic01, basic02, …).
- Quản lý học liệu nhiều chặng, dùng lại được giữa các khóa học.
- Theo dõi tiến độ từng học viên.

Vấn đề cốt lõi cần giải quyết từ đầu: **học liệu thay đổi theo thời gian, nhưng lớp đang học không được bị ảnh hưởng**. Vì vậy học liệu được quản lý bằng phiên bản (versioning).

---

## 2. Mục tiêu

| # | Mục tiêu | Cách đo | Mục tiêu đạt |
|---|---|---|---|
| G1 | Admin mở được một lớp mới nhanh: tạo lớp, gắn khóa, mời học viên | Thời gian từ tạo lớp đến gửi xong lời mời cho 30 học viên | ≤ 30 phút **[đề xuất]** |
| G2 | Học viên vào học được ngay sau khi được mời | % học viên đăng nhập và đổi mật khẩu trong 72 giờ | ≥ 90% **[đề xuất]** |
| G3 | Giảng viên/Admin thấy tiến độ cả lớp trên một màn hình, không tổng hợp thủ công | Có / không | Có |
| G4 | Sửa học liệu không làm sai lệch lớp đang chạy | Số sự cố tiến độ hoặc nội dung lớp bị thay đổi ngoài ý muốn | 0 |
| G5 | Học liệu được dùng lại giữa các khóa | Số chặng được gắn vào ≥ 2 khóa học | Theo dõi, chưa đặt ngưỡng |

---

## 3. Ngoài phạm vi MVP

| Hạng mục | Lý do |
|---|---|
| Thanh toán, hóa đơn | Học viên không trả phí |
| Tự đăng ký, OTP, import danh sách, duyệt hồ sơ | Admin mời trực tiếp qua email là đủ cho MVP |
| Quiz / bài thi tự động | Làm sau. Mô hình dữ liệu đã chừa loại học liệu `quiz` |
| Điểm danh, lịch học theo chặng | Làm rõ sau |
| Chứng chỉ, trang xác thực chứng chỉ | Không cần cho MVP |
| Xuất Excel | Báo cáo xem trên màn hình là đủ |
| Theo dõi tiến độ tự động (đo thời gian xem video) | Tiến độ chỉ do học viên tự tích |
| Nâng phiên bản khóa học cho lớp đang `active` | Phức tạp do phải ánh xạ tiến độ; lớp chạy trọn đời trên một phiên bản |
| Tùy biến khóa học riêng cho từng lớp | Lớp học nguyên khóa |
| App mobile | Web responsive là đủ |

---

## 4. Vai trò và quyền

| Vai trò | Quyền chính |
|---|---|
| **Admin** | Quản lý học liệu, chặng, khóa học, phiên bản; tạo và vận hành lớp; mời học viên; xem mọi báo cáo; vô hiệu hóa tài khoản |
| **Giảng viên** | Xem lớp được phân công và tiến độ học viên trong lớp đó **[đề xuất – xem câu hỏi mở Q2]** |
| **Học viên** | Xem các lớp mình tham gia; học; tích hoặc bỏ tích hoàn thành học liệu |

---

## 5. User stories

### Admin
1. Là Admin, tôi muốn mời học viên vào lớp bằng email để họ nhận được thông tin đăng nhập mà không cần tự đăng ký.
2. Là Admin, tôi muốn gửi lại lời mời cho học viên chưa đăng nhập hoặc mật khẩu tạm đã hết hạn, để họ vẫn vào được lớp.
3. Là Admin, tôi muốn tạo chặng (Database, Golang basic, …) và thêm học liệu video, markdown vào chặng, để xây dựng nội dung một lần và dùng lại nhiều nơi.
4. Là Admin, tôi muốn ghép các chặng thành một khóa học theo thứ tự, để tạo chương trình học hoàn chỉnh.
5. Là Admin, tôi muốn sửa học liệu bằng cách tạo phiên bản chặng mới, để lớp đang học không bị ảnh hưởng.
6. Là Admin, tôi muốn áp dụng phiên bản chặng mới vào khóa học bằng một thao tác, để không phải tự nhân bản và phát hành khóa học từng bước.
7. Là Admin, tôi muốn thấy khóa học nào còn dùng phiên bản chặng cũ, để quyết định có cập nhật hay không.
8. Là Admin, tôi muốn tạo lớp và gắn một phiên bản khóa học, và được đổi phiên bản khi lớp chưa bắt đầu.
9. Là Admin, tôi muốn kích hoạt và kết thúc lớp, để kiểm soát thời điểm học viên được học.

### Giảng viên
10. Là Giảng viên, tôi muốn xem tiến độ từng học viên theo từng chặng, để biết ai đang chậm và cần nhắc.
11. Là Giảng viên, tôi muốn lọc học viên chưa đăng nhập hoặc lâu không hoạt động, để liên hệ kịp thời.

### Học viên
12. Là Học viên, tôi muốn đăng nhập bằng mật khẩu tạm trong email và đặt mật khẩu của mình, để bảo vệ tài khoản.
13. Là Học viên, tôi muốn thấy lộ trình chặng và học liệu của lớp cùng % tiến độ, để biết mình đang ở đâu.
14. Là Học viên, tôi muốn xem video và đọc tài liệu markdown ngay trên hệ thống.
15. Là Học viên, tôi muốn tích "Đã học xong" cho từng học liệu, và bỏ tích nếu tích nhầm.
16. Là Học viên, tôi muốn lấy lại mật khẩu qua email khi quên.

### Trường hợp biên
- Mật khẩu tạm hết hạn trước khi học viên đăng nhập.
- Email được mời đã có tài khoản (học viên học tiếp lớp thứ hai).
- Email gửi thất bại hoặc bị trả về.
- Học viên đã ở trong lớp được mời lại lần nữa.
- Admin cố sửa học liệu của một phiên bản đã phát hành.
- Admin cố đổi phiên bản khóa học của lớp đang `active`.

---

## 6. Yêu cầu chức năng P0

### 6.1 Tài khoản và lời mời

**FR-01 – Mời học viên vào lớp**

Admin nhập email và họ tên học viên, chọn lớp, bấm mời.

- [ ] Email được chuẩn hóa (cắt khoảng trắng, chuyển chữ thường) trước khi so khớp.
- [ ] **Email chưa có tài khoản:** tạo tài khoản vai trò Học viên, trạng thái `invited`; sinh mật khẩu ngẫu nhiên tối thiểu 12 ký tự bằng bộ sinh số ngẫu nhiên an toàn mật mã; chỉ lưu dạng hash (argon2id hoặc bcrypt); đặt `must_change_password = true`; mật khẩu tạm hết hạn sau 72 giờ **[đề xuất]**; thêm vào lớp; gửi email gồm email đăng nhập, mật khẩu tạm, đường dẫn đăng nhập, thời hạn.
- [ ] **Email đã có tài khoản đang hoạt động:** không tạo lại mật khẩu; chỉ thêm vào lớp và gửi email thông báo được thêm vào lớp.
- [ ] **Học viên đã ở trong lớp:** báo lỗi "Học viên đã có trong lớp", không gửi email.
- [ ] Không mời được vào lớp `ended`.
- [ ] Mật khẩu tạm không bao giờ hiển thị lại trên giao diện Admin và không xuất hiện trong log.
- [ ] Email được gửi qua hàng đợi, thử lại tối đa 3 lần **[đề xuất]**. Trạng thái lời mời (`queued` / `sent` / `failed`) hiển thị trong danh sách học viên của lớp.

**FR-02 – Gửi lại lời mời**
- [ ] Chỉ áp dụng cho tài khoản chưa đổi mật khẩu lần đầu.
- [ ] Sinh mật khẩu tạm mới, mật khẩu tạm cũ mất hiệu lực ngay, thời hạn tính lại từ đầu.

**FR-03 – Đăng nhập lần đầu và bắt buộc đổi mật khẩu**
- Given mật khẩu tạm đã hết hạn, When học viên đăng nhập, Then từ chối và hiển thị "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên".
- Given `must_change_password = true`, When học viên gọi bất kỳ API nào ngoài đổi mật khẩu và đăng xuất, Then backend trả lỗi 403 với mã `PASSWORD_CHANGE_REQUIRED`. **Chặn ở backend, không chỉ ở giao diện.**
- [ ] Mật khẩu mới tối thiểu 8 ký tự **[đề xuất]** và khác mật khẩu tạm.
- [ ] Đổi thành công: `must_change_password = false`, trạng thái tài khoản `active`, xóa thời hạn mật khẩu tạm.

**FR-04 – Quên mật khẩu**
- [ ] Gửi email chứa đường dẫn có token dùng một lần, hết hạn sau 30 phút **[đề xuất]**.
- [ ] Phản hồi giống nhau dù email có tồn tại hay không, để không lộ danh sách tài khoản.

**FR-05 – Chống dò mật khẩu**
- [ ] Tạm khóa đăng nhập sau 5 lần sai trong 15 phút **[đề xuất]**.

**FR-06 – Vô hiệu hóa tài khoản**
- [ ] Admin vô hiệu hóa được tài khoản học viên; tài khoản bị vô hiệu không đăng nhập được, phiên đăng nhập hiện tại bị hủy. Dữ liệu tiến độ giữ nguyên.

### 6.2 Học liệu và phiên bản

Quy tắc chi tiết xem **mục 7**.

**FR-10 – Tạo chặng**
- [ ] Admin nhập mã (duy nhất) và tên chặng. Hệ thống tạo chặng kèm phiên bản nháp v1.

**FR-11 – Quản lý học liệu trong chặng nháp**
- [ ] Thêm, sửa, xóa, sắp xếp học liệu. Chỉ thao tác được trên phiên bản **nháp**.
- [ ] Mỗi học liệu có: tiêu đề, loại (`video` / `markdown`), thứ tự, cờ bắt buộc.
- [ ] Học liệu `video` bắt buộc có file video. Học liệu `markdown` bắt buộc có nội dung.
- [ ] Markdown được render và lọc HTML phía server để chặn mã độc (XSS). Ảnh trong bài được upload lên kho lưu trữ, không lưu dạng base64.
- [ ] Video phát qua đường dẫn có chữ ký, hết hạn ngắn; hỗ trợ tua.

**FR-12 – Phát hành chặng**
- [ ] Điều kiện: có ít nhất 1 học liệu.
- [ ] Sau khi phát hành, phiên bản chặng và toàn bộ học liệu của nó **không sửa được**.

**FR-13 – Nhân bản chặng**
- [ ] Từ một phiên bản đã phát hành, tạo phiên bản nháp mới với số phiên bản kế tiếp.
- [ ] Học liệu được sao chép sang phiên bản mới, giữ nguyên `lesson_key`. File video, ảnh chỉ sao chép tham chiếu, không sao chép file.
- [ ] Mỗi chặng chỉ có tối đa một bản nháp. Nếu đã có nháp, báo lỗi và dẫn tới bản nháp đó.

**FR-14 – Tạo và soạn khóa học**
- [ ] Admin nhập mã (duy nhất) và tên. Hệ thống tạo khóa học kèm phiên bản nháp v1.
- [ ] Trong bản nháp, Admin chọn các phiên bản chặng và sắp xếp thứ tự.
- [ ] Một phiên bản khóa học không được chứa hai phiên bản của cùng một chặng.

**FR-15 – Phát hành khóa học**
- [ ] Điều kiện: có ít nhất 1 chặng và **mọi chặng đều đã phát hành**.
- [ ] Sau khi phát hành, danh sách và thứ tự chặng không sửa được.

**FR-16 – Nhân bản khóa học**
- [ ] Tạo phiên bản nháp mới trỏ tới đúng các phiên bản chặng của bản gốc (nhân bản nông, không sao chép chặng).
- [ ] Mỗi khóa học chỉ có tối đa một bản nháp.

**FR-17 – Áp dụng phiên bản chặng mới cho khóa học (một thao tác)**

Ở màn hình chặng vừa phát hành, Admin chọn một hoặc nhiều khóa học để áp dụng.

- Given chặng Database v2 đã phát hành và khóa Basic đang có v1 (dùng Database v1), When Admin áp dụng Database v2 cho khóa Basic, Then hệ thống trong **một giao dịch**: nhân bản Basic v1 thành Basic v2, thay Database v1 bằng Database v2, giữ nguyên thứ tự và các chặng khác, phát hành Basic v2.
- [ ] Điều kiện: phiên bản chặng đã phát hành; khóa học có chứa một phiên bản của cùng chặng; khóa học chưa có bản nháp.
- [ ] Không thỏa điều kiện hoặc lỗi giữa chừng: **không có gì thay đổi**, hiển thị lý do.
- [ ] Các lớp đang dùng Basic v1 không bị ảnh hưởng.

**FR-18 – Cảnh báo khóa học dùng phiên bản cũ**
- [ ] Màn hình chặng hiển thị danh sách khóa học (phiên bản phát hành mới nhất) đang dùng phiên bản chặng cũ hơn phiên bản phát hành mới nhất, kèm nút áp dụng (FR-17).

**FR-19 – Xóa và lưu trữ**
- [ ] Bản nháp xóa được.
- [ ] Phiên bản đã phát hành mà đang được tham chiếu (chặng nằm trong khóa học, khóa học gắn với lớp) **không xóa được**; thông báo rõ đang được dùng ở đâu.
- [ ] Phiên bản đã phát hành chuyển được sang `archived`: không gắn mới được, nhưng các lớp và khóa học đang dùng vẫn hoạt động bình thường.

### 6.3 Lớp học

**FR-20 – Tạo lớp**
- [ ] Admin nhập mã lớp (duy nhất, ví dụ `basic01`), tên, phiên bản khóa học **đã phát hành**, ngày bắt đầu và kết thúc dự kiến, giảng viên phụ trách.
- [ ] Lớp mới ở trạng thái `draft`.

**FR-21 – Đổi phiên bản khóa học của lớp**
- [ ] Chỉ được đổi khi lớp ở trạng thái `draft`, và chỉ sang phiên bản đã phát hành.
- [ ] Lớp `active` hoặc `ended`: không cho đổi, hiển thị lý do.

**FR-22 – Vòng đời lớp**
- [ ] Chuyển trạng thái một chiều: `draft` → `active` → `ended`.
- [ ] Học viên chỉ học và tích hoàn thành được khi lớp `active`.
- [ ] Lớp `ended`: học viên chỉ xem, không tích được **[đề xuất – xem Q3]**.

**FR-23 – Quản lý học viên trong lớp**
- [ ] Mời học viên theo FR-01.
- [ ] Gỡ học viên khỏi lớp: chuyển trạng thái thành viên sang `dropped`, giữ nguyên dữ liệu tiến độ.

### 6.4 Học tập và tiến độ

**FR-30 – Màn hình lớp của học viên**
- [ ] Học viên thấy danh sách lớp mình tham gia.
- [ ] Trong mỗi lớp: chặng theo thứ tự, học liệu trong từng chặng, trạng thái từng học liệu, % theo chặng và % toàn khóa.
- [ ] Học viên học các chặng theo thứ tự tùy ý (không khóa tuần tự trong MVP).

**FR-31 – Ghi nhận mở học liệu**
- [ ] Lần đầu học viên mở một học liệu, hệ thống ghi `first_opened_at`. Các lần sau không ghi đè.

**FR-32 – Tích và bỏ tích hoàn thành**
- [ ] Nút "Đã học xong" chỉ dùng được khi học liệu đã được mở.
- [ ] Tích: ghi `completed_at`. Bỏ tích: đặt `completed_at` về rỗng.
- [ ] Gửi trùng thao tác không làm sai dữ liệu (idempotent).
- [ ] Server kiểm tra học liệu thuộc đúng phiên bản khóa học của lớp, học viên là thành viên đang học của lớp, và lớp đang `active`.

**FR-33 – Cách tính tiến độ**
- % chặng = số học liệu **bắt buộc** đã tích trong chặng / tổng số học liệu bắt buộc của chặng.
- % khóa = số học liệu bắt buộc đã tích / tổng số học liệu bắt buộc của khóa.
- Học liệu không bắt buộc không tính vào %.
- Tiến độ là số liệu tính toán, không lưu cứng.

### 6.5 Báo cáo lớp

**FR-40 – Bảng tiến độ lớp**
- [ ] Mỗi dòng một học viên: họ tên, email, trạng thái lời mời, % từng chặng, % toàn khóa, lần hoạt động cuối.
- [ ] Bấm vào học viên: xem từng học liệu với trạng thái đã mở / đã tích và thời điểm.
- [ ] Giảng viên chỉ xem được lớp mình phụ trách.

**FR-41 – Lọc và sắp xếp**
- [ ] Lọc: chưa đăng nhập; không hoạt động quá N ngày; % toàn khóa dưới ngưỡng.
- [ ] Sắp xếp theo % toàn khóa và theo lần hoạt động cuối.

**FR-42 – Ghi chú trên báo cáo**
- [ ] Hiển thị rõ: "Tiến độ do học viên tự xác nhận".

### 6.6 Email

**FR-50 – Các email giao dịch trong MVP**

| Email | Kích hoạt khi |
|---|---|
| Lời mời kèm mật khẩu tạm | FR-01 với email mới, FR-02 |
| Thông báo được thêm vào lớp | FR-01 với email đã có tài khoản |
| Đặt lại mật khẩu | FR-04 |

---

## 7. Mô hình học liệu và quy tắc phiên bản

### 7.1 Cấu trúc

```
Lớp ──gắn──> Phiên bản khóa học ──gồm (có thứ tự)──> Phiên bản chặng ──gồm──> Học liệu
                                                                              (video | markdown | quiz sau này)
```

- **Chặng** dùng chung được giữa nhiều khóa học.
- **Học liệu** thuộc về đúng một phiên bản chặng; không dùng chung ở cấp học liệu.
- **Tiến độ** gắn với cặp (thành viên lớp, học liệu).

### 7.2 Vòng đời phiên bản (áp dụng cho cả khóa học và chặng)

```
draft ──phát hành──> published ──lưu trữ──> archived
```

| Quy tắc | Nội dung |
|---|---|
| Chỉ sửa bản nháp | `published` và `archived` bất biến. Muốn sửa thì nhân bản thành bản nháp mới |
| Một bản nháp | Mỗi khóa học / chặng có tối đa một bản nháp tại một thời điểm |
| Số phiên bản | Tăng dần theo từng khóa học / chặng (v1, v2, …) |
| Gắn kết | Lớp chỉ gắn phiên bản khóa học `published`. Khóa học chỉ phát hành khi mọi chặng `published` |
| Xóa | Nháp xóa được. Bản đã phát hành có tham chiếu không xóa được, chỉ lưu trữ |
| Nhân bản chặng | Sâu: sao chép học liệu, giữ `lesson_key`, không sao chép file |
| Nhân bản khóa học | Nông: trỏ lại các phiên bản chặng cũ |
| Không tự lan | Phiên bản chặng mới không tự cập nhật vào khóa học. Admin áp dụng qua FR-17 |
| Lớp | Đổi phiên bản khóa học chỉ khi lớp `draft`. Lớp `active` chạy trọn đời trên một phiên bản |

### 7.3 Kịch bản tham chiếu

**Ban đầu**
```
Khóa Basic v1 ─┬─ Database v1
               ├─ Data structure v1
               ├─ Golang basic v1
               ├─ ReactJS basic v1
               └─ HTML CSS JS v1

basic01 → Basic v1
basic02 → Basic v1
```

**Sửa một bài trong chặng Database**
1. Nhân bản Database v1 → Database v2 (nháp).
2. Sửa học liệu trong Database v2.
3. Phát hành Database v2.
4. Ở màn hình Database v2, chọn "Áp dụng cho khóa Basic" → hệ thống tạo và phát hành Basic v2 (FR-17).
5. Tạo lớp basic03 gắn với Basic v2 (hoặc đổi basic03 sang Basic v2 nếu lớp còn `draft`).

**Kết quả**
```
basic01 → Basic v1 → Database v1
basic02 → Basic v1 → Database v1
basic03 → Basic v2 → Database v2   (4 chặng còn lại dùng chung v1)
```

Tiêu chí chấp nhận cho kịch bản:
- [ ] Học viên basic01, basic02 vẫn thấy nội dung Database v1 và tiến độ không đổi.
- [ ] Học viên basic03 thấy nội dung Database v2.
- [ ] Màn hình chặng Database không còn cảnh báo khóa Basic dùng phiên bản cũ (bản phát hành mới nhất của Basic đã dùng v2).

---

## 8. Yêu cầu phi chức năng

**Bảo mật**
- HTTPS cho toàn bộ hệ thống; hệ thống mở ra Internet nên tách khỏi mạng nội bộ.
- Mật khẩu lưu hash argon2id hoặc bcrypt; không log mật khẩu, token.
- Giới hạn tần suất cho đăng nhập, quên mật khẩu, mời học viên.
- Lọc HTML khi render markdown.
- File video, ảnh truy cập qua đường dẫn có chữ ký, hết hạn ngắn; không public bucket.

**Toàn vẹn dữ liệu**
- Quy tắc bất biến của bản `published` được bảo vệ ở cả tầng service và tầng database (ràng buộc, trigger), không chỉ ở giao diện.
- FR-17 chạy trong một giao dịch.

**Nhật ký thao tác**
- Ghi lại người thực hiện và thời điểm cho: phát hành, nhân bản, áp dụng chặng cho khóa, lưu trữ, đổi phiên bản khóa học của lớp, đổi trạng thái lớp, mời và gửi lại lời mời, vô hiệu hóa tài khoản.

**Dữ liệu cá nhân**
- Học viên là người bên ngoài, nên cần nội dung đồng ý xử lý dữ liệu cá nhân và chính sách lưu trữ, xóa dữ liệu, căn cứ Luật Bảo vệ dữ liệu cá nhân (hiệu lực từ 01/01/2026). Chi tiết do pháp chế xác nhận (Q4).

**Giao diện**
- Web responsive, dùng được trên điện thoại; hỗ trợ bản mới nhất của Chrome, Edge, Safari, Firefox.

**Quy mô**
- Chưa xác định (Q1).

---

## 9. P1 và P2

### P1 – làm ngay sau MVP
| Hạng mục | Ghi chú |
|---|---|
| Quiz | Thêm loại học liệu `quiz`; cấu trúc khóa – chặng – học liệu không đổi |
| Lịch theo chặng cho từng lớp | Bảng `class_stage_schedules`; cho phép báo cáo "chậm so với lịch" |
| Nhắc học viên không hoạt động | Email sau N ngày không hoạt động |
| Mời hàng loạt | Dán nhiều email hoặc import file |
| Xuất Excel báo cáo lớp | Thêm định dạng đầu ra trên cùng tầng truy vấn báo cáo |
| Link mời có token thay cho mật khẩu tạm | Giảm rủi ro mật khẩu nằm trong hộp thư |

### P2 – định hướng thiết kế, chưa làm
| Hạng mục | Ảnh hưởng thiết kế hiện tại |
|---|---|
| Chứng chỉ và trang xác thực | Trạng thái thành viên lớp đã có `completed` để gắn điều kiện cấp |
| Điểm danh | Nếu cần buổi học, tách bảng buổi học riêng |
| Mở chặng tuần tự | Cấu hình ở cấp lớp |
| Phát video HLS, watermark, giới hạn thiết bị | Kho lưu trữ media tách riêng, truy cập qua đường dẫn có chữ ký |
| So sánh kết quả giữa các phiên bản | Dùng `lesson_key` để nối học liệu giữa các phiên bản |

---

## 10. Chỉ số thành công

**Leading (đo trong 1–4 tuần sau khi chạy lớp đầu tiên)**
| Chỉ số | Mục tiêu |
|---|---|
| Tỉ lệ email mời gửi thành công | ≥ 98% **[đề xuất]** |
| Tỉ lệ học viên đăng nhập và đổi mật khẩu trong 72 giờ | ≥ 90% **[đề xuất]** |
| Tỉ lệ học viên có ít nhất 1 học liệu hoàn thành trong tuần đầu | ≥ 80% **[đề xuất]** |
| Thời gian Admin tạo lớp và mời 30 học viên | ≤ 30 phút **[đề xuất]** |

**Lagging (đo sau 1–3 lớp)**
| Chỉ số | Mục tiêu |
|---|---|
| Tỉ lệ hoàn thành khóa học theo lớp | Lấy lớp đầu tiên làm mốc |
| Số sự cố nội dung hoặc tiến độ lớp bị thay đổi ngoài ý muốn | 0 |
| Số chặng được dùng ở ≥ 2 khóa học | Theo dõi |

---

## 11. Câu hỏi mở

| # | Câu hỏi | Người trả lời | Chặn |
|---|---|---|---|
| Q1 | Quy mô: số lớp chạy đồng thời, học viên mỗi lớp, tổng dung lượng và độ dài video | Nghiệp vụ, Hạ tầng | Chặn thiết kế hạ tầng video |
| Q2 | Giảng viên có quyền soạn học liệu và quản lý lớp không, hay chỉ xem tiến độ? | Nghiệp vụ | Chặn phân quyền |
| Q3 | Sau khi lớp kết thúc, học viên còn xem được học liệu không, trong bao lâu? | Nghiệp vụ | Không |
| Q4 | Nội dung đồng ý xử lý dữ liệu cá nhân; thời hạn lưu và xóa dữ liệu học viên | Pháp chế | Chặn go-live |
| Q5 | Lưu trữ video ở đâu (S3, MinIO, hạ tầng nội bộ)? | Hạ tầng | Chặn FR-11 |
| Q6 | Dịch vụ gửi email và tên miền gửi (cấu hình SPF, DKIM) | Hạ tầng | Chặn go-live |
| Q7 | Xác nhận các giá trị [đề xuất]: hạn mật khẩu tạm 72 giờ, độ dài mật khẩu, số lần thử lại email, ngưỡng khóa đăng nhập | Bảo mật, Kỹ thuật | Không |
| Q8 | Lịch học theo chặng cho từng lớp | Nghiệp vụ | Không (đã dời sang P1) |
| Q9 | Admin có cần mời nhiều email trong một lần ngay từ MVP không? | Nghiệp vụ | Không |

---

## 12. Phân giai đoạn và phụ thuộc

### Thứ tự triển khai đề xuất
| Đợt | Nội dung | Kết quả kiểm chứng |
|---|---|---|
| 1 | Tài khoản, lời mời, đổi mật khẩu, quên mật khẩu, email | Mời được học viên thật và đăng nhập được |
| 2 | Học liệu, chặng, khóa học, phiên bản, FR-17 | Chạy được kịch bản mục 7.3 ở mức dữ liệu |
| 3 | Lớp học, màn hình học tập, tích hoàn thành | Học viên học và tích được trong lớp `active` |
| 4 | Báo cáo lớp, lọc | Giảng viên xem tiến độ cả lớp |
| 5 | Chạy thử 1 lớp pilot | Đo các chỉ số leading ở mục 10 |

Chưa ước lượng thời gian vì chưa chốt nhân sự và quy mô (Q1).

### Phụ thuộc
- Dịch vụ gửi email và cấu hình tên miền (Q6).
- Kho lưu trữ file video, ảnh (Q5).
- Xác nhận pháp chế về dữ liệu cá nhân (Q4).

### Mô hình dữ liệu tham chiếu
| Bảng | Trường chính |
|---|---|
| `users` | email, họ tên, vai trò, trạng thái, password_hash, must_change_password, temp_password_expires_at |
| `invitations` | user, lớp, người gửi, trạng thái gửi email, số lần thử, lỗi gần nhất |
| `media_files` | khóa lưu trữ, loại file, dung lượng |
| `stages` | mã, tên |
| `stage_versions` | chặng, số phiên bản, trạng thái, nhân bản từ, thời điểm phát hành |
| `lessons` | phiên bản chặng, lesson_key, loại, tiêu đề, nội dung markdown, file video, thứ tự, bắt buộc |
| `courses` | mã, tên |
| `course_versions` | khóa học, số phiên bản, trạng thái, nhân bản từ, thời điểm phát hành |
| `course_version_stages` | phiên bản khóa học, phiên bản chặng, thứ tự |
| `classes` | mã, tên, phiên bản khóa học, trạng thái, ngày bắt đầu và kết thúc, giảng viên |
| `class_members` | lớp, học viên, trạng thái (đang học / hoàn thành / bỏ học) |
| `lesson_progress` | thành viên lớp, học liệu, first_opened_at, completed_at |
| `audit_logs` | người thực hiện, hành động, đối tượng, thời điểm, dữ liệu trước và sau |

Ràng buộc chính:
- `UNIQUE(course_id, version_no)`, `UNIQUE(stage_id, version_no)`.
- Partial unique index: mỗi khóa học / chặng chỉ một bản `draft`.
- `UNIQUE(course_version_id, stage_id)`: một khóa không chứa hai phiên bản của cùng chặng.
- `UNIQUE(class_id, user_id)` trong `class_members`.
- Khóa ngoại `ON DELETE RESTRICT` cho các tham chiếu tới bản đã phát hành.
