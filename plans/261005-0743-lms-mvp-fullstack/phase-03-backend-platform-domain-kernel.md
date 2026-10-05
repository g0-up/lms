---
phase: 03
title: "Phase 3: Platform backend và shared kernel"
status: pending
priority: P1
effort: "1.5 ngày"
dependencies: [2]
---

# Phase 3: Platform backend và shared kernel

## Goal

Hoàn thiện tầng `internal/platform/*` và `internal/domain` để các feature (Phase 04–09) chỉ còn viết handler, service, repository. Sau phase này: value object dùng chung có kiểm tra và thông điệp tiếng Việt; một kiểu lỗi ứng dụng duy nhất ánh xạ sang HTTP; lỗi Postgres (23505, 23503, 23514) được dịch ở **một chỗ** thành mã lỗi của [plan.md §7](./plan.md); transaction, clock, id, audit, middleware, hộp mã hóa bí mật outbox (`platform/secretbox`) và test DB sẵn dùng; `router.go` của Phase 01 chuyển sang dùng các package này. Tên constraint chỉ được tham chiếu qua hằng trong `platform/db/pgerr/constraints.go` (Phase 02). Gói `db` cung cấp `ExecAffectOne`/`ErrNoRowsAffected`, nền cho hợp đồng "Bất biến phiên bản ở tầng ứng dụng" của Phase 02 (DB không có trigger, D1). <!-- Red Team: RT-11 - dùng sổ tên constraint --> <!-- Updated: Validation Session 1 - V1 secretbox --> <!-- Updated: Session 2 - D1 -->

## Context & Requirements

- Mẫu tham chiếu: `~/Documents/personal-workspace/sidecup/apps/api/internal/platform/{config,db/testdb}`; sidecup dùng GORM, ở đây thay bằng sqlx + pgx stdlib theo quyết định người dùng.
- Thông điệp lỗi tiếng Việt lấy từ prototype (`prototype/app.js`): "Phiên bản đã phát hành không thể sửa", "Đã có bản nháp, hãy tiếp tục chỉnh sửa bản nháp đó", "Không thể xóa vì đang được sử dụng", "Email không hợp lệ", "Mật khẩu tạm đã hết hạn, liên hệ quản trị để được cấp lại", "Tài khoản đã bị vô hiệu hóa", "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút." (FR-05). <!-- Updated: Validation Session 1 - V2 thông điệp khóa đăng nhập -->
- Thư viện: gin v1.12.0, sqlx v1.4.0, pgx v5.11.0 (`pgconn.PgError`, `pgerrcode`), testify v1.12.1, `golang.org/x/time/rate` v0.16.0, `github.com/google/uuid` (v7), `log/slog` chuẩn.
- Ràng buộc: `domain` không import gì ngoài stdlib; `platform/*` không import `features/*`; `platform/db/pgerr` là nơi duy nhất dịch lỗi Postgres và chỉ dùng hằng tên constraint trong `pgerr/constraints.go` do Phase 02 sinh (không viết chuỗi `uq_*`/`ck_*`/`fk_*` ở bất kỳ feature nào). <!-- Red Team: RT-11 - sổ tên constraint -->
- Cấu hình chỉ qua struct `config.Config` của Phase 01 với đúng tên trường (`TrustedProxies`, `RateLimitLoginIPPerMin`, `RateLimitForgotIPPerMin`, `OutboxSecretKey`, `AppTZ`...); phase này không thêm biến env mới. <!-- Red Team: RT-11 - Config một nguồn -->
- Bí mật của `email_outbox` (mật khẩu tạm, token đặt lại) được mã hóa AES-256-GCM trong `secret_enc` bằng `OUTBOX_SECRET_KEY`; gói `platform/secretbox` ở phase này cung cấp `Seal`/`Open`, Phase 04 (worker, identity) và Phase 07 (invitations) chỉ gọi. <!-- Updated: Validation Session 1 - V1 secretbox -->

## Architecture / Design

### `internal/domain` (shared kernel, chỉ stdlib)

