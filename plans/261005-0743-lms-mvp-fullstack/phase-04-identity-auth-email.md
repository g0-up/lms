---
phase: 04
title: "Phase 4: Identity, phiên đăng nhập, email outbox"
status: pending
priority: P1
effort: "3 ngày"
dependencies: [3]
---

# Phase 4: Identity, phiên đăng nhập, email outbox

## Goal

Hoàn thành hai feature backend `identity` và `mailer`: đăng nhập bằng session cookie lưu Postgres, bắt buộc đổi mật khẩu lần đầu (chặn ở backend), quên/đặt lại mật khẩu, khóa tạm khi dò mật khẩu, vô hiệu hóa tài khoản, và hàng đợi email outbox với worker `lms worker` gửi qua SMTP (Mailpit ở dev). Feature `classes` (Phase 7) chỉ cần gọi `identity.UserProvisioner` và `mailer.Enqueuer`, không biết chi tiết.

## Context & Requirements

FR phủ: FR-03, FR-04, FR-05, FR-06, FR-50; nền cho FR-01/FR-02 (sinh mật khẩu tạm, template email mời) được dùng ở Phase 7. NFR §8: hash argon2id, không log mật khẩu/token, rate limit đăng nhập và quên mật khẩu, audit `user.disabled` / `user.enabled`.

Quy tắc spec phải giữ nguyên:

- Email chuẩn hóa: cắt khoảng trắng, chữ thường (`domain.NewEmail`, Phase 3).
- Mật khẩu tạm: CSPRNG, tối thiểu 12 ký tự (`TEMP_PASSWORD_LENGTH`, mặc định 12), hết hạn `TEMP_PASSWORD_TTL` (mặc định 72h); chỉ lưu hash; không bao giờ trả về trong response Admin, không log, không ghi audit payload.
- Đăng nhập khi mật khẩu tạm đã hết hạn → từ chối với "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên."
- `must_change_password = true` → mọi API ngoài allowlist trả `403 PASSWORD_CHANGE_REQUIRED`.
- Mật khẩu mới ≥ 8 ký tự (`PASSWORD_MIN_LENGTH`, mặc định 8), khác mật khẩu tạm. Thành công → `must_change_password=false`, `status=active`, `temp_password_expires_at=NULL`.
- Quên mật khẩu: token dùng một lần, hết hạn `RESET_TOKEN_TTL` (mặc định 30m); phản hồi giống nhau dù email tồn tại hay không.
- Khóa đăng nhập: ≥ `LOGIN_MAX_FAILURES` (5) lần sai trong `LOGIN_LOCK_WINDOW` (15m) **theo email** → từ chối. Không khóa cứng theo IP; chỉ có limiter IP ngưỡng cao `RATE_LIMIT_LOGIN_IP_PER_MIN` (300) chống brute force phân tán. Khi `APP_ENV=e2e` các limiter IP tắt. <!-- Updated: Validation Session 1 - lockout chỉ theo email, limiter IP ngưỡng cao -->
- Vô hiệu hóa: không đăng nhập được, phiên hiện tại bị hủy, reset token đang mở bị vô hiệu, tiến độ giữ nguyên. <!-- Red Team: RT-09 - disable vô hiệu reset token -->
- Email qua hàng đợi, tối đa 3 lần gửi; trạng thái `queued`/`sending`/`sent`/`failed` đọc được từ outbox (hợp đồng cột do Phase 2 sở hữu). Bí mật trong email (mật khẩu tạm, token reset) chỉ nằm ở cột `secret_enc` mã hóa AES-256-GCM bằng `OUTBOX_SECRET_KEY`, bị xóa (`NULL`) ngay khi `sent` hoặc `failed` cuối. <!-- Updated: Validation Session 1 - secret_enc mã hóa, xóa sau khi gửi -->
- Thứ tự kiểm khi đăng nhập: verify argon2 trước; chỉ khi mật khẩu đúng mới trả lý do `disabled` / mật khẩu tạm hết hạn. Mật khẩu sai luôn là "Email hoặc mật khẩu không đúng." để không lộ trạng thái tài khoản. <!-- Red Team: RT-09 - verify trước, lý do sau -->
- Đổi mật khẩu lần đầu: form chỉ có 2 ô (mật khẩu mới, nhập lại) theo prototype; API bỏ qua `currentPassword` khi `status=invited`, bắt buộc khi `status=active`. <!-- Updated: Validation Session 1 - first-login 2 ô -->
- Cấu hình chỉ dùng tên biến do Phase 1 sở hữu trong `platform/config` (`APP_ENV`, `SESSION_TTL`, `COOKIE_SECURE`, `TRUSTED_PROXIES`, `PASSWORD_MIN_LENGTH`, `TEMP_PASSWORD_TTL`, `RESET_TOKEN_TTL`, `LOGIN_MAX_FAILURES`, `LOGIN_LOCK_WINDOW`, `RATE_LIMIT_LOGIN_IP_PER_MIN`, `RATE_LIMIT_FORGOT_IP_PER_MIN`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `MAIL_FROM`, `MAIL_FAIL_PATTERN`, `EMAIL_POLL_INTERVAL`, `OUTBOX_SECRET_KEY`, `PUBLIC_BASE_URL`, `SEED_PASSWORD`); phase này không thêm biến mới. <!-- Red Team: RT-11 - config một nguồn -->

Tài liệu copy: thông điệp mật khẩu tạm hết hạn và tài khoản vô hiệu lấy nguyên văn `spec-lms-mvp.md:119` và `prototype/app.js:325-327` (giữ "Vui lòng"). <!-- Red Team: RT-09 - copy nguyên văn -->

Copy tiếng Việt dùng lại từ `prototype/app.js`:

| Tình huống | Message |
|---|---|
| Sai email/mật khẩu, hoặc email không tồn tại | "Email hoặc mật khẩu không đúng." |
| Email sai định dạng | "Email không hợp lệ." |
| Mật khẩu tạm hết hạn | "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên." |
| Tài khoản disabled đăng nhập | "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên." |
| Mật khẩu mới ngắn | "Mật khẩu mới cần tối thiểu 8 ký tự." |
| Mật khẩu mới trùng tạm | "Mật khẩu mới phải khác mật khẩu tạm." |
| Hai mật khẩu không khớp (client; server vẫn kiểm `confirm`) | "Hai mật khẩu không khớp." |
| Bị khóa tạm | "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút." (mới, không có trong prototype) |

