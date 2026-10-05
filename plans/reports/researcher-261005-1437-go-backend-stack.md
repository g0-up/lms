# Nghiên cứu stack backend Go cho LMS MVP

Ngày: 2026-10-05. Phạm vi: Go + Gin + sqlx/pgx v5 + golang-migrate + cobra + PostgreSQL. Phiên bản lấy trực tiếp từ `proxy.golang.org/<module>/@latest` [S1].

## Kết luận nhanh

- **Stack cố định dùng được ngay.** Mọi module đều có bản ổn định, yêu cầu Go >= 1.25. Đặt `go 1.27` trong `go.mod` (Go mới nhất là 1.27.1 [S2]).
- **Rủi ro lớn nhất: MinIO server đã bị archive** ngày 2026-04-25, README ghi "NO LONGER MAINTAINED" [S3][S4]. Vẫn dùng client `minio-go` (còn phát hành), nhưng server tự host nên chọn Garage, SeaweedFS hoặc RustFS. Xem mục 6.
- **sqlx đứng yên từ 2024** (v1.4.0, ~400 issue mở) [S5]. Chấp nhận được vì API nhỏ và ổn định, nhưng đừng phụ thuộc tính năng mới.
- Khuyến nghị theo chủ đề: `caarlos0/env`, `alexedwards/argon2id` với tham số OWASP, transaction "UpdateFn" + transaction provider, trigger bất biến với SQLSTATE riêng, outbox bằng `SKIP LOCKED` có lease, session lưu DB + `http.CrossOriginProtection`, video qua presigned GET.

## 1. Phiên bản và module path

| Mục đích | Module path | Bản | Phát hành | Ghi chú |
|---|---|---|---|---|
| Go toolchain | go | 1.27.1 | 2026 | [S2] |
| HTTP | `github.com/gin-gonic/gin` | v1.12.0 | 2026-02 | cần go 1.25 [S6] |
| SQL helper | `github.com/jmoiron/sqlx` | v1.4.0 | 2024-04 | không còn commit mới [S5] |
| Driver | `github.com/jackc/pgx/v5` (+ `/stdlib`, `/pgconn`) | v5.11.0 | 2026-09 | [S7] |
| Migration | `github.com/golang-migrate/migrate/v4` | v4.20.1 | 2026-09 | driver `.../database/pgx/v5`, source `.../source/iofs` [S8] |
| CLI | `github.com/spf13/cobra` | v1.10.2 | 2025-12 | |
| Config (chọn) | `github.com/caarlos0/env/v11` | v11.4.1 | 2026-05 | không dùng viper v1.21.0 |
| Hash mật khẩu (chọn) | `github.com/alexedwards/argon2id` | v1.0.0 | 2023-10 | bọc `golang.org/x/crypto/argon2` v0.57.0 |
| Markdown | `github.com/yuin/goldmark` | v1.8.6 | 2026-09 | CommonMark |
| Sanitizer | `github.com/microcosm-cc/bluemonday` | v1.0.27 | 2024-07 | ít hoạt động, vẫn là chuẩn de facto |
| S3 client (chọn) | `github.com/minio/minio-go/v7` | v7.3.0 | 2026-08 | thay thế: `aws-sdk-go-v2/service/s3` v1.114.0 |
| Validate | `github.com/go-playground/validator/v10` | v10.30.5 | 2026-09 | Gin dùng sẵn qua `binding` |
| Rate limit | `golang.org/x/time/rate` | v0.16.0 | 2026-08 | chỉ cho giới hạn in-process |
| Test | `github.com/stretchr/testify` | v1.12.1 | 2026-08 | |
| Lint | `github.com/golangci/golangci-lint/v2` | v2.14.0 | 2026-09 | config v2 (`version: "2"`) |
| SMTP (chọn) | `github.com/wneessen/go-mail` | v0.8.1 | 2026-07 | xem mục 4 |
| Postgres | server | 18.6 mới nhất, 16 vẫn hỗ trợ | | [S9] |

Ghi chú tích hợp:

- **sqlx + pgx**: mở bằng `stdlib.OpenDB(*pgx.ConnConfig)` rồi `sqlx.NewDb(db, "pgx")`. Tên driver phải là `"pgx"` vì sqlx chỉ map tên này sang placeholder `$1` [S10].
- **migrate nhúng SQL**: `//go:embed migrations/*.sql`, `iofs.New(fs, "migrations")`, rồi `pgx.WithInstance(sqlDB, &pgx.Config{})` và `migrate.NewWithInstance("iofs", src, "pgx5", drv)`. Cách này tái dùng `*sql.DB`, tránh phải đổi DSN sang scheme `pgx5://` [S8].
- **Config**: `caarlos0/env` đọc struct tag `env:"DATABASE_URL,required"`, không phụ thuộc phụ. Viper hướng tới file + nhiều nguồn, quá nặng cho cấu hình chỉ qua env.
- **argon2id**: `DefaultParams` của thư viện là 64 MiB, t=1, p=NumCPU [S11]. Đặt tường minh theo OWASP tối thiểu m=19 MiB, t=2, p=1 [S12] để chi phí CPU ổn định giữa các máy. Thư viện lo encode PHC và so sánh constant-time, nên không cần tự viết.
- **goldmark + bluemonday**: render Markdown với goldmark (tắt `html.WithUnsafe`), sau đó luôn qua `bluemonday.UGCPolicy()`. Render khi lưu bản published và lưu HTML đã sanitize, không render mỗi request.
- **Rate limit**: `x/time/rate` chỉ hợp cho chống spam API theo process. Throttle đăng nhập phải lưu DB (mục 5).

## 2. Bố cục DDD-lite với sqlx

```text
cmd/lms/main.go                  # cobra root: serve, migrate, seed, healthcheck, worker
internal/domain/<aggregate>/     # entity, value object, lỗi domain, interface Repository
internal/app/<usecase>/          # application service, nhận interface, không import sqlx
internal/infra/postgres/         # repo sqlx, TxProvider, map lỗi pg
internal/infra/{mail,storage}/   # go-mail, minio-go
internal/http/                   # handler Gin, middleware, DTO
migrations/                      # *.up.sql / *.down.sql, embed
```

- **Entity**: struct có field unexported, tạo qua constructor kiểm tra bất biến, thay đổi qua method (`course.Publish(now)`). Repo dùng hàm `Unmarshal...FromDB` riêng để dựng lại mà không chạy validate nghiệp vụ, như Wild Workouts [S13].
- **Value object**: kiểu nhỏ bất biến có constructor trả lỗi, ví dụ `NewEmail(s)` chuẩn hóa lowercase, `VersionNo` là `int` > 0.
- **Enum trạng thái**: `type Status string` + bảng chuyển trạng thái `map[Status][]Status` và method `CanTransitionTo`. DB vẫn giữ `CHECK (status IN (...))` làm lưới an toàn.
- **Repository**: một interface cho mỗi aggregate, khai báo trong package domain, triển khai trong `infra/postgres`. Repo nhận `sqlx.ExtContext`, nên cùng code chạy được với `*sqlx.DB` lẫn `*sqlx.Tx`.
- **Transaction (khuyến nghị)**: dùng mẫu `Update(ctx, id, func(*Agg) error) error` cho thao tác trên một aggregate. Three Dots Labs gọi đây là giải pháp mặc định của họ [S14].
- **Nhiều aggregate trong một tx** (ví dụ ghi user + outbox): dùng transaction provider `Transact(ctx, func(r Repos) error)`. Hàm này `BeginTxx`, dựng repo trên tx, commit hoặc rollback. Three Dots Labs khuyên chỉ dùng khi cần [S14].
- **Không giấu `*sqlx.Tx` trong `context`.** Cách này ngầm định và dễ chạy nhầm ngoài tx; truyền qua provider thì tường minh và dễ test.
- Ben Johnson khuyên gom kiểu domain ở gốc và gom theo dependency [S15]. Hướng dẫn chính thức của Go chỉ yêu cầu `cmd/` + `internal/` [S16]. Bố cục trên khớp cả hai mà không cần `pkg/`.

## 3. Postgres: bản published bất biến