| Kiểu | Hành vi |
|---|---|
| `Email` | `ParseEmail(s) (Email, error)`: trim, lowercase, kiểm `^[^@\s]+@[^@\s]+\.[^@\s]+$`, ≤ 254 ký tự; `String()`; `Display` giữ nguyên input đã trim cho cột `email`. Lỗi `ErrInvalidEmail` "Email không hợp lệ" |
| `Code` | `ParseCode(s, kind)` với `kind` ∈ `StageCode|CourseCode|ClassCode`; regex trùng CHECK ở Phase 02 (`^[A-Z0-9][A-Z0-9_-]{1,19}$` cho stage/course, `^[a-z0-9][a-z0-9-]{1,29}$` cho class) |
| `LessonKey` | `ParseLessonKey` regex `^[a-z0-9][a-z0-9-]{1,39}$`; `Slugify(title)` sinh key gợi ý từ tiêu đề tiếng Việt (bỏ dấu, thay khoảng trắng bằng `-`) |
| `VersionNo` | `int` ≥ 1; `Next()` |
| `VersionStatus` | `Draft|Published|Archived`; `transitions = {Draft:{Published}, Published:{Archived}, Archived:{}}`; `CanTransitionTo(to) bool`; `Transition(to) error` trả `ErrInvalidTransition` kèm thông điệp: Draft→Archived "Chỉ phiên bản đã phát hành mới có thể lưu trữ"; Published→Draft "Phiên bản đã phát hành không thể sửa"; Archived→* "Phiên bản đã lưu trữ không thể thay đổi"; `IsMutable()` = Draft |
| `ClassStatus` | `Draft|Active|Ended` (chuỗi `draft|active|ended` khớp `ck_classes_status`); transitions `Draft→Active`, `Active→Ended`; thông điệp "Lớp đã kết thúc không thể thay đổi", "Chỉ lớp nháp mới có thể kích hoạt" <!-- Red Team: RT-03 - Closed → Ended --> |
| `Role` | `Admin|Teacher|Student`; `ParseRole`; `Can(action)` **không** ở đây (authz thuộc feature identity) |
| `UserStatus` | `Invited|Active|Disabled`; `Invited→Active` (đổi mật khẩu lần đầu), `Invited|Active→Disabled`; `Enable(neverLoggedIn bool)`: `Disabled→Invited` khi người dùng chưa đăng nhập lần nào (còn dùng mật khẩu tạm), ngược lại `Disabled→Active`; thông điệp "Tài khoản đã bị vô hiệu hóa" <!-- Red Team: RT-09 - chuyển trạng thái disabled --> |
| `MemberStatus` | `Active|Dropped|Completed` (khớp `ck_class_members_status`; MVP không có endpoint ghi `Completed`) <!-- Updated: Validation Session 1 - V3 --> |
| `Percent` | `Percent(done, total int) int`: `total==0 → 0`, ngược lại `int(math.Round(float64(done)*100/float64(total)))` rồi clamp 0..100; làm tròn nửa-ra-xa-0 giống `round(numeric)` của Postgres nên báo cáo tính ở SQL và ở Go cho cùng số <!-- Red Team: RT-12 - công thức Percent --> |
| `Password` | `ValidatePassword(s, minLen)`: độ dài ≥ minLen, không toàn khoảng trắng, ≤ 128 byte; thông điệp "Mật khẩu phải có ít nhất %d ký tự" |

Lỗi domain: `errors.go` định nghĩa `type Error struct{ Kind Kind; Msg string }` với `Kind` ∈ `Invalid|Transition|NotFound|Conflict|Forbidden|Immutable|InUse`; sentinel `ErrInvalidEmail`, `ErrInvalidTransition`, `ErrNotFound`, … dùng `errors.Is`. Domain **không** biết HTTP status.

### `platform/apperr`

```go
type Error struct {
  Status  int               // HTTP
  Code    string            // plan.md §7
  Message string            // tiếng Việt
  Details map[string]string // field -> thông điệp, cho VALIDATION_FAILED
  cause   error
}
func (e *Error) Error() string; Unwrap() error
func New(status int, code, msg string) *Error
func Wrap(cause error, status int, code, msg string) *Error
func Validation(details map[string]string) *Error              // 400 VALIDATION_FAILED "Dữ liệu không hợp lệ"
func NotFound(what string) *Error                              // 404 NOT_FOUND "Không tìm thấy <what>"
func Forbidden() *Error                                        // 403
func Unauthenticated() *Error                                  // 401
func Conflict(msg string) *Error                               // 409 CONFLICT
func FromDomain(err error) *Error                              // map domain.Kind -> status/code; không match -> Internal
func Internal(cause error) *Error                              // 500 INTERNAL "Lỗi hệ thống, vui lòng thử lại"
func Is(err error, code string) bool
```

Bảng `FromDomain`: `Invalid→400 VALIDATION_FAILED`, `Transition→409 INVALID_TRANSITION`, `NotFound→404`, `Conflict→409 CONFLICT`, `Forbidden→403`, `Immutable→409 VERSION_IMMUTABLE`, `InUse→409 IN_USE`. Message lấy từ `domain.Error.Msg` khi có.

Hằng mã lỗi: `codes.go` liệt kê đủ 14 mã của plan.md §7 (`CodeValidationFailed = "VALIDATION_FAILED"`, …) kèm thông điệp mặc định tiếng Việt.

### `platform/db`