Nhãn trạng thái tài khoản (`STATUS_VI`): `invited` → "Chưa đăng nhập", `active` → "Đã kích hoạt", `disabled` → "Vô hiệu hóa".

## Domain model

### Aggregate `User` (feature `identity`)

```go
type User struct {              // field unexported, dựng qua NewInvitedStudent / NewInternalUser / RehydrateUser
    id          ids.ID
    email       domain.Email
    name        string
    role        Role            // admin | teacher | student
    status      UserStatus      // invited | active | disabled
    password    PasswordHash
    mustChange  bool
    tempExpires *time.Time
    lastLoginAt, lastActiveAt, disabledAt *time.Time
    // không có optimistic lock: bảng users không có cột version; ghi theo id trong tx có FOR UPDATE khi cần  <!-- Red Team: RT-09 - bỏ version -->
}

func NewInvitedStudent(id ids.ID, email domain.Email, name string, tmp TemporaryPassword, hash PasswordHash, now time.Time, ttl time.Duration) (*User, error)
func (u *User) Reinvite(hash PasswordHash, now time.Time, ttl time.Duration) error   // FR-02: chỉ khi mustChange == true, nếu không → ErrAlreadyActivated
func (u *User) Authenticate(now time.Time, verify func(PasswordHash) bool) error    // thứ tự: verify → disabled → temp hết hạn  <!-- Red Team: RT-09 - verify trước -->
func (u *User) ChangePassword(newHash PasswordHash, now time.Time) error            // mustChange=false, status=active, tempExpires=nil
func (u *User) ResetPassword(newHash PasswordHash, now time.Time) error             // như ChangePassword; disabled → ErrAccountDisabled, không đổi status ngoài invited→active  <!-- Red Team: RT-09 -->
func (u *User) Disable(now time.Time) error                                         // active|invited → disabled; admin không tự disable mình (kiểm ở service)
func (u *User) Enable() error                                                       // disabled → (mustChange ? invited : active)
func (u *User) RecordLogin(now time.Time); func (u *User) Touch(now time.Time)      // last_login_at / last_active_at
func (u *User) IsStudent() bool; func (u *User) MustChangePassword() bool
```

Lỗi domain → mã apperr: `ErrInvalidCredentials` → `401 UNAUTHENTICATED` (message uniform), `ErrTempPasswordExpired` → `401 TEMP_PASSWORD_EXPIRED`, `ErrAccountDisabled` → `403 ACCOUNT_DISABLED`, `ErrPasswordTooShort`/`ErrPasswordSameAsTemp` → `422 VALIDATION_FAILED`, `ErrAlreadyActivated` → `409 CONFLICT` ("Học viên đã đổi mật khẩu; không cần gửi lại lời mời."), `ErrInvalidTransition` → `409 INVALID_TRANSITION`.

### Value object

- `UserStatus`, `Role`: `type X string` + bảng chuyển trạng thái `map[UserStatus][]UserStatus{invited:{active,disabled}, active:{disabled}, disabled:{active,invited}}` và `CanTransitionTo`. `Enable()` chọn `disabled → invited` khi `mustChange == true` (chưa từng đăng nhập bằng mật khẩu tạm), ngược lại `disabled → active`; khớp VO ở Phase 3 và `prototype/app.js:321`. <!-- Red Team: RT-09 - transition disabled→invited -->
- `TemporaryPassword`: `GenerateTemporaryPassword(rand io.Reader, length int) (TemporaryPassword, error)` dùng `crypto/rand` với bảng chữ `[A-Za-z2-9]` bỏ ký tự dễ nhầm; `String()` chỉ gọi đúng một lần khi dựng payload email; không có `MarshalJSON`, `fmt` in ra `"***"` (implement `GoString`/`Format`).
- `PasswordHash`: wrapper string PHC, `NewPasswordHash(plain string) (PasswordHash, error)` qua `argon2id.CreateHash(plain, &argon2id.Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32})`; `Verify(plain) bool` dùng `argon2id.ComparePasswordAndHash`.
- `SessionToken`: 32 byte ngẫu nhiên, encode base64url; `Hash() string` = hex(sha256). DB chỉ lưu hash.
- `ResetToken`: như `SessionToken`, TTL riêng.

### Entity khác

- `Session{id, userID, tokenHash, expiresAt, lastSeenAt, createdAt}`; `IsExpired(now)`.
- `LoginAttempt{emailNormalized, ip, succeeded, attemptedAt}` (append-only).
- `PasswordResetToken{tokenHash, userID, expiresAt, usedAt}`; tiêu thụ token bằng một câu `UPDATE … RETURNING user_id` (xem use case 6), 0 hàng → `ErrResetTokenInvalid` → `400 VALIDATION_FAILED` "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn.". <!-- Red Team: RT-09 - consume token bằng UPDATE điều kiện -->

### Feature `mailer`

- Aggregate `OutboxMessage{id, toEmail, template, payload map[string]any, secretEnc []byte, status, attempts, runAt, lockedUntil, lastError, sentAt}` khớp đúng cột `email_outbox` của Phase 2 (`to_email`, `template` CHECK IN `invite|added|resend|password_reset`, `payload jsonb` **không chứa bí mật**, `secret_enc bytea NULL`, `status` CHECK IN `queued|sending|sent|failed`, `attempts`, `run_at`, `locked_until`, `last_error`, `sent_at`). Không có cột body: worker render template lúc gửi. <!-- Red Team: RT-02 - hợp đồng outbox một nguồn -->
  - `MarkSent(now)`: `status=sent`, `sent_at=now`, `secret_enc=NULL`.
  - `MarkFailedAttempt(err, now) (final bool)`: `attempts+1` (chỗ **duy nhất** tăng `attempts`), `last_error=err`; nếu `attempts >= 3` → `status=failed`, `secret_enc=NULL`, `final=true`; ngược lại `status=queued`, `run_at = now + backoff[attempts]` với backoff `1m, 5m, 15m`. <!-- Red Team: RT-02 - attempts tăng một nơi, backoff 1m/5m/15m -->