```sql
CREATE TABLE course_versions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  course_id bigint NOT NULL REFERENCES courses(id) ON DELETE RESTRICT,
  version_no int NOT NULL,
  status text NOT NULL CHECK (status IN ('draft','published','archived')),
  UNIQUE (course_id, version_no)
);
CREATE UNIQUE INDEX uq_course_versions_one_draft
  ON course_versions (course_id) WHERE status = 'draft';

CREATE FUNCTION forbid_non_draft_change() RETURNS trigger AS $$
BEGIN
  IF OLD.status <> 'draft'
     AND NOT (TG_OP = 'UPDATE' AND OLD.status = 'published' AND NEW.status = 'archived') THEN
    RAISE EXCEPTION 'version % is immutable', OLD.id
      USING ERRCODE = 'LMS01', CONSTRAINT = 'course_version_immutable';
  END IF;
  RETURN COALESCE(NEW, OLD);
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_course_versions_immutable
  BEFORE UPDATE OR DELETE ON course_versions
  FOR EACH ROW EXECUTE FUNCTION forbid_non_draft_change();
```

- **Bảng con** (lesson, quiz của một version) cần trigger tương tự. Trigger đó tra `status` của version cha qua `version_id`. Nhớ chặn cả INSERT vào version không còn là draft.
- **Partial unique index** đảm bảo mỗi course chỉ có một draft. Vi phạm trả `23505` kèm tên index.
- **`ON DELETE RESTRICT`** chặn xóa cha còn con. Vi phạm trả `23503`.
- `RAISE ... USING ERRCODE, CONSTRAINT` là cú pháp chuẩn của PL/pgSQL [S17]. SQLSTATE tự định nghĩa phải gồm 5 ký tự và không kết thúc bằng `000`.
- **Map sang lỗi ứng dụng** ở một chỗ duy nhất trong `infra/postgres`. Dùng `errors.As(err, &pgErr)` với `*pgconn.PgError`, rồi xét `Code` và `ConstraintName` [S18]. Lỗi đi qua `database/sql` vẫn giữ kiểu này. Dùng hằng số từ `github.com/jackc/pgerrcode` [S19].

| Code + constraint | Lỗi domain | HTTP |
|---|---|---|
| `LMS01` / `course_version_immutable` | `ErrVersionImmutable` | 409 |
| `23505` / `uq_course_versions_one_draft` | `ErrDraftExists` | 409 |
| `23503` (RESTRICT) | `ErrInUse` | 409 |

## 4. Outbox email trong Postgres

Ghi job vào bảng `email_outbox` trong cùng transaction với thay đổi nghiệp vụ. Worker (`lms worker`) gửi mail tách khỏi tx.

```sql
CREATE TABLE email_outbox (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  to_addr text NOT NULL, subject text NOT NULL, body_html text NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed')),
  attempts int NOT NULL DEFAULT 0,
  run_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz, last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_email_outbox_due ON email_outbox (run_at) WHERE status IN ('pending','sending');

-- claim: tx ngắn, lease 5 phút để worker chết thì job được nhận lại
UPDATE email_outbox SET status='sending', attempts=attempts+1, locked_until=now()+interval '5 minutes'
WHERE id IN (
  SELECT id FROM email_outbox
  WHERE (status='pending' AND run_at<=now()) OR (status='sending' AND locked_until<now())
  ORDER BY run_at FOR UPDATE SKIP LOCKED LIMIT 10)
RETURNING *;
```

- `FOR UPDATE SKIP LOCKED` cho phép nhiều worker chạy song song mà không nhận trùng job [S20].
- **Sau khi gửi**: thành công thì `status='sent'`. Thất bại khi `attempts < 3` thì `status='pending'` và `run_at = now() + 2^attempts phút` (2, 4 phút). Đến lần thứ 3 thì `status='failed'` và lưu `last_error`.
- **Worker loop**: `time.NewTicker(5s)`, `select` giữa `ctx.Done()` và `ticker.C`, mỗi tick claim và gửi tuần tự. Dừng gọn qua `signal.NotifyContext(ctx, SIGINT, SIGTERM)`.
- Delivery là at-least-once vì worker có thể chết sau khi gửi nhưng trước khi cập nhật. Với email MVP điều này chấp nhận được.
- **SMTP: chọn `wneessen/go-mail`.** Thư viện hỗ trợ STARTTLS/SMTPS, auth, multipart HTML + text, context và timeout. `net/smtp` đã "frozen", không nhận tính năng mới [S21][S22].
- **Dev**: Mailpit, SMTP cổng 1025 và web UI cổng 8025 [S23].