```go
func Open(ctx context.Context, dsn string) (*sqlx.DB, error)   // Phase 01, giữ nguyên
type Executor interface { sqlx.ExtContext; sqlx.PreparerContext }  // *sqlx.DB và *sqlx.Tx đều thỏa
func Transact(ctx context.Context, db *sqlx.DB, fn func(tx Executor) error) (err error)

var ErrNoRowsAffected = errors.New("db: no rows affected")
// ExecAffectOne chạy câu lệnh và trả ErrNoRowsAffected khi RowsAffected() != 1 (lỗi khác trả nguyên).
// Repository của aggregate có phiên bản bắt buộc dùng cho mọi UPDATE/DELETE/INSERT...SELECT có điều kiện trạng thái
// (Phase 02 mục "Bất biến phiên bản ở tầng ứng dụng"); caller dịch ErrNoRowsAffected thành ErrVersionImmutable / ErrInvalidTransition.
func ExecAffectOne(ctx context.Context, ex Executor, query string, args ...any) error   // Updated: Session 2 - D1
```

`Transact`: `BeginTxx`; `defer` xử lý `recover()` → rollback rồi `panic` lại; `fn` lỗi → rollback (join lỗi rollback nếu có bằng `errors.Join`); thành công → `Commit`. Có `TransactWithOptions(ctx, db, *sql.TxOptions, fn)` cho `Serializable` khi cần (clone version).

`platform/db/pgerr/map.go` (cùng gói với `constraints.go` của Phase 02; gói `pgerr` import `apperr`, không import `db` để tránh vòng): <!-- Red Team: RT-11 - pgerr thành gói con dùng hằng -->

```go
// Map dịch lỗi Postgres sang apperr. Nơi DUY NHẤT dịch tên constraint; tên lấy từ constraints.go.
func Map(err error) error {
  var pg *pgconn.PgError
  if !errors.As(err, &pg) { return err }
  switch pg.Code {   // không có mã lỗi tự định nghĩa: DB không có function/trigger (Phase 02 D1)
  case pgerrcode.UniqueViolation:
    switch pg.ConstraintName {
    case UqStageVersionsOneDraft, UqCourseVersionsOneDraft:
      return apperr.Wrap(err, 409, apperr.CodeDraftExists, "Đã có bản nháp, hãy tiếp tục chỉnh sửa bản nháp đó")
    case UqUsersEmailNormalized:
      return apperr.Wrap(err, 409, apperr.CodeConflict, "Email đã được sử dụng")
    case UqStagesCode, UqCoursesCode, UqClassesCode:
      return apperr.Wrap(err, 409, apperr.CodeConflict, "Mã đã tồn tại")
    case UqClassMembersClassUser:
      return apperr.Wrap(err, 409, apperr.CodeConflict, "Học viên đã có trong lớp")
    case UqLessonsVersionKey, UqLessonsVersionPosition, UqCvsVersionStage, UqCvsVersionPosition:
      return apperr.Wrap(err, 409, apperr.CodeConflict, "Dữ liệu bị trùng trong phiên bản")
    default:
      return apperr.Wrap(err, 409, CodeConflict, "Dữ liệu bị trùng")
    }
  case pgerrcode.ForeignKeyViolation:
    return apperr.Wrap(err, 409, CodeInUse, "Không thể xóa vì đang được sử dụng")
  case pgerrcode.CheckViolation:
    return apperr.Wrap(err, 400, CodeValidationFailed, checkMessage(pg.ConstraintName))
  case pgerrcode.SerializationFailure, pgerrcode.DeadlockDetected:
    return apperr.Wrap(err, 409, CodeConflict, "Xung đột dữ liệu, vui lòng thử lại")
  }
  return err
}
```

`checkMessage` map vài CHECK có nghĩa với người dùng (`CkClassesDates` → "Ngày kết thúc phải sau ngày bắt đầu", `CkLessonsTypeContent` → "Bài học video cần tệp video, bài đọc cần nội dung Markdown", `CkLessonsDuration` → "Thời lượng không hợp lệ", `CkUsersEmailNormalized` → "Email không hợp lệ"), còn lại "Dữ liệu không hợp lệ". <!-- Red Team: RT-07/RT-11 --> FK violation khi **insert** (tham chiếu không tồn tại, `pg.Detail` chứa "is not present") map sang `404 NOT_FOUND` thay vì IN_USE; phân biệt bằng `strings.Contains(pg.Detail, "is not present in table")`.

Repository của feature gọi `return pgerr.Map(err)` ở mọi điểm trả lỗi từ sqlx và dùng `pgerr.UqLessonsVersionPosition`/`pgerr.UqCvsVersionPosition` khi viết `SET CONSTRAINTS ... DEFERRED`. <!-- Red Team: RT-11 --> `sql.ErrNoRows` → `apperr.NotFound` làm ở repository vì chỉ nó biết "cái gì" không tìm thấy.

### `platform/httpx`