- Bí mật (`TempPassword`, token reset) được niêm phong bằng `platform/secretbox.Seal` (AES-256-GCM, khóa `OUTBOX_SECRET_KEY` base64 32 byte, Phase 3 cung cấp) khi enqueue và mở bằng `secretbox.Open` ngay trước khi render; không bao giờ log, không trả qua API. <!-- Updated: Validation Session 1 - secretbox -->
- Interface công khai cho feature khác (DIP):

```go
package mailer
type Enqueuer interface {
    Enqueue(ctx context.Context, ex db.Executor, msg Message) (ids.ID, error) // chạy trong tx của caller
}
type Message struct {
    To       domain.Email
    Template TemplateName
    Payload  map[string]any // chỉ dữ liệu không nhạy cảm: Name, ClassName, ClassCode, LoginURL, ExpiresAt, ExpiresMinutes
    Secret   string         // "" nếu không có; được Seal thành secret_enc, không bao giờ vào payload/log
}
const ( TemplateInvite TemplateName = "invite"; TemplateAdded = "added"; TemplateResend = "resend"; TemplatePasswordReset = "password_reset" )
type Sender interface { Send(ctx context.Context, m RenderedMail) error }        // go-mail impl; fake trong test
type OutboxStatusReader interface {                                             // dùng bởi classes (Phase 7) để hiển thị trạng thái gửi
    StatusByIDs(ctx context.Context, ex db.Executor, ids []ids.ID) (map[ids.ID]DeliveryStatus, error)
}
```
<!-- Red Team: RT-02 - template 4 giá trị khớp CHECK, OutboxStatusReader đặt tên nhất quán -->

Mẫu áp dụng: Repository, Unit of Work (`db.Transact`), State machine (`UserStatus`), Outbox (ghi `email_outbox` cùng tx nghiệp vụ), Strategy (`Sender`: SMTP thật / `LogSender` khi `SMTP_HOST` rỗng ở test), Decorator middleware (RequestAuth → MustChangePassword → RequireRole). SOLID: handler không biết sqlx (DIP); `Authenticate` nhận closure verify để entity không phụ thuộc argon2 (ISP); thêm template email mới chỉ thêm file trong `templates/` và một hằng (OCP).

## Repository interfaces

```go
package identity
type UserRepo interface {
    Create(ctx context.Context, ex db.Executor, u *User) error
    Update(ctx context.Context, ex db.Executor, u *User) error            // UPDATE … WHERE id=$1 (không có cột version)  <!-- Red Team: RT-09 -->
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*User, error)
    ByIDForUpdate(ctx context.Context, ex db.Executor, id ids.ID) (*User, error)   // SELECT … FOR UPDATE, dùng trong tx login/reset/disable  <!-- Red Team: RT-09 -->
    ByEmail(ctx context.Context, ex db.Executor, e domain.Email) (*User, error) // sql.ErrNoRows → ErrNotFound
    ListByRole(ctx context.Context, ex db.Executor, role Role, status *UserStatus) ([]*User, error)
}
type SessionRepo interface {
    Create(ctx context.Context, ex db.Executor, s *Session) error
    ByTokenHash(ctx context.Context, ex db.Executor, hash string) (*Session, *User, error) // JOIN users; middleware từ chối (401 + xóa session) khi users.status='disabled'  <!-- Red Team: RT-09 -->
    TouchLastSeen(ctx context.Context, ex db.Executor, id ids.ID, now time.Time) error // chỉ ghi nếu cách lần trước > 1 phút
    DeleteByTokenHash(ctx context.Context, ex db.Executor, hash string) error
    DeleteAllForUser(ctx context.Context, ex db.Executor, userID ids.ID, exceptID *ids.ID) error
    DeleteExpired(ctx context.Context, ex db.Executor, now time.Time) (int64, error)
}
type LoginAttemptRepo interface {
    LockEmail(ctx context.Context, ex db.Executor, email domain.Email) error     // SELECT pg_advisory_xact_lock(hashtext($1)) với email_normalized; gọi đầu tx login  <!-- Updated: Validation Session 1 - advisory lock chống bypass song song -->
    Record(ctx context.Context, ex db.Executor, a LoginAttempt) error
    CountFailures(ctx context.Context, ex db.Executor, email domain.Email, since time.Time) (int, error)
    DeleteOlderThan(ctx context.Context, ex db.Executor, cutoff time.Time) (int64, error) // worker dọn bản ghi > 30 ngày  <!-- Updated: Validation Session 1 - bỏ CountFailuresByIP, thêm dọn dẹp -->
}
type ResetTokenRepo interface {
    Create(ctx context.Context, ex db.Executor, t *PasswordResetToken) error
    Consume(ctx context.Context, ex db.Executor, hash string, now time.Time) (ids.ID, error)
    // UPDATE password_reset_tokens SET used_at=$2 WHERE token_hash=$1 AND used_at IS NULL AND expires_at>$2 RETURNING user_id; 0 hàng → ErrResetTokenInvalid  <!-- Red Team: RT-09 -->
    InvalidateForUser(ctx context.Context, ex db.Executor, userID ids.ID, now time.Time) error
}

package mailer
type OutboxRepo interface {
    Insert(ctx context.Context, ex db.Executor, m *OutboxMessage) error
    Claim(ctx context.Context, ex db.Executor, limit int) ([]*OutboxMessage, error)   // SQL bên dưới
    MarkSent(ctx context.Context, ex db.Executor, id ids.ID, now time.Time) error
    MarkFailedAttempt(ctx context.Context, ex db.Executor, m *OutboxMessage) error    // ghi attempts/status/run_at/last_error/secret_enc đã tính ở entity
    StatusByIDs(ctx context.Context, ex db.Executor, ids []ids.ID) (map[ids.ID]DeliveryStatus, error) // impl OutboxStatusReader
    SupersedeQueued(ctx context.Context, ex db.Executor, toEmail string, templates []string, now time.Time) (int64, error) // UPDATE … SET status='failed', last_error='superseded', secret_enc=NULL WHERE to_email=$1 AND template = ANY($2) AND status='queued'; Phase 7 gọi trong tx xoay mật khẩu tạm  <!-- Red Team: RT-10 - supersede hàng queued cũ -->
}
```