## 5. Session và auth cho SPA cùng origin

**Khuyến nghị: session opaque lưu trong Postgres.** Không dùng cookie ký stateless.

| Tiêu chí | Session lưu DB | Cookie ký stateless |
|---|---|---|
| Thu hồi (logout, đổi mật khẩu, khóa user) | Tức thì, xóa dòng | Không làm được nếu không có denylist |
| Cưỡng chế `must_change_password` | Đọc từ DB mỗi request | Cờ cũ nằm trong cookie |
| Chi phí | 1 query mỗi request | 0 query |

- **Cookie**: tên `__Host-sid`, `HttpOnly; Secure; SameSite=Lax; Path=/`. Giá trị là 32 byte ngẫu nhiên, DB chỉ lưu SHA-256 của token.
- **Bảng `sessions`**: `token_hash`, `user_id`, `expires_at`, `last_seen_at`. Đổi token khi đăng nhập. Xóa mọi session khác khi đổi mật khẩu [S24].
- **CSRF**: bọc engine Gin bằng `http.CrossOriginProtection` của Go 1.25+. Nó chặn request không an toàn đến từ origin khác dựa trên `Sec-Fetch-Site`/`Origin` [S25].
- Thêm lớp phòng thủ bằng header tùy chỉnh bắt buộc cho POST/PUT/PATCH/DELETE, ví dụ `X-Requested-With`. OWASP liệt kê kỹ thuật này [S26]. SameSite=Lax đã chặn phần lớn trường hợp còn lại.
- **Throttle đăng nhập**: bảng `login_attempts(email_norm, ip, succeeded, at)`. Nếu có >= 5 lần thất bại trong 15 phút cho email đó thì trả `429` kèm `Retry-After`.
- Kiểm tra throttle trước khi verify argon2 để tiết kiệm CPU. Xóa các dòng cũ trong job dọn dẹp.
- **`must_change_password`**: middleware chạy sau auth. Nếu cờ bật và route không thuộc allowlist (`POST /auth/change-password`, `POST /auth/logout`, `GET /auth/me`) thì trả `403 {"code":"PASSWORD_CHANGE_REQUIRED"}`. SPA bắt mã này để chuyển trang.

## 6. Phát video: presigned URL và HTTP Range

**Khuyến nghị cho MVP: presigned GET từ object storage.** API chỉ cấp URL sau khi kiểm tra quyền.

| Tiêu chí | Presigned GET | Proxy qua API Go |
|---|---|---|
| Range/seek | Storage S3 tự xử lý | Được qua `http.ServeContent`, vì object minio-go là `ReadSeeker` |
| Tải cho API | Không | Toàn bộ băng thông và kết nối đi qua Go |
| Kiểm soát truy cập | Tại thời điểm cấp URL | Mỗi request |
| CORS | Có thể cần (xem dưới) | Không, vì cùng origin |