- `BindJSON(c *gin.Context, dst any) error`: `http.MaxBytesReader(64<<10)`, `json.NewDecoder` + `DisallowUnknownFields`, lỗi → `apperr.Validation({"body": "JSON không hợp lệ"})`; sau decode gọi `validate.Struct(dst)` (validator v10.30.5) và chuyển `FieldError` → `Details[jsonFieldName]` với thông điệp tiếng Việt theo tag (`required` "Bắt buộc", `email`, `min`, `max`, `oneof`).
- `OK(c, status, data any)` → `c.JSON(status, data)`; `NoContent(c)`.
- `Fail(c, err error)`: `apperr.Error` → envelope `{"error":{"code","message","details"}}` đúng status; lỗi khác → log `ERROR` kèm `request_id` và trả 500 `INTERNAL`; `429` thêm header `Retry-After` nếu `Details["retry_after"]`.
- `Param UUID`: `UUIDParam(c, "id") (uuid.UUID, error)` → `NotFound` khi sai định dạng (không lộ 400 khác với 404 để không gợi ý id hợp lệ).
- Không có `Paginate`: mọi danh sách MVP trả toàn bộ (quy mô pilot), đúng spec. <!-- Updated: Validation Session 1 - V7 bỏ httpx.Paginate -->

### `platform/middleware`

| Middleware | Hành vi |
|---|---|
| `RequestID()` | đọc `X-Request-ID` (≤ 64 ký tự, `[A-Za-z0-9-]`) hoặc sinh uuid v7; set header response + `c.Set("request_id")` + đưa vào `context` qua `slog` attr |
| `Logger(l *slog.Logger)` | JSON: `method, path (route pattern, không query), status, latency_ms, request_id, user_id (nếu có), ip`; không log body, không log query string, không log header |
| `Recover(l)` | bắt panic → log stack, `httpx.Fail(c, apperr.Internal(...))`; `c.Abort()` |
| `SecurityHeaders()` | `X-Content-Type-Options: nosniff`, `Cache-Control: no-store` cho `/api/*`, `X-Frame-Options: DENY`; CSP/HSTS để Caddy |
| `RateLimit(key func(*gin.Context) string, perMin int, enabled bool)` | `x/time/rate` theo key (IP cho `/auth/login` với `cfg.RateLimitLoginIPPerMin=300`, `/auth/forgot-password` với `cfg.RateLimitForgotIPPerMin=30`, user id cho `/media/*` 120/phút ở Phase 05), burst = perMin, map `sync.Map` + dọn entry cũ mỗi 10 phút; vượt → `429 RATE_LIMITED` "Bạn thao tác quá nhanh, thử lại sau" với `Retry-After`. `enabled=false` khi `cfg.IsE2E()` → middleware pass-through. Khóa đăng nhập theo email (5 lần/15 phút) KHÔNG ở đây mà ở Phase 04 qua bảng `login_attempts`; không khóa theo IP <!-- Updated: Validation Session 1 - V2 limiter theo IP, tắt ở e2e --> |
| `ClientIP()` | `engine.SetTrustedProxies(cfg.TrustedProxies)`; mặc định rỗng → `nil` (không tin `X-Forwarded-For`, dùng `RemoteAddr`); prod đặt CIDR của Caddy/Traefik <!-- Red Team: RT-11 - TRUSTED_PROXIES mặc định rỗng --> |

`Auth`/`RequireRole`/`CSRF` thuộc Phase 04 (identity) vì cần bảng `sessions`. Quy tắc CSRF chuẩn mà Phase 04 cài và Phase 10 tuân theo: mọi POST/PUT/PATCH/DELETE dưới `/api/v1` phải qua CẢ `http.CrossOriginProtection` (Sec-Fetch-Site/Origin) VÀ có header `X-Requested-With: fetch`; thiếu một trong hai → 403 `FORBIDDEN`. PUT thẳng lên MinIO/R2 không đi qua API nên không cần header. <!-- Red Team: RT-05 - CSRF hai lớp -->

### `platform/secretbox`

```go
type Box struct{ aead cipher.AEAD }
func New(key []byte) (*Box, error)              // key 32 byte (cfg.OutboxSecretKey đã decode base64); AES-256-GCM
func (b *Box) Seal(plaintext []byte) ([]byte, error)   // nonce 12 byte ngẫu nhiên || ciphertext+tag; lưu vào email_outbox.secret_enc
func (b *Box) Open(sealed []byte) ([]byte, error)      // sai khóa/bị sửa → ErrSecretInvalid
```

Không log plaintext, không có `String()`; worker gọi `Open` ngay trước khi render email rồi bỏ kết quả. Test: round-trip; đổi 1 byte → lỗi; khóa khác → lỗi; hai lần `Seal` cùng plaintext cho ciphertext khác nhau (nonce). <!-- Updated: Validation Session 1 - V1 gói secretbox -->

### `platform/clock`, `platform/ids`, `platform/audit`, `platform/testdb`