Claim SQL (viết đủ, không tham chiếu báo cáo): <!-- Red Team: RT-02 - claim SQL viết rõ -->

```sql
UPDATE email_outbox
SET status = 'sending', locked_until = now() + interval '2 minutes'
WHERE id IN (
  SELECT id FROM email_outbox
  WHERE (status = 'queued' AND run_at <= now())
     OR (status = 'sending' AND locked_until < now())
  ORDER BY run_at
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

Index phục vụ claim: `ix_email_outbox_claim ON email_outbox (run_at) WHERE status IN ('queued','sending')` (Phase 2 tạo). Lease hết hạn (`sending` quá 2 phút) được claim lại mà **không** tăng `attempts`; chỉ `MarkFailedAttempt` tăng.

`DeliveryStatus{Status string /* queued|sending|sent|failed */, Attempts int, LastError *string, SentAt *time.Time}`; FE gộp `sending` vào nhãn "Đang chờ gửi".

## Use cases / Service methods

`identity.Service` nhận `db.Tx` (Transact), `UserRepo`, `SessionRepo`, `LoginAttemptRepo`, `ResetTokenRepo`, `mailer.Enqueuer`, `clock.Clock`, `audit.Recorder`, và đọc trực tiếp các field của `config.Config` Phase 1: `TempPasswordTTL`, `PasswordMinLength`, `ResetTokenTTL`, `SessionTTL`, `LoginMaxFailures`, `LoginLockWindow`, `PublicBaseURL`, `AppEnv`. Độ dài mật khẩu tạm là hằng 12 trong `password.go` (spec: tối thiểu 12), không thêm biến env. <!-- Red Team: RT-11 - dùng tên field Phase 1 -->

1. `Login(ctx, email, password, ip) (*Session, SessionToken, *User, error)` — **một `Transact`** để kiểm khóa + ghi attempt + tạo phiên không bị đua: <!-- Red Team: RT-09 / Validation Session 1 - login trong tx có advisory lock -->
   1. `domain.NewEmail` lỗi → `VALIDATION_FAILED` "Email không hợp lệ.".
   2. `LockEmail(email)` (`pg_advisory_xact_lock(hashtext(email_normalized))`) rồi `CountFailures(email, now-LoginLockWindow) >= LoginMaxFailures` → `429 TOO_MANY_ATTEMPTS` "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút.", header `Retry-After` = giây còn lại của cửa sổ. Không đếm theo IP. Kiểm trước khi verify argon2.
   3. `ByEmail` không có → ghi attempt thất bại, verify hash giả (`argon2id.ComparePasswordAndHash` với hash cố định) để đồng đều thời gian, trả `401 UNAUTHENTICATED`.
   4. `u.Authenticate(now, verify)`: **verify trước**; sai → ghi attempt, `401 UNAUTHENTICATED` "Email hoặc mật khẩu không đúng."; đúng nhưng `disabled` → `403 ACCOUNT_DISABLED` "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên."; đúng nhưng temp hết hạn → `401 TEMP_PASSWORD_EXPIRED` "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên." (hai trường hợp này không tính là attempt sai).
   5. Cùng tx: `ByIDForUpdate(u.id)` kiểm lại `status` (chặn đua với `DisableUser`), ghi attempt thành công, `u.RecordLogin(now)` + `Update`, tạo `Session` TTL `SessionTTL` (mặc định 12h, gia hạn trượt qua `TouchLastSeen`). Không ghi audit cho login.
2. `Logout(ctx, tokenHash)` → `DeleteByTokenHash`; luôn 204.
3. `Me(ctx, user) MeDTO` → id, name, email, role, status, mustChangePassword, tempPasswordExpiresAt (chỉ khi mustChange).
4. `ChangePassword(ctx, user, cmd{CurrentPassword *string, NewPassword, ConfirmPassword}, sessionID)` <!-- Updated: Validation Session 1 - currentPassword tùy chọn khi invited -->
   1. `NewPassword != ConfirmPassword` → `422` "Hai mật khẩu không khớp." (server vẫn kiểm dù client đã kiểm).
   2. `len(new) < PasswordMinLength` → "Mật khẩu mới cần tối thiểu 8 ký tự."
   3. Nếu `user.status == active`: `CurrentPassword` bắt buộc (thiếu → 422 "Nhập mật khẩu hiện tại."), verify với hash hiện tại; sai → `401 UNAUTHENTICATED` "Email hoặc mật khẩu không đúng.". Nếu `user.status == invited` (`must_change_password=true`, phiên đã được tạo bằng mật khẩu tạm): bỏ qua `CurrentPassword` kể cả khi client gửi.
   4. Mật khẩu mới trùng hash hiện tại (`Verify(new)` đúng) → "Mật khẩu mới phải khác mật khẩu tạm." (message chung cho cả user active).
   5. Transact: `ByIDForUpdate`; `u.ChangePassword(hash, now)` → `Update`; `DeleteAllForUser(userID, except=sessionID)`; `InvalidateForUser` reset tokens.
5. `ForgotPassword(ctx, email, ip)`: luôn trả 202 với body `{"message":"Nếu email tồn tại, chúng tôi đã gửi hướng dẫn đặt lại mật khẩu."}`. Hình dạng xử lý **giống nhau** dù user có hay không để không đo được thời gian: luôn sinh `ResetToken` + hash; luôn mở một `Transact`; nếu user tồn tại và `status != disabled` thì `InvalidateForUser` token cũ, `Create` token (TTL `ResetTokenTTL` 30m), `Enqueue(TemplatePasswordReset, Payload{Name, ExpiresMinutes}, Secret: raw)`; nếu không thì tx rỗng rồi commit. Link do worker ghép lúc render: `PublicBaseURL + "/reset-password#token=" + secret` (fragment, không vào log server/nginx). Rate limit chỉ ở middleware theo IP `RATE_LIMIT_FORGOT_IP_PER_MIN` (30), tắt khi `APP_ENV=e2e`. <!-- Red Team: RT-09 - constant-shape, link dùng fragment -->
6. `ResetPassword(ctx, cmd{Token, NewPassword, ConfirmPassword})` — **một `Transact`**: `ConfirmPassword` khớp + độ dài; `Consume(hash(token), now)` → `userID` (0 hàng → 400 "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn."); `ByIDForUpdate(userID)`; `status == disabled` → `403 ACCOUNT_DISABLED` không đổi gì; `u.ResetPassword(hash, now)` (chỉ `invited → active`, không đổi status khác); `Update`; `DeleteAllForUser(userID, nil)`; không ghi audit (tập 20 action chuẩn plan.md §7 không có hành động đặt lại mật khẩu; dấu vết nằm ở `password_reset_tokens.used_at`). Thành công 204. <!-- Red Team: RT-09 - reset trong một tx, token tiêu thụ nguyên tử -->
7. `DisableUser(ctx, actor, userID)`: chỉ `role=student` hoặc `teacher` (admin không disable admin, `409 CONFLICT` "Không vô hiệu hóa được tài khoản quản trị."); Transact: `ByIDForUpdate`, `u.Disable(now)`, `Update`, `sessions.DeleteAllForUser(userID, nil)`, `resetTokens.InvalidateForUser(userID, now)`, audit `user.disabled` `{before:{status}, after:{status}}`. <!-- Red Team: RT-09 - disable vô hiệu reset token -->
8. `EnableUser(ctx, actor, userID)`: `u.Enable()`, audit `user.enabled`.
9. `ListUsers(ctx, role, status)` → DTO list cho select giảng viên.
10. `ProvisionStudent(ctx, ex db.Executor, email, name, now) (*User, TemporaryPassword, created bool, error)` và `RotateTemporaryPassword(ctx, ex, u *User, now) (TemporaryPassword, error)`: API nội bộ cho Phase 7 (interface `UserProvisioner` khai báo trong `identity`, gọi trong tx của classes). Không ghi audit ở đây; classes ghi `class.member_invited`.

`mailer.Service`: `Enqueue(ctx, ex, Message{To, Template, Payload, Secret})` **không render**; chỉ kiểm template thuộc 4 giá trị cho phép, kiểm `Payload` không chứa key `TempPassword`/`Link`/`Token` (panic trong test, lỗi `ErrSecretInPayload` lúc chạy), `Seal(Secret)` → `secret_enc` (NULL khi `Secret==""`), rồi `Insert` với `status='queued'`, `run_at=now`. Worker render lúc gửi: `Open(secret_enc)` → ghép vào data (`TempPassword` cho `invite`/`resend`, `Link = PublicBaseURL + "/reset-password#token=" + secret` cho `password_reset`) → `Render(template, data)` → `Sender.Send`. `Worker.RunOnce(ctx)`: Transact ngắn `Claim(ctx, 10)` (lease 2 phút qua `locked_until`) → ngoài tx gửi tuần tự → mỗi message một tx `MarkSent(id, now)` (đặt `secret_enc=NULL`) hoặc `MarkFailedAttempt(id, now, err)` (`attempts+1`; `<3` → `run_at = now + {1m,5m,15m}[attempts-1]`, `status='queued'`; `=3` → `status='failed'`, `secret_enc=NULL`); `last_error` cắt 500 ký tự, không chứa body. `Worker.Run(ctx)`: ticker `EMAIL_POLL_INTERVAL` (5s), dừng theo `ctx`; mỗi giờ gọi `LoginAttemptRepo.DeleteOlderThan(now-30d)`. Không có lệnh CLI xem/retry outbox ở MVP; trạng thái đọc qua `OutboxStatusReader` cho Phase 7. <!-- Red Team: RT-02 / Validation Session 1 - render lúc gửi, secretbox, 3 lần, backoff 1/5/15 -->

Template (`internal/features/mailer/templates/*.html.tmpl` + `.txt.tmpl`, `html/template` tự escape, tiếng Việt, có `{{.AppName}}`):

| Template | Subject | Data |
|---|---|---|
| `invite` | "[GoUp LMS] Lời mời vào lớp {{.ClassCode}}" | payload: Name, ClassName, ClassCode, LoginURL, Email, ExpiresAt; từ `secret_enc`: TempPassword |
| `added` | "[GoUp LMS] Bạn đã được thêm vào lớp {{.ClassCode}}" | payload: Name, ClassName, ClassCode, LoginURL |
| `resend` | "[GoUp LMS] Mật khẩu tạm mới cho lớp {{.ClassCode}}" | payload: Name, ClassName, ClassCode, LoginURL, Email, ExpiresAt; từ `secret_enc`: TempPassword |
| `password_reset` | "[GoUp LMS] Đặt lại mật khẩu" | payload: Name, ExpiresMinutes; từ `secret_enc`: Link (worker ghép fragment) |

<!-- Red Team: RT-02 - 4 template invite/added/resend/password_reset; bỏ account_disabled; secret không nằm trong payload -->

## HTTP API

Tất cả dưới `/api/v1`. Middleware chuỗi: `RequestID → Logger → Recover → CrossOriginProtection → RateLimit(theo route) → SessionAuth → MustChangePassword → RequireRole`.

| Endpoint | Vai trò | Request | Response | Lỗi |
|---|---|---|---|---|
| `POST /auth/login` | public, limiter IP `RATE_LIMIT_LOGIN_IP_PER_MIN` (300/phút, chống flood; khóa thật theo email 5/15m trong service), tắt khi `APP_ENV=e2e` <!-- Updated: Validation Session 1 - V2 --> | `{email, password}` | `200 {user: MeDTO}` + `Set-Cookie: __Host-sid=…; HttpOnly; Secure; SameSite=Lax; Path=/` | 422 `VALIDATION_FAILED`; 401 `UNAUTHENTICATED`; 401 `TEMP_PASSWORD_EXPIRED`; 403 `ACCOUNT_DISABLED`; 429 `TOO_MANY_ATTEMPTS` + `Retry-After` |
| `POST /auth/logout` | đã đăng nhập (allowlist) | – | 204, cookie `Max-Age=0` | – |
| `GET /auth/me` | đã đăng nhập (allowlist) | – | `200 MeDTO` | 401 `UNAUTHENTICATED` |
| `POST /auth/change-password` | đã đăng nhập (allowlist) | `{currentPassword?, newPassword, confirmPassword}` — `currentPassword` bắt buộc khi `status=active`, bỏ qua khi `invited` <!-- Updated: Validation Session 1 - V4 first-login 2 trường --> | `200 MeDTO` | 422 `VALIDATION_FAILED` (message ở use case 4), 401 |
| `POST /auth/forgot-password` | public, limiter IP `RATE_LIMIT_FORGOT_IP_PER_MIN` (30/phút), tắt khi `APP_ENV=e2e` <!-- Red Team: RT-11 - tên biến --> | `{email}` | 202 message uniform | 422 chỉ khi JSON hỏng; email sai định dạng vẫn 202 |
| `POST /auth/reset-password` | public, dùng chung limiter `RATE_LIMIT_FORGOT_IP_PER_MIN` | `{token, newPassword, confirmPassword}` (token lấy từ `location.hash` phía web) | 204 | 400 `VALIDATION_FAILED` "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn."; 422 độ dài/không khớp; 403 `ACCOUNT_DISABLED` |
| `GET /users?role=teacher&status=active` | admin | query | `200 {items:[{id,name,email,role,status}]}` | 403 |
| `POST /users/{id}/disable` | admin | – | `200 UserDTO` | 404, 409 `INVALID_TRANSITION` |
| `POST /users/{id}/enable` | admin | – | `200 UserDTO` | 404, 409 |

`MeDTO{id, name, email, role, status, mustChangePassword, tempPasswordExpiresAt?}`. Không có field nào chứa hash/mật khẩu. Middleware `MustChangePassword` trả `403 {"error":{"code":"PASSWORD_CHANGE_REQUIRED","message":"Bạn cần đổi mật khẩu trước khi tiếp tục."}}`; allowlist so khớp `(method, fullPath)`: `POST /api/v1/auth/change-password`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`.

Cookie: tên `__Host-sid` khi `COOKIE_SECURE=true` (prod); dev http://localhost dùng `sid` không prefix và `Secure=false` (flag `COOKIE_SECURE=false`), vì `__Host-` bắt buộc `Secure` và Safari từ chối cookie Secure trên http. CSRF: **cả hai** lớp — `http.CrossOriginProtection` bọc `gin.Engine` **và** `httpx.RequireCustomHeader` yêu cầu `X-Requested-With: fetch` cho mọi POST/PUT/PATCH/DELETE (thiếu → 403 `FORBIDDEN`), bỏ qua cho `/healthz`, `/readyz`. Web client luôn gửi header này (Phase 10). <!-- Red Team: RT-05 - CSRF hai lớp -->

## Files to Create / Modify

```text
apps/api/internal/features/identity/
  entity.go            # User, Session, LoginAttempt, PasswordResetToken, UserStatus, Role, lỗi domain
  password.go          # PasswordHash (argon2id), TemporaryPassword, SessionToken, ResetToken
  repository.go        # 4 interface + UserProvisioner
  repository_pg.go     # sqlx impl, scan struct riêng (userRow) → RehydrateUser
  service.go           # use case 1–10
  handler.go           # Register(r gin.IRouter, auth *Middleware)
  dto.go
  middleware.go        # SessionAuth, MustChangePassword, RequireRole, CurrentUser(c)
  entity_test.go password_test.go service_test.go middleware_test.go
  service_integration_test.go   # //go:build integration
apps/api/internal/features/mailer/
  entity.go repository.go repository_pg.go service.go worker.go sender_smtp.go sender_log.go templates.go
  secretbox.go         # dùng platform/secretbox (AES-256-GCM, key OUTBOX_SECRET_KEY)  <!-- Updated: Validation Session 1 - V1 -->
  templates/{invite,added,resend,password_reset}.{html,txt}.tmpl  <!-- Red Team: RT-02 -->
  worker_integration_test.go templates_test.go
apps/api/internal/platform/middleware/csrf.go        # CrossOriginProtection + custom header
apps/api/internal/platform/middleware/ratelimit.go   # x/time/rate keyed by IP, LRU bound 10k, no-op khi APP_ENV=e2e
apps/api/internal/platform/secretbox/secretbox.go    # Seal/Open AES-256-GCM, nonce 12 byte prefix
apps/api/internal/platform/db/pgerr/constraints.go   # thêm const UqUsersEmailNormalized = "uq_users_email_normalized" (Phase 2 sở hữu file; `pgerr.Map(err)` ở Phase 3)  <!-- Red Team: RT-11 -->
apps/api/internal/app/router.go                      # mount identity, mailer deps
apps/api/internal/app/deps.go
apps/api/cmd/lms/worker.go                           # `lms worker`: outbox + dọn login_attempts, Run until signal
apps/api/cmd/lms/seed.go                             # seed users theo prototype/seed.js; hash argon2id qua identity.NewPasswordHash (hook, không tự hash); hàng email_outbox queued của Phase 2 (secret_enc NULL) được phase này bổ sung secret_enc = secretbox.Seal(SEED_PASSWORD)  <!-- Updated: Validation Session 1 - V1 seed Seal -->
```

Không sửa `config.go` ở phase này: struct `config.Config` Phase 1 đã có đủ field (`SessionTTL`, `TempPasswordTTL`, `ResetTokenTTL`, `PasswordMinLength`, `LoginMaxFailures`, `LoginLockWindow`, `RateLimitLoginIPPerMin`, `RateLimitForgotIPPerMin`, `OutboxSecretKey`, `EmailPollInterval`, `SMTPHost/Port/User/Password`, `MailFrom`, `PublicBaseURL`, `CookieSecure`, `AppEnv`). `.env.example` do Phase 1 sở hữu; phase này chỉ xác nhận giá trị dev `SMTP_HOST=mailpit`, `SMTP_PORT=1025`, `MAIL_FROM`, `COOKIE_SECURE=false`, `OUTBOX_SECRET_KEY` (32 byte base64). <!-- Red Team: RT-11 - không trùng sở hữu config -->

```text
```

## Tasks & Steps

1. Viết `entity.go` + `password.go` với test bảng chuyển trạng thái, `Authenticate` thứ tự lỗi, `GenerateTemporaryPassword` độ dài và tập ký tự, `PasswordHash` round-trip và tham số argon2 (parse PHC kiểm `m=19456,t=2,p=1`).
2. Viết `repository_pg.go`; `pgerr.Map(err)` rồi so `Constraint == pgerr.UqUsersEmailNormalized` (`uq_users_email_normalized`) → `ErrEmailTaken` (`409 CONFLICT`), không tự parse `23505`; `SessionRepo.ByTokenHash` JOIN `users` lọc `status <> 'disabled'`. <!-- Red Team: RT-11 / RT-09 -->
3. Viết `middleware.go`: `SessionAuth` đọc cookie → `ByTokenHash` → hết hạn thì xóa + 401; gắn `*User` và `sessionID` vào `gin.Context`; `TouchLastSeen` throttle 1 phút; `MustChangePassword`; `RequireRole(roles...)`.
4. Viết `service.go` theo thứ tự rule ở trên (verify → disabled → temp hết hạn; login/reset/disable mỗi cái một tx với `ByIDForUpdate`); inject `clock.Clock` để test hết hạn.
5. Viết `mailer`: `platform/secretbox`, `Enqueue` chỉ seal + insert, render **lúc gửi** trong `Worker`, `OutboxRepo.Claim` SQL ở trên, `MarkSent`/`MarkFailedAttempt`, `SMTPSender` (go-mail: `mail.NewClient(host, WithPort, WithTLSPolicy(mail.TLSOpportunistic), WithSMTPAuth khi có user)`), `LogSender` (log subject + to, **không** log body). <!-- Red Team: RT-02 -->
6. Handler + DTO + `Register`; limiter IP `RATE_LIMIT_LOGIN_IP_PER_MIN` trên `/auth/login`, `RATE_LIMIT_FORGOT_IP_PER_MIN` trên `/auth/forgot-password` và `/auth/reset-password`; cả hai no-op khi `APP_ENV=e2e`.
7. `cmd/lms/worker.go`; `cmd/lms/seed.go` tạo admin `quan.tran@goup.vn`, giảng viên `huong.le@goup.vn`, `bao.pham@goup.vn`, học viên theo `prototype/seed.js` (status/mustChangePassword/tempPasswordExpiresAt tương ứng: `dung.pham` temp hết hạn, `minh.bui`/`quyen.ly`/`son.trinh`/`tu.mai` invited còn hạn, `thao.vo` disabled), mật khẩu dev từ `SEED_PASSWORD` (bắt buộc đặt, không hardcode). Seed **không tự hash**: gọi `identity.NewPasswordHash` để một nguồn tham số argon2. Phase 2 đã seed các hàng `email_outbox` `queued` mẫu với `secret_enc = NULL`; phase này sửa seed để những hàng `invite`/`resend` còn `queued` mang `secret_enc = secretbox.Seal(SEED_PASSWORD)` (mật khẩu tạm của học viên `invited` chính là `SEED_PASSWORD`), nhờ đó worker dev gửi được mail thật từ dữ liệu seed; hàng `sent`/`failed` giữ NULL. <!-- Updated: Validation Session 1 - seed dùng hook argon2 của identity; V1 seed Seal -->
8. Bổ sung `router.go`, `deps.go` (đọc field `config.Config` sẵn có; không sửa `config.go`).
9. Viết integration test (testdb Phase 3): login lockout (5 sai → 429, đúng mật khẩu vẫn 429 trong cửa sổ; 20 goroutine login sai song song → đúng 5 attempt được ghi rồi 429, nhờ advisory lock), temp hết hạn, disabled (login 403 **và** session đang có bị `SessionAuth` từ chối ngay sau disable; reset token tạo trước disable → `ResetPassword` 403 không đổi gì), must-change middleware chặn `GET /stages` nhưng cho `GET /auth/me`, change-password khi `invited` không cần `currentPassword`, change-password thu hồi session khác, forgot/reset round-trip qua outbox (link chứa `#token=`), outbox claim/retry/fail (sender giả lỗi 3 lần → `failed`, `last_error` lưu, `secret_enc IS NULL`; sau `sent` cũng `secret_enc IS NULL`), hàng `queued` cũ của cùng `to_email`+template bị `last_error='superseded'` khi Phase 7 gửi lại, SKIP LOCKED với 2 worker song song không trùng. <!-- Red Team: RT-09 / RT-02 - test bổ sung -->
10. Chạy lint, test, cập nhật `docs/` phần cấu hình env (chỉ bảng biến môi trường).

## Verification

```bash
cd apps/api && go test ./internal/features/identity/... ./internal/features/mailer/... ./internal/platform/middleware/...
cd apps/api && go test -tags integration ./internal/features/identity/... ./internal/features/mailer/...
make dev && make seed
# login sai 5 lần → lần 6 phải 429 kèm Retry-After
for i in 1 2 3 4 5 6; do curl -s -o /dev/null -w "%{http_code}\n" -H 'X-Requested-With: fetch' -H 'Content-Type: application/json' \
  -d '{"email":"an.nguyen@gmail.com","password":"sai"}' localhost:8080/api/v1/auth/login; done
# login đúng (học viên invited) → 200, sau đó GET /me/classes phải 403 PASSWORD_CHANGE_REQUIRED
curl -s -c c.txt -H 'X-Requested-With: fetch' -H 'Content-Type: application/json' -d '{"email":"minh.bui@gmail.com","password":"<temp từ Mailpit>"}' localhost:8080/api/v1/auth/login
curl -s -b c.txt localhost:8080/api/v1/me/classes | jq .error.code      # "PASSWORD_CHANGE_REQUIRED"
curl -s -b c.txt localhost:8080/api/v1/auth/me | jq .mustChangePassword # true
# đổi mật khẩu → 200, sau đó /me/classes 200
# học viên invited: không cần currentPassword  <!-- Updated: Validation Session 1 - V4 -->
curl -s -b c.txt -H 'X-Requested-With: fetch' -H 'Content-Type: application/json' -d '{"newPassword":"matkhaumoi1","confirmPassword":"matkhaumoi1"}' localhost:8080/api/v1/auth/change-password
# forgot → 202 uniform; Mailpit có mail
curl -s -w "%{http_code}" -H 'X-Requested-With: fetch' -H 'Content-Type: application/json' -d '{"email":"khongtontai@example.com"}' localhost:8080/api/v1/auth/forgot-password
curl -s 'http://localhost:8025/api/v1/messages?limit=5' | jq '.messages[] | {To, Subject}'
# forgot limiter: 31 request/phút cùng IP → 429 (APP_ENV=development)
for i in $(seq 1 31); do curl -s -o /dev/null -w "%{http_code} " -H 'X-Requested-With: fetch' -H 'Content-Type: application/json' -d '{"email":"x@example.com"}' localhost:8080/api/v1/auth/forgot-password; done; echo
# outbox sau khi gửi: không còn bí mật
docker compose -f infra/docker-compose.yml exec -T postgres psql -U lms -d lms -c "select count(*) from email_outbox where status in ('sent','failed') and secret_enc is not null"  # 0
# POST không có X-Requested-With hoặc Origin lạ → 403 (hai lớp CSRF)
curl -s -o /dev/null -w "%{http_code}\n" -H 'Origin: https://evil.example' -H 'Content-Type: application/json' -d '{}' localhost:8080/api/v1/auth/logout
curl -s -o /dev/null -w "%{http_code}\n" -b c.txt -H 'Content-Type: application/json' -d '{}' localhost:8080/api/v1/auth/logout   # thiếu header → 403
# log không chứa mật khẩu
docker compose -f infra/docker-compose.yml logs api | grep -ci 'password"' ; # phải là 0 ngoài tên field trong path
```

## Security notes

- Hash argon2id tham số OWASP; verify luôn chạy kể cả khi user không tồn tại (chống đo thời gian).
- Session opaque 32 byte, DB lưu sha256; đổi/đặt lại mật khẩu và disable thu hồi mọi session ngay (spec FR-06).
- `slog` bỏ qua field `password`, `newPassword`, `token`, cookie; `httpx.BindJSON` không log body. Audit payload chỉ gồm `status`.
- `TemporaryPassword` không có `String()` công khai ngoài `Reveal()` dùng đúng một chỗ trong `mailer` data; `Format` in `***`.
- Phản hồi forgot-password uniform; reset token dùng một lần, hash trong DB.
- Khóa đăng nhập theo email nằm trong DB (`login_attempts` + advisory lock) nên đúng khi chạy nhiều instance; chỉ limiter IP (chống flood) là in-process, chấp nhận ở MVP. <!-- Updated: Validation Session 1 - V2 -->
- `secret_enc` chỉ được giải mã trong worker lúc render; `payload` jsonb không bao giờ chứa mật khẩu tạm/token; `MarkSent`/`failed` xóa `secret_enc`. Mất `OUTBOX_SECRET_KEY` → hàng `queued` cũ không gửi được, chuyển `failed` với `last_error='secretbox: open'`. <!-- Updated: Validation Session 1 - V1 -->
- Link đặt lại mật khẩu dùng fragment `#token=` nên không vào access log nginx/API; web đọc `location.hash` rồi xóa khỏi URL. <!-- Red Team: RT-09 -->

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| Cookie `__Host-` không hoạt động trên http dev (Safari) | Flag `COOKIE_SECURE=false` dùng tên `sid`; E2E chạy qua Caddy https hoặc Chromium |
| Khóa theo email cho phép kẻ tấn công khóa người khác 15 phút | Chấp nhận ở MVP (quyết định V2): chỉ khóa theo email; limiter IP 300/phút chỉ chống flood, không khóa; admin có thể hướng dẫn chờ hoặc dùng quên mật khẩu <!-- Updated: Validation Session 1 - V2 --> |
| Worker chết giữa lúc gửi → gửi trùng | At-least-once chấp nhận; lease 2 phút qua `locked_until`; ghi log `outbox_id` <!-- Red Team: RT-02 --> |
| SMTP prod chưa có | `LogSender` khi `SMTP_HOST` rỗng; outbox vẫn ghi, trạng thái `queued` hiển thị đúng |

Rollback: phase chỉ thêm code và template; migration đã ở Phase 2. Tắt bằng không mount feature trong `router.go`.

## Success Criteria

- [x] `go test ./internal/features/identity/... ./internal/features/mailer/...` xanh; coverage entity ≥ 90%.
- [x] Integration: 5 lần sai → 429 + `Retry-After` (song song cũng chỉ ghi 5 attempt); mật khẩu tạm hết hạn → `TEMP_PASSWORD_EXPIRED`; disabled → `ACCOUNT_DISABLED`; sai mật khẩu của tài khoản disabled → `UNAUTHENTICATED` (không lộ trạng thái). <!-- Red Team: RT-09 -->
- [x] Sau `DisableUser`: session cũ bị từ chối ngay; reset token tạo trước đó → 403 và không đổi mật khẩu. <!-- Red Team: RT-09 -->
- [x] `must_change_password=true` → `GET /stages` 403 `PASSWORD_CHANGE_REQUIRED`; `GET /auth/me`, `POST /auth/change-password`, `POST /auth/logout` đi qua.
- [x] Đổi mật khẩu xong: `must_change_password=false`, `status=active`, `temp_password_expires_at IS NULL`, session khác bị xóa; học viên `invited` đổi được với body 2 trường. <!-- Updated: Validation Session 1 - V4 -->
- [x] Forgot/reset round-trip qua Mailpit; token dùng lần hai → 400; phản hồi forgot giống nhau cho email có/không tồn tại.
- [x] Outbox: sender lỗi → `attempts` tăng, `run_at` lùi 1/5/15 phút, lần 3 → `failed` + `last_error`; `secret_enc IS NULL` sau `sent`/`failed`; `payload` không chứa mật khẩu tạm; 2 worker song song không claim trùng. <!-- Red Team: RT-02 -->
- [x] `grep` log API trong suite integration không thấy mật khẩu tạm hay token thô.
- [x] Audit có `user.disabled` / `user.enabled` với `actor_id`.