- `PresignedGetObject(ctx, bucket, key, expires, params)` và `PresignedPutObject` có sẵn trong minio-go [S27].
- **Bẫy expiry**: mỗi lần seek là một request Range mới dùng cùng URL. URL hết hạn giữa chừng thì seek lỗi 403. Đặt expiry bằng độ dài video + biên, ví dụ 2 giờ, hoặc để player xin URL mới khi gặp 403.
- **CORS khi phát**: `<video src>` không có thuộc tính `crossorigin` thì tải media không cần CORS. Chỉ cần CORS khi dùng `fetch`, MSE hoặc hls.js.
- **CORS khi upload**: upload trực tiếp bằng presigned PUT từ trình duyệt bắt buộc có CORS trên storage. Cần cho phép `PUT`, header `Content-Type`, và expose `ETag`.
- **Host khi ký**: chữ ký gắn với host. Client presign phải được cấu hình bằng endpoint công khai mà trình duyệt thấy, không phải hostname nội bộ của Docker.
- **Chọn server storage**: MinIO community đã bị archive [S3][S4]. Lựa chọn còn hoạt động:
  - **Garage**: một binary Rust, nhẹ, được khuyên nhiều nhất cho self-host nhỏ [S28].
  - **SeaweedFS**: Apache-2.0, trưởng thành hơn 10 năm [S29].
  - **RustFS**: Apache-2.0, tương thích mô hình MinIO, còn mới [S28].
  - Cả ba đều nói S3 nên `minio-go` dùng được. Cần kiểm thử presign + Range + CORS trên server được chọn, vì độ phủ API S3 khác nhau.
- **Vì sao chọn minio-go thay aws-sdk-go-v2**: API presign gọn hơn, chỉ có một module, và tương thích mọi server S3. Nên bọc sau interface `Storage` (`PresignGet`, `PresignPut`) để đổi SDK rẻ.

## Hạn chế của nghiên cứu

- Phiên bản là "latest" trên proxy tại 2026-10-05. Chưa build thử một `go.mod` tổng hợp để kiểm tra xung đột dependency.
- Chưa kiểm thử thực tế presign, Range và CORS trên Garage, SeaweedFS hay RustFS. Đây là điểm cần spike sớm.
- Tham số argon2 chưa được benchmark trên phần cứng đích.

## Câu hỏi chưa giải quyết

1. Server S3 tự host nào thay MinIO? Hay dùng AIStor Free (giấy phép riêng của MinIO Inc.)?
2. Video là file MP4 progressive hay cần HLS? HLS kéo theo yêu cầu CORS và ký nhiều segment.
3. Throttle đăng nhập chỉ theo email, hay theo cả email và IP để tránh khóa tài khoản người khác?

## Nguồn

- [S1] https://proxy.golang.org (truy vấn `/<module>/@latest`, 2026-10-05)
- [S2] https://go.dev/dl/?mode=json
- [S3] https://github.com/minio/minio (archived, README "NO LONGER MAINTAINED")
- [S4] https://pinggy.io/blog/minio_archived_self_hosted_s3_alternatives/
- [S5] https://github.com/jmoiron/sqlx
- [S6] https://github.com/gin-gonic/gin/blob/v1.12.0/go.mod
- [S7] https://github.com/jackc/pgx
- [S8] https://github.com/golang-migrate/migrate/tree/master/database/pgx/v5 , https://pkg.go.dev/github.com/golang-migrate/migrate/v4/source/iofs
- [S9] https://www.postgresql.org/support/versioning/
- [S10] https://github.com/jmoiron/sqlx/blob/v1.4.0/bind.go
- [S11] https://github.com/alexedwards/argon2id
- [S12] https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- [S13] https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example , https://threedots.tech/post/repository-pattern-in-go/
- [S14] https://threedots.tech/post/database-transactions-in-go/
- [S15] https://www.gobeyond.dev/standard-package-layout/
- [S16] https://go.dev/doc/modules/layout
- [S17] https://www.postgresql.org/docs/16/plpgsql-errors-and-messages.html
- [S18] https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError
- [S19] https://github.com/jackc/pgerrcode
- [S20] https://www.postgresql.org/docs/16/sql-select.html#SQL-FOR-UPDATE-SHARE
- [S21] https://github.com/wneessen/go-mail
- [S22] https://pkg.go.dev/net/smtp
- [S23] https://mailpit.axllent.org/
- [S24] https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- [S25] https://go.dev/doc/go1.25 (net/http `CrossOriginProtection`)
- [S26] https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- [S27] https://github.com/minio/minio-go/blob/v7.3.0/api-presigned.go
- [S28] https://wz-it.com/en/blog/minio-successor-s3-storage-comparison/ , https://www.xda-developers.com/garage-best-self-hosted-alternative-to-minio/
- [S29] https://github.com/seaweedfs/seaweedfs