```go
// clock
type Clock interface { Now() time.Time }
type Real struct{ Loc *time.Location }   // Now() = time.Now().In(Loc)
type Fake struct{ T time.Time }          // Set(t), Advance(d)

// ids
func New() uuid.UUID                      // uuid.Must(uuid.NewV7())
func Parse(s string) (uuid.UUID, error)

// audit
type Entry struct {
  ActorID    *uuid.UUID
  Action     string      // hằng trong actions.go, đúng danh sách plan.md §7
  TargetType string
  TargetID   *uuid.UUID
  Before, After any      // marshal jsonb; strip khóa nhạy cảm
  RequestID  string
}
type Recorder interface { Record(ctx context.Context, tx db.Executor, e Entry) error }
type PG struct{ Clock clock.Clock }       // INSERT INTO audit_logs ... dùng tx của caller
type Noop struct{}                        // cho unit test
```

`audit.PG.Record` chạy trong `Executor` của caller để audit và thay đổi nghiệp vụ cùng commit/rollback. `sanitize(v any) any`: marshal → unmarshal map → xóa đệ quy khóa `password_hash, token_hash, password, temp_password`. `actions.go`: đúng 20 hằng, chép nguyên văn tập chuẩn plan.md §7 (dạng quá khứ, chấm phân cách): `stage_version.published, stage_version.cloned, stage_version.archived, stage_version.deleted, course_version.published, course_version.cloned, course_version.archived, course_version.deleted, course.stage_version_applied, class.course_version_changed, class.activated, class.ended, class.member_invited, class.invitation_resent, class.member_dropped, user.disabled, user.enabled, stage.created, course.created, class.created`. Không dùng dạng lẫn thì (`user.invite`, `stage_version.publish`, `class.member_add`…); `actions_test.go` so tập hằng với danh sách trên để chặn thêm action ngoài plan.md §7. Phase này chỉ tạo file và hằng; Phase 09 ánh xạ sang nhãn tiếng Việt chép nguyên văn từ `prototype/app.js` ("Phát hành chặng", "Kích hoạt lớp", "Mời học viên"...). <!-- Red Team: RT-03/RT-07 - chuẩn hóa 20 action theo plan.md §7, bỏ dạng lẫn thì -->

`testdb` (`//go:build integration`):

```go
func Open(t testing.TB) *sqlx.DB   // skip nếu không có TEST_DATABASE_URL; once: migrate up; trả DB chung của package
func Reset(t testing.TB, db *sqlx.DB)  // TRUNCATE tất cả bảng (trừ schema_migrations) RESTART IDENTITY CASCADE
func Tx(t testing.TB, db *sqlx.DB) db.Executor   // mở tx, t.Cleanup rollback: test cô lập, nhanh
func Fixture(t, ex, name string)                 // chạy file SQL trong testdb/fixtures/<name>.sql, dữ liệu đặt theo thực thể của prototype/seed.js (ví dụ admin_quan_tran, stage_db_published)  // Red Team: RT-14
```

Mỗi package test có thể dùng DB riêng theo tên package (`lms_test_<pkg>`) như sidecup khi chạy `-p` song song; mặc định dùng `-p 1` cho tag integration qua Makefile để đơn giản.

### Nối vào `internal/app`

`router.go`: thay middleware inline của Phase 01 bằng `middleware.RequestID(), middleware.Logger(l), middleware.Recover(l), middleware.SecurityHeaders()`; `engine.NoRoute` → `httpx.Fail(c, apperr.NotFound("đường dẫn"))`; `engine.HandleMethodNotAllowed = true` → 405 envelope. `Deps` thêm `Clock clock.Clock`, `Audit audit.Recorder`, `IDs` không cần (package function). `/readyz` dùng `db.PingContext` với timeout 2s.

### Ghi chú SOLID

- **S**: `domain` chỉ quy tắc nghiệp vụ; `apperr` chỉ biểu diễn lỗi; `pgerr` chỉ dịch lỗi DB; `httpx` chỉ HTTP I/O.
- **O**: thêm mã lỗi hoặc constraint mới = thêm case trong `pgerr`/`codes.go`, không sửa handler.
- **L**: `*sqlx.DB` và `*sqlx.Tx` đều là `Executor`; repository không biết đang trong tx hay không.
- **I**: `Clock`, `Recorder`, `Executor` là interface nhỏ một hai method; feature chỉ phụ thuộc interface.
- **D**: feature nhận `Clock`, `Recorder`, `*sqlx.DB` qua constructor; test truyền `clock.Fake`, `audit.Noop`, `testdb.Tx`.

## Files to Create / Modify

```text
apps/api/internal/domain/{email,code,lesson_key,version,class_status,role,user_status,member_status,percent,password,errors}.go
apps/api/internal/domain/*_test.go                       # bảng test cho từng VO
apps/api/internal/platform/apperr/{apperr,codes,apperr_test}.go
apps/api/internal/platform/db/{db,migrate}.go            # giữ; thêm Executor
apps/api/internal/platform/db/{tx,tx_test}.go
apps/api/internal/platform/db/tx_integration_test.go     # //go:build integration
apps/api/internal/platform/db/{exec,exec_test}.go         # ExecAffectOne, ErrNoRowsAffected; exec_test //go:build integration  <!-- Updated: Session 2 - D1 -->
apps/api/internal/platform/db/pgerr/{map,map_test}.go    # cùng gói với constraints.go của Phase 02; map_integration_test.go (//go:build integration)  <!-- Red Team: RT-11 -->
apps/api/internal/platform/secretbox/{secretbox,secretbox_test}.go   # AES-256-GCM Seal/Open  <!-- Updated: Validation Session 1 - V1 -->
apps/api/internal/platform/httpx/{bind,respond,params,validate_vi,httpx_test}.go
apps/api/internal/platform/middleware/{request_id,logger,recover,security_headers,rate_limit,client_ip,middleware_test}.go
apps/api/internal/platform/clock/{clock,fake}.go
apps/api/internal/platform/ids/ids.go
apps/api/internal/platform/audit/{audit,actions,actions_test,pg,noop,sanitize,audit_test,pg_integration_test}.go   # actions_test chốt 20 action plan.md §7  <!-- Red Team: RT-03/RT-07 -->
apps/api/internal/platform/testdb/{testdb,fixtures}.go + fixtures/{admin_quan_tran,teacher_huong_le,student_an_nguyen,stage_db_published,stage_db_draft,course_basic_published,class_basic01_active}.sql   # tên theo seed.js  <!-- Red Team: RT-14 -->
apps/api/internal/app/{router,deps}.go                   # sửa: dùng platform
apps/api/cmd/lms/serve.go                                # sửa: tạo clock.Real, audit.PG, truyền Deps
apps/api/Makefile                                        # test-integration thêm -p 1
docs/architecture.md                                     # sửa (file của Phase 01): thêm mục Backend — layout package, luồng lỗi, quy ước repository/service/handler, bất biến thứ tự ghi  <!-- Updated: Validation Session 1 - V7 gộp vào docs/architecture.md -->
```

Thêm module: `github.com/google/uuid`, `golang.org/x/time@v0.16.0`, `github.com/go-playground/validator/v10@v10.30.5`, `github.com/jackc/pgerrcode` (đi kèm pgx).

## Tasks & Steps

1. **`domain`**: viết từng VO và test bảng (`testify/require`). Test transition dùng bảng đầy đủ 3×3 cho `VersionStatus`, `ClassStatus` (`Active→Ended`) và `UserStatus` (`Disabled→Invited` khi chưa đăng nhập, `Disabled→Active` khi đã đăng nhập), assert cả `bool` và thông điệp. `Percent`: `(0,0)=0`, `(1,3)=33`, `(2,3)=67`, `(1,2)=50`, `(3,3)=100`, `(5,3)=100`. <!-- Red Team: RT-03/RT-09/RT-12 --> `Email`: `"  An.Nguyen@GoUp.vn "` → `an.nguyen@goup.vn`; thiếu `@`, hai `@`, > 254 → lỗi. `Code`: `"DB"`, `"GO_1"` hợp lệ; `"db"`, `"D"`, 21 ký tự → lỗi; class code `"basic01"` hợp lệ, `"BASIC01"` lỗi. `Slugify("Giới thiệu SQL")` → `"gioi-thieu-sql"`.
2. **`apperr`**: struct, constructor, `FromDomain`, `Is`, test map từng `Kind` và `errors.Is/As` xuyên qua `Wrap`.
3. **`db` + `pgerr`**: `Executor`, `Transact` trong `db`; `Map` trong `db/pgerr/map.go` chỉ dùng hằng của `constraints.go` (lint: `grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' | grep -v pgerr/constraints.go` phải rỗng). Unit test `map_test.go` dựng `*pgconn.PgError` thủ công cho từng nhánh (Code + ConstraintName + Detail) và assert `Status/Code/Message`; mã lạ (ví dụ `LMS01`) rơi vào nhánh mặc định và không được dịch thành 409; lỗi không phải PgError trả nguyên. `exec_test.go` (integration): `ExecAffectOne` trả `nil` khi 1 dòng, `ErrNoRowsAffected` khi 0 dòng (`UPDATE ... WHERE false`), trả nguyên lỗi pg khi câu lệnh vi phạm constraint. <!-- Updated: Session 2 - D1 --> Integration `tx_integration_test.go`: (a) `fn` trả lỗi → không có dòng insert; (b) `fn` panic → rollback và panic được re-raise (dùng `require.Panics`); (c) thành công → commit. `map_integration_test.go`: `Map` nhận lỗi thật từ DB cho 23505 `UqStageVersionsOneDraft`, 23503 `FkCvsStageVersion` (stage_id lệch), 23514 `CkStageVersionsArchivedAt` (fixture `stage_db_published`, `stage_db_draft`) và trả đúng mã. <!-- Red Team: RT-01/RT-11 -->
4. **`httpx`**: `BindJSON` test: body > 64KB → VALIDATION_FAILED; field lạ → VALIDATION_FAILED chi tiết `"body"`; thiếu `required` → `Details["email"]="Bắt buộc"`; `Fail` với `*apperr.Error` trả envelope đúng; `Fail` với `errors.New` trả 500 không lộ message gốc; `UUIDParam` sai → 404.
5. **`middleware`**: test với `httptest` + gin `TestMode`: `RequestID` giữ header hợp lệ, bỏ header quá dài; `Logger` ghi JSON chứa `request_id`, không chứa query string khi gọi `/x?token=abc`; `Recover` biến panic thành 500 envelope; `RateLimit(perMin=2)` lần 3 trong cùng phút → 429 có `Retry-After`, `enabled=false` → lần 3 vẫn 200; `ClientIP` với `TrustedProxies` rỗng bỏ qua `X-Forwarded-For`; `SecurityHeaders` có đủ 3 header. <!-- Updated: Validation Session 1 - V2 --> <!-- Red Team: RT-11 -->
6. **`clock`, `ids`, `secretbox`**: `Fake.Advance`; `ids.New()` version 7 (`uuid.Version()==7`) và tăng dần theo thời gian (1000 id liên tiếp sắp xếp tăng); `secretbox.New` từ chối khóa ≠ 32 byte, test round-trip/tamper/khóa sai/nonce khác nhau. <!-- Updated: Validation Session 1 - V1 -->
7. **`audit`**: `sanitize` xóa `password_hash` lồng trong map/slice; `PG.Record` integration: ghi trong `testdb.Tx` rồi đọc lại đúng `action, target, before/after` jsonb; rollback tx → không còn dòng.
8. **`testdb`**: `Open` chạy `db.Migrate` một lần, `Reset`, `Tx`, `Fixture`; fixtures lấy dữ liệu từ `prototype/seed.js`: `admin_quan_tran.sql`, `teacher_huong_le.sql`, `student_an_nguyen.sql`, `stage_db_published.sql` (stage `DB` "Database" v1 published + 2 bài markdown đã render HTML + `lesson_media` rỗng), `stage_db_draft.sql` (v2 draft), `course_basic_published.sql` (`BASIC` "Lập trình cơ bản" v1), `class_basic01_active.sql`. Chuyển `migrations_test.go` của Phase 02 sang dùng `testdb.Fixture` nếu đang dùng SQL inline. <!-- Red Team: RT-14 - fixture theo seed.js -->
9. **Nối `app`/`cmd`**: `router.go`, `serve.go`, `deps.go`; chạy `make dev-api` và gọi `GET /api/v1/khong-ton-tai` → `404 {"error":{"code":"NOT_FOUND",...}}` kèm `X-Request-ID`.
10. **`docs/architecture.md` mục Backend**: sơ đồ layer (handler → service → repository → db), quy ước lỗi (domain → apperr → httpx → pgerr), bảng mã lỗi ↔ HTTP, bất biến thứ tự ghi và `ByIDForUpdate`, quy tắc CSRF hai lớp, cách viết feature mới (checklist 8 bước), cách viết integration test với `testdb`. Không tạo `docs/backend-architecture.md`. <!-- Updated: Validation Session 1 - V7 -->

## Verification

```bash
cd apps/api
go build ./... && go vet ./... && go vet -tags integration ./... && golangci-lint run ./...
go test -race -count=1 ./internal/domain/... ./internal/platform/...                       # unit (gồm pgerr/map_test, secretbox)
! grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' | grep -v 'pgerr/constraints.go'  # không chuỗi tên constraint ngoài sổ  <!-- Red Team: RT-11 -->
TEST_DATABASE_URL=postgres://lms:lms@localhost:5432/lms_test?sslmode=disable \
  go test -race -count=1 -p 1 -tags integration ./internal/platform/... ./migrations/...   # integration
go test -cover ./internal/domain/... | grep -E 'coverage: (9[0-9]|100)'                   # domain ≥ 90%
make dev-api & sleep 3
curl -si localhost:8080/api/v1/nope | grep -E 'HTTP/1.1 404|X-Request-Id|NOT_FOUND'
curl -si -X POST localhost:8080/healthz | grep -E '405'
```

Kỳ vọng: mọi lệnh exit 0; `golangci-lint` không cảnh báo; log server là JSON một dòng mỗi request, không có query string.

## Security notes

- `BindJSON` giới hạn 64KB và từ chối field lạ: chống payload lớn và mass-assignment.
- Logger không ghi body, query, header, cookie; `Fail` không lộ thông điệp lỗi nội bộ (chỉ `request_id` để tra log).
- `sanitize` của audit chặn rò `password_hash`/`token_hash` vào `audit_logs`.
- `RateLimit` theo IP phụ thuộc `ClientIP` tin đúng proxy; `cfg.TrustedProxies` (Phase 01) mặc định rỗng nên dev/e2e giới hạn theo `RemoteAddr`, prod phải đặt CIDR của Caddy/Traefik, nếu không mọi request sau proxy chung một IP và limiter 300/phút sẽ chặn nhầm. <!-- Red Team: RT-11 - TRUSTED_PROXIES do Phase 01 sở hữu -->
- `secretbox` không bao giờ log plaintext; khóa đọc một lần từ `cfg.OutboxSecretKey` lúc khởi động. <!-- Updated: Validation Session 1 - V1 -->
- `UUIDParam` sai định dạng trả 404 đồng nhất với không tìm thấy.
- `pgerr` không đưa `pg.Detail`/`pg.Message` (có thể chứa giá trị cột) ra response; chỉ giữ trong `cause` để log.

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| `pgerr` map thiếu constraint mới của phase sau → lỗi 409 "Dữ liệu bị trùng" chung chung | Mọi tên có hằng trong `pgerr/constraints.go` (test hai chiều của Phase 02 bắt thiếu hằng); integration test của mỗi feature assert mã lỗi cụ thể; thêm case vào `map.go` khi thêm constraint (ghi trong `docs/architecture.md`) <!-- Red Team: RT-11 --> |
| `RateLimit` in-memory không chia sẻ giữa nhiều replica | MVP một replica (plan.md §5.4); ghi chú trong docs; nếu scale, đổi sang Postgres `login_attempts` (đã có cho đăng nhập) |
| Thông điệp validator tiếng Việt thiếu tag → rơi về tiếng Anh | `validate_vi.go` có fallback "Giá trị không hợp lệ"; test `oneof`, `min`, `max`, `required`, `email`, `uuid` |
| `-p 1` làm integration test chậm khi số package tăng | Đổi sang DB-per-package như sidecup ở Phase 14 nếu CI > 3 phút |
| Rollback | Phase chỉ thêm package và sửa `router.go`; revert commit, Phase 01 vẫn chạy |

## Success Criteria

- [ ] `internal/domain` không import ngoài stdlib (`go list -deps` kiểm); coverage ≥ 90%; bảng transition `VersionStatus`/`ClassStatus` (`Draft|Active|Ended`)/`UserStatus` (`Disabled→Invited|Active`) được test đầy đủ với thông điệp tiếng Việt; `MemberStatus` có `Completed`; `Percent(1,2)=50`. <!-- Red Team: RT-03/RT-09/RT-12 --> <!-- Updated: Validation Session 1 - V3 -->
- [ ] `apperr` phủ đủ 14 mã của plan.md §7; `FromDomain` map đúng từng `Kind`.
- [ ] `db.Transact` rollback khi lỗi và khi panic (test chứng minh), commit khi thành công.
- [ ] `pgerr.Map` không có nhánh mã lỗi tự định nghĩa (D1); `db.ExecAffectOne` trả `ErrNoRowsAffected` đúng khi 0 dòng; `uq_*_one_draft` → 409 `DRAFT_EXISTS`; 23503 → 409 `IN_USE` (hoặc 404 khi tham chiếu không tồn tại); 23514 → 400 `VALIDATION_FAILED`; test với lỗi thật từ Postgres; không còn chuỗi tên constraint ngoài `constraints.go`. <!-- Red Team: RT-01/RT-11 -->
- [ ] `httpx.BindJSON` giới hạn 64KB, từ chối field lạ, trả `Details` theo tên field JSON; `Fail` trả envelope `{"error":{code,message,details}}`.
- [ ] Middleware `RequestID`, `Logger` (JSON, không body/query), `Recover`, `SecurityHeaders`, `RateLimit` (429 + `Retry-After`, tắt khi `APP_ENV=e2e`), `ClientIP` (không tin proxy khi `TRUSTED_PROXIES` rỗng) có test. <!-- Updated: Validation Session 1 - V2 --> <!-- Red Team: RT-11 -->
- [ ] `secretbox.Seal/Open` round-trip, phát hiện sửa đổi, từ chối khóa sai độ dài. <!-- Updated: Validation Session 1 - V1 -->
- [ ] `clock.Fake`, `ids.New()` (uuid v7), `audit.PG.Record` trong tx của caller, `testdb.Open/Reset/Tx/Fixture` sẵn dùng; `router.go` dùng toàn bộ platform, không còn middleware inline.
- [ ] `docs/architecture.md` có mục Backend mô tả layer, luồng lỗi, bất biến thứ tự ghi, quy tắc CSRF và checklist viết feature mới; không có `docs/backend-architecture.md`. <!-- Updated: Validation Session 1 - V7 -->
