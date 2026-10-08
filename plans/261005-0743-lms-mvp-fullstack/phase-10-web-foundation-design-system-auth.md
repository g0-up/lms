---
phase: 10
title: "Phase 10: Web foundation: design system, shell, auth"
status: in-progress
priority: P1
effort: "3 ngày"
dependencies: [1, 4]
---

# Phase 10: Web foundation: design system, shell, auth

## Goal

Dựng nền `apps/web` để ba phase sau (11, 12, 13) chỉ cần thêm thư mục `src/features/<feature>` là có trang chạy được:
toolchain Vite + React 19 + TypeScript + Tailwind 4 + shadcn, design system GoUp phủ 100% token của
`prototype/goup.css`, bộ `shared/ui` đã chỉnh theo `docs/design.md`, lớp HTTP + React Query với xử lý 401/403 tập
trung, router React Router v8 data mode với middleware auth/role, `AppShell` + `AuthLayout`, và ba trang auth
(`/login`, `/forgot`, `/reset-password`, `/first-login`) hoạt động thật với API phase 4.

## Context & Requirements

- Nguồn: `docs/design.md`, `docs/prototype.md`, `prototype/{index.html,goup.css,styles.css,app.js}`, báo cáo
  `plans/reports/researcher-261005-1437-react-frontend-stack.md`, `plan.md` §5, §7, §8.
- Phụ thuộc: phase 1 (monorepo, docker compose, api reverse proxy `/api`), phase 4 (API auth: `POST
  /auth/login|logout|change-password|forgot-password|reset-password`, `GET /auth/me`).
- **Quy tắc sở hữu file (bắt buộc cho phase 10–13):** chỉ phase 10 được tạo/sửa `src/app/**`, `src/shared/**`,
  `src/styles/**`, cấu hình gốc (`vite.config.ts`, `eslint.config.js`, `tsconfig*.json`, `components.json`,
  `package.json`). Phase 11–13 chỉ thêm file dưới `src/features/<feature>/**` và đăng ký route qua
  `src/features/<feature>/routes.tsx` (hợp đồng mô tả ở mục Architecture). Phase 10 phải tạo trước bảy file
  `routes.tsx` rỗng (`export const routes: RouteObject[] = []`) cho `auth`, `dashboard`, `stages`, `courses`,
  `classes`, `reports`, `learning` để router import được ngay từ đầu; phase 10 chỉ điền nội dung cho
  `auth/routes.tsx`.
- Phiên bản (registry 2026-10-05): react 19.3, vite 8.3, @vitejs/plugin-react 6.1, typescript ~6.0.3 (không dùng 7),
  tailwindcss 4.3 + @tailwindcss/vite, shadcn CLI v4 (`-b radix`, gói `radix-ui`), react-router 8.4 (import từ
  `react-router`, `RouterProvider` từ `react-router/dom`; không có `react-router-dom`), @tanstack/react-query 5.104,
  react-hook-form 7.89 + zod 4.6 + @hookform/resolvers 5.9, vitest 5 + jsdom 30 + @testing-library/*, msw 3.0.2 (pin),
  eslint 10 + typescript-eslint 8.71 + eslint-plugin-react-hooks 7 (flat), lucide-react 1.x, sonner 2.0.8,
  tw-animate-css, dompurify 3.4, @fontsource/{inter-tight,manrope,roboto} 5.3 (subset vietnamese).
- Không dark mode, một vai trò cho mỗi user, admin không vào `/teach/*` (quyết định plan.md §11).

## Design system / Architecture

### Token và theme

- `src/styles/goup-tokens.css`: chép nguyên khối `:root` của `prototype/goup.css` (navy-900 `#0a0d53`, navy-700 `#233c65`,
  navy-deep, accent-orange `#f45d29`, accent-crimson `#f2295b`, slate-600, gray-800/600/500/300/200,
  surface-0/50/100/blue-50/blue-100/dark, on-accent, `--font-sans/display/ui`, `--space-1..8`
  2/6/10/18/24/35/60/100px, `--radius-xs/sm/dot/pill/round`, `--shadow-card/card-strong/float/control`,
  `--gradient-accent`, `--gradient-navy`). Ghi chú đầu file: không sửa tay, đổi ở design system.
- `src/styles/app.css`: `@import "tailwindcss"; @import "tw-animate-css"; @import "./goup-tokens.css";` rồi:
  - `:root` token sản phẩm (lấy từ `prototype/styles.css`): `--ink: var(--navy-900)`, `--ink-2: var(--gray-800)`,
    `--ink-3: var(--gray-600)`, `--line: var(--gray-200)`, `--ok #1f7a3f`, `--warn #a35a00`, `--danger #b42318`,
    `--ok-bg #e8f5ec`, `--warn-bg #fff4e5`, `--danger-bg #fdeceb`, `--container 1170px`, `--control-h 40px`; ngữ nghĩa
    shadcn: `--background: var(--surface-50)`, `--foreground: var(--ink-2)`, `--card: var(--surface-0)`, `--primary:
    var(--navy-700)`, `--secondary: var(--surface-blue-50)`, `--muted: var(--surface-100)`, `--border:
    var(--gray-200)`, `--input: var(--gray-300)`, `--ring: var(--navy-700)`, `--destructive: var(--danger)`.
  - `@theme inline` ánh xạ sang Tailwind: `--color-navy-900/700/deep`, `--color-accent-orange/crimson`,
    `--color-ink/ink-2/ink-3`, `--color-ok/ok-bg/warn/warn-bg/danger/danger-bg`, `--color-on-accent`,
    `--color-surface-0/50/100/blue-50/blue-100/dark`, `--color-background/foreground/card/primary/...`; font ghi
    literal `--font-sans: "Inter Tight", sans-serif`, `--font-display: Manrope, sans-serif`, `--font-ui: Roboto,
    sans-serif` (không `var(--font-sans)` tự tham chiếu); `--radius-xs: 2px`, `--radius-sm/md/lg/xl: 3px`,
    `--radius-pill: 50px`; shadow trùng goup; `--text-xs 12px / sm 13px / md 15px / lg 18px / xl 24px / 2xl 32px`;
    `--ease-brand: cubic-bezier(.2,.7,.2,1)`.
  - `@theme { --animate-pop: pop 180ms var(--ease-brand); --animate-slide; --animate-rise; @keyframes pop/slide/rise
    }` (chép từ styles.css).
  - `@layer base`: `body` bg surface-50, text ink-2, `font-sans` 15px, `letter-spacing: .2px`; `h1` `font-display`;
    `:focus-visible { outline: 2px solid var(--navy-700); outline-offset: 2px }`; `@media (prefers-reduced-motion:
    reduce) { *, *::before, *::after { animation: none !important; transition: none !important } }`.
  - Giữ spacing mặc định Tailwind (`p-4` = 16px); không `--color-*: initial`. Bẫy: `--font-sans`, `--radius-*`,
    `--shadow-*` ở `:root` unlayered của goup thắng `@layer theme`, nên giá trị trong `@theme` phải trùng goup từng ký
    tự.
- Font: `main.tsx` import `@fontsource/inter-tight/vietnamese-{400,500,600,700}.css`,
  `@fontsource/inter-tight/vietnamese-400-italic.css`, `@fontsource/manrope/vietnamese-{600,700}.css`,
  `@fontsource/roboto/vietnamese-{400,700}.css`. Không dùng Google Fonts CDN.
- `index.html`: `lang="vi"`, `<title>GoUp LMS</title>`, `meta color-scheme light`, `theme-color #0a0d53`, favicon SVG
  inline giống prototype, `noscript` "Cần bật JavaScript".

### shadcn và bảng chỉnh cva (`src/shared/ui`)

Chạy `pnpm dlx shadcn@latest init -b radix` rồi `add button badge alert input textarea select checkbox label dialog
sheet dropdown-menu tooltip progress skeleton sonner table`. Xóa `next-themes` khỏi `sonner.tsx`. Chỉnh:

| Component | Chỉnh theo design.md |
|---|---|
| `button` | base `rounded-full uppercase font-medium text-[13px] leading-[13px] tracking-[1.3px] min-h-11 px-[35px] py-[18px] active:translate-y-px disabled:opacity-45 focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-navy-700`; variants `default` (navy: `bg-navy-700 text-white hover:bg-navy-900`), `gradient` (`bg-navy-700 bg-(image:--gradient-accent) text-on-accent hover:brightness-106`), `outline` (`border border-navy-700 bg-white text-navy-700 hover:bg-surface-blue-50`), `ghost` (`px-3 text-navy-700 hover:bg-surface-blue-50`), `destructive` (`bg-destructive text-white hover:brightness-92`), `link`; size `sm` `min-h-9 px-4 py-2.5 text-xs tracking-[1px]`, `icon` `size-11 rounded-sm`. |
| `badge` | vuông, `text-[11px] leading-[14px] tracking-[.5px] px-1.5 py-0.5`; variants `solid` (navy), `outline` (viền navy), `muted` (surface-100 + gray-300), `warn` (warn-bg, viền `#f0c48a` khai báo token `--warn-line`), `subtle` (surface-100 text gray-600). |
| `alert` | vuông, `text-sm px-4 py-3 border flex gap-3`; variants `info/warn/danger/ok` theo `.alert-*`; slot `AlertActions`. |
| `input`, `textarea`, `select` | `h-10 rounded-xs border-input bg-white text-[16px]`, focus `border-navy-700 ring-3 ring-surface-blue-100`, `aria-invalid:border-destructive`, disabled bg surface-100; textarea `font-mono text-sm min-h-35`. |
| `dialog` | `rounded-none shadow-float w-[min(560px,calc(100vw-32px))] max-h-[calc(100vh-48px)]`, overlay `bg-navy-900/45`, `animate-pop`; close button `aria-label="Đóng"`. |
| `sheet` | side right `w-[min(520px,100vw)]`, `animate-slide`; full width khi ≤960. |
| `progress` | track `h-1.5 bg-surface-blue-100 rounded-none`, indicator `bg-navy-700`; prop `size="lg"` → `h-2.5`. |
| `dropdown-menu`, `select`, `tooltip` | `rounded-sm shadow-float`; tooltip `bg-surface-dark text-white`. |
| `sonner` | `<Toaster position="bottom-right" duration={4000} visibleToasts={3} closeButton containerAriaLabel="Thông báo" offset={24} mobileOffset={16} toastOptions={{unstyled:true, classNames:{toast:'bg-surface-dark text-white shadow-float border-l-3 border-ok-line px-4 py-3 text-sm flex gap-3', error:'border-accent-crimson', closeButton:'...'}}} />`. |

Component bổ sung (không có trong shadcn, viết tay trong `shared/ui`): `PageHead` (title, lede, actions, badges,
crumbs), `Crumbs` (`nav aria-label="Đường dẫn"`, item cuối `aria-current="page"`), `EmptyState` (title, text, action),
`Field` (label + `.req` dấu `*` màu crimson, help, error, nối `aria-describedby`/`aria-invalid`), `ProgressBar`
(`role="img" aria-label="{pct}% hoàn thành"` + số `%` tabular), `StatusDot` (variants
ok/queued/failed/invited/disabled/dropped), `VersionPill` + `Lineage` (link `?v=`, `aria-current="true"` cho bản đang
xem, dashed cho draft, hậu tố "nháp"), `IconButton` (size 44, `aria-label` bắt buộc), `TableWrap` (`overflow-x-auto`,
`th` sticky uppercase 12px), `KpiCard`, `ConfirmDialog` (title, text, confirm, cancel="Hủy", danger, loading),
`FormDialog` (RHF + zod, lỗi chung trong `Alert danger role="alert"`, autofocus input đầu), `MarkdownContent`,
`TypeIcon` (play/file-text, `title` Video/Markdown). Icon chỉ dùng `lucide-react`, không emoji.

### Thư mục và layering

```
apps/web/src/
  app/        main.tsx, router.tsx, providers.tsx, middleware.ts, query-client.ts,
              layouts/{AppShell,AuthLayout}.tsx, route-announcer.tsx, not-found.tsx
  features/<f>/{api/,model/,hooks/,components/,pages/,routes.tsx,index.ts}
  shared/     api/{http.ts,errors.ts,schemas.ts}, ui/*, lib/{format.ts,cn.ts}, domain/*,
              hooks/*, layout/*, test/{setup.ts,msw.ts,render.tsx}
  styles/     goup-tokens.css, app.css
```

- `shared/domain`: `Role` (`admin|teacher|student`), `homeOf(role)` → `/admin`, `/teach`, `/learn`; `VersionStatus` +
  `canTransition` (draft→published, published→archived); `ClassStatus` (draft→active→ended); `STATUS_VI` chép nguyên
  từ `prototype/app.js`; `Percent` branded + `percentOf(done,total)`.
- `shared/lib/format.ts`: `fmtDate` (dd/mm/yyyy), `fmtDateTime` (dd/mm/yyyy HH:mm), `rel(iso)` ("Vừa xong", "{n} phút
  trước", "{n} giờ trước", "{n} ngày trước", quá 30 ngày → `fmtDate`; rỗng → "Chưa có"), `versionLabel(v)` → `v{no}`.
- ESLint `no-restricted-imports`: cấm `@/features/*/*` (chỉ import qua `@/features/<f>` index) và cấm
  `react-router-dom`. `model/` không import React/fetch.

### Hợp đồng `routes.tsx` (phase 11–13 bám theo)

```ts
// src/features/<feature>/routes.tsx
import type { RouteObject } from 'react-router';
export const routes: RouteObject[] = [
  { path: 'stages', lazy: () => import('./pages/stages-page'), handle: { title: 'Chặng' } },
];
```

- `app/router.tsx` gom: `publicRoutes` (AuthLayout, không middleware) ← `auth.routes`; `adminRoutes` (`/admin`,
  middleware `[authMiddleware, requireRole('admin')]`, element `AppShell`) ← `dashboard.routes + stages.routes +
  courses.routes + classes.routes` (nhánh classes cũng dùng `reports` components); `teachRoutes` (`/teach`,
  `requireRole('teacher')`) ← `reports.teachRoutes`; `learnRoutes` (`/learn`, `requireRole('student')`) ←
  `learning.routes`. Mỗi feature export `routes` (và `teachRoutes` nếu cần) với `path` tương đối, không chứa tiền tố
  `/admin`.
- `handle.title` là chuỗi hoặc `(data) => string`; `RouteAnnouncer` đọc qua `useMatches()`, đặt `document.title =
  "{title} · GoUp LMS"`, ghi vào `p.sr-only[aria-live=polite]`, focus `h1` sau điều hướng.
- Page component export `default` (cho `lazy`) hoặc `Component`; loader không bắt buộc, data lấy qua React Query trong
  page.

### HTTP và React Query

- `shared/api/http.ts`: `http<T>(path, {method, body, schema})` gọi `fetch('/api/v1'+path, {credentials:'same-origin',
  headers:{'Content-Type':'application/json','X-Requested-With':'fetch'}})` — header `X-Requested-With` gửi **mọi**
  request (kể cả GET) vì API kiểm nó cùng `http.CrossOriginProtection` (phase 4) <!-- Red Team: RT-05 - luôn gửi X-Requested-With -->; đọc envelope lỗi `{"error":{code,message,details}}` → `throw new
  ApiError(status, code, message, details)`; 204 → `undefined`; có `schema` thì `schema.parse(json)` (zod ở biên).
  Không tự redirect.
- `app/query-client.ts`: `QueryClient` với `QueryCache.onError`/`MutationCache.onError`: 401 → `queryClient.clear()`
  rồi `setQueryData(meKey, null)` + `router.navigate('/login?next='+pathname)` (bỏ qua khi đang ở `/login`), để dữ
  liệu của người dùng trước không còn trong cache khi người khác đăng nhập trên cùng tab <!-- Red Team: RT-09 - clear cache khi 401 -->; 403 `PASSWORD_CHANGE_REQUIRED` →
  `/first-login`; retry `(n,e) => !(e instanceof ApiError && e.status < 500) && n < 2`; `staleTime` 30s mặc định.
  Toast lỗi mặc định `toast.error(e.message)` cho mutation khi caller không tự xử lý (`meta.silent` để tắt).
- `middleware.ts`: `authMiddleware` dùng `queryClient.ensureQueryData(meQuery)` (`queryFn` trả `null` khi 401); chưa
  đăng nhập → `redirect('/login?next=…')`; `mustChangePassword && path !== '/first-login'` →
  `redirect('/first-login')`; set `userContext`. `requireRole(role)` → sai vai trò `redirect(homeOf(user.role))`.
  Route public khi đã đăng nhập → `homeOf` hoặc `/first-login`. Index `/` → redirect `homeOf`; `*` → `NotFound`
  ("Không tìm thấy trang", link về trang chủ vai trò).
- Vite dev proxy `/api` → `http://localhost:8080`; CSRF hai lớp do server kiểm: `http.CrossOriginProtection`
  (`Sec-Fetch-Site`/`Origin`) **và** header `X-Requested-With: fetch`; JS không giữ token. <!-- Red Team: RT-05 -->

## Pages & components

### AppShell (`app/layouts/AppShell.tsx`) — 1:1 với shell prototype

- Skip link `<a href="#main" className="sr-only focus:not-sr-only …">Bỏ qua điều hướng</a>`.
- `header.topbar` nền navy-700: `IconButton.nav-toggle` (`aria-label="Mở menu"`, `aria-expanded`, chỉ hiện ≤720),
  `a.brand` → `homeOf` với `BrandMark` SVG + "GoUp" + `<span>LMS</span>`; `nav#nav aria-label="Chính"` với NAV theo
  vai trò: admin `[['/admin','Tổng quan'],['/admin/stages','Chặng'],['/admin/courses','Khóa
  học'],['/admin/classes','Lớp học']]`, teacher `[['/teach','Lớp của tôi']]`, student `[['/learn','Lớp của tôi']]`;
  `aria-current="page"` khi path bằng href hoặc bắt đầu bằng `href+'/'` (trang chủ chỉ khớp chính xác); viền dưới
  accent-orange khi current. `topbar-end`: `.account` (avatar chữ cái đầu, tên, `small` vai trò "Admin"/"Giảng
  viên"/"Học viên"), `IconButton` `aria-label="Đăng xuất"` (log-out) → `POST /auth/logout`,
  `setQueryData(meKey,null)`, `queryClient.clear()`, navigate `/login`; lỗi mạng vẫn chuyển về `/login`.
- `main#main.main tabIndex={-1}` max-width 1170px, `<Outlet/>`; `RouteAnnouncer`; `Toaster`.
- ≤720: nav thu thành drawer (Sheet trái, full width), đóng khi chọn link hoặc Escape, focus trả về nút toggle.

### AuthLayout (`app/layouts/AuthLayout.tsx`)

`main.auth-main` căn giữa, `Card.auth-card` 440px, `.auth-brand` (BrandMark 36px + "GoUp LMS"), `.auth-tagline` "Nền
tảng học nội bộ của GoUp: chặng → khóa học có phiên bản → lớp học → tiến độ học viên." Không topbar, vẫn có
`RouteAnnouncer` + `Toaster`.

### Feature `auth` (phase 10 điền `features/auth/routes.tsx`)

| Route | Trang | Nội dung |
|---|---|---|
| `/login` | `LoginPage` (title "Đăng nhập") | h1 "Đăng nhập", lede "Dùng email và mật khẩu trong thư mời của bạn." Form `stack-sm`: `Field` Email (`type=email autocomplete=username required`), Mật khẩu (`type=password autocomplete=current-password required`), `Alert danger role=alert` lỗi chung, hàng `row-between`: link "Quên mật khẩu?" → `/forgot` + `Button variant=gradient` "Đăng nhập" (gradient duy nhất của trang). Submit → `POST /auth/login {email,password}`; thành công → `queryClient.clear()` rồi `setQueryData(meKey, user)` rồi `navigate(mustChangePassword ? '/first-login' : (next ?? homeOf(role)))`; `next` chỉ nhận path nội bộ bắt đầu bằng `/` (chặn open redirect). Lỗi: hiện nguyên văn `error.message` của server trong `Alert danger` (phase 4 là nguồn copy: `UNAUTHENTICATED` "Email hoặc mật khẩu không đúng.", `ACCOUNT_DISABLED` "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên.", `TEMP_PASSWORD_EXPIRED` "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên.", `RATE_LIMITED` "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút."); FE không viết lại copy, chỉ fallback "Không kết nối được máy chủ. Thử lại sau." khi lỗi mạng. Nút disabled + "Đang đăng nhập…" khi pending. <!-- Red Team: RT-09 - copy lỗi từ server --> |
| `/forgot` | `ForgotPage` (title "Quên mật khẩu") | h1 "Quên mật khẩu", lede "Nhập email đăng nhập. Nếu email tồn tại, bạn sẽ nhận được đường dẫn đặt lại mật khẩu, hiệu lực 30 phút." Field Email; nút `gradient` "Gửi đường dẫn" + link "Quay lại đăng nhập" → `/login`. Submit `POST /auth/forgot-password {email}`; luôn (kể cả 2xx cho email lạ) hiện `Alert ok` "Nếu email tồn tại trong hệ thống, chúng tôi đã gửi đường dẫn đặt lại mật khẩu. Vui lòng kiểm tra hộp thư." và xóa ô email. `RATE_LIMITED` → Alert danger "Bạn gửi quá nhiều yêu cầu. Thử lại sau ít phút." |
| `/reset-password#token=` | `ResetPasswordPage` (title "Đặt lại mật khẩu") | Token nằm ở **fragment** (`location.hash`, không vào query string nên không lọt log nginx/proxy/Referer): khi mount đọc `new URLSearchParams(location.hash.slice(1)).get('token')` vào state rồi `history.replaceState(null, '', '/reset-password')` để xóa khỏi URL. Thiếu `token` → Alert danger "Đường dẫn không hợp lệ. Yêu cầu đường dẫn mới." + link `/forgot`. Fields "Mật khẩu mới" (`new-password`, min 8, help "Tối thiểu 8 ký tự."), "Nhập lại mật khẩu mới"; nút `gradient` "Đặt lại mật khẩu". `POST /auth/reset-password {token,newPassword,confirmPassword}`; thành công → toast "Đã đặt lại mật khẩu. Hãy đăng nhập bằng mật khẩu mới." → `/login`. Lỗi server hiện nguyên văn `error.message` (`NOT_FOUND` token "Đường dẫn đã hết hạn hoặc đã dùng. Yêu cầu đường dẫn mới.", `ACCOUNT_DISABLED`, `VALIDATION_FAILED`). Client-side chặn sớm: "Mật khẩu mới cần tối thiểu 8 ký tự.", "Hai mật khẩu không khớp." <!-- Red Team: RT-09 - token trong fragment, confirmPassword, copy từ server --> |
| `/first-login` | `FirstLoginPage` (title "Đặt mật khẩu mới"; cần đăng nhập, nếu `!mustChangePassword` → `homeOf`) | h1 "Đặt mật khẩu của bạn", lede "Xin chào {name}. Trước khi vào lớp, hãy thay mật khẩu tạm bằng mật khẩu riêng của bạn. Mật khẩu tạm còn hiệu lực đến {fmtDateTime(tempPasswordExpiresAt)}." Hai field: "Mật khẩu mới" (`new-password`, help "Tối thiểu 8 ký tự."), "Nhập lại mật khẩu mới" — **không** hỏi mật khẩu tạm vì người dùng vừa đăng nhập bằng nó (phase 4: `currentPassword` chỉ bắt buộc khi tài khoản `active`); hàng: `Button variant=link` "Đăng xuất" + `gradient` "Lưu mật khẩu". `POST /auth/change-password {newPassword,confirmPassword}`; thành công → `invalidateQueries(meKey)`, toast "Đã lưu mật khẩu. Chào mừng bạn vào lớp." → `homeOf`. Lỗi client: "Mật khẩu mới cần tối thiểu 8 ký tự.", "Hai mật khẩu không khớp."; lỗi server hiện nguyên văn `error.message` (`VALIDATION_FAILED` khi trùng mật khẩu tạm, `TEMP_PASSWORD_EXPIRED` "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên."). <!-- Updated: Validation Session 1 - V4 first-login hai field --> |

a11y chung auth: mỗi `Field` nối `label htmlFor`, lỗi field `aria-describedby` + `aria-invalid`; lỗi chung trong
`Alert role="alert"`; nút submit không đổi kích thước khi loading; Enter submit; toàn trang 320px không tràn ngang.

## Files to Create / Modify

- Gốc: `apps/web/package.json`, `vite.config.ts` (plugin react, tailwind, alias `@`, proxy `/api`, `test` config
  vitest jsdom + `setupFiles`), `tsconfig.json`/`tsconfig.app.json` (strict, `verbatimModuleSyntax`, paths `@/*`),
  `eslint.config.js` (ts-eslint strict-type-checked, react-hooks flat recommended, `no-restricted-imports`),
  `components.json`, `index.html`, `.env.example` (`VITE_API_PROXY=http://localhost:8080`), `playwright.config.ts`
  (khung, phase 14 dùng), `nginx.conf` (SPA fallback, proxy `/api`; `log_format` chỉ ghi `$request_method $uri $status`,
  **không** `$args`/`$request`, để token/email không vào access log <!-- Red Team: RT-09 - nginx không log query -->),
  `scripts/design-audit.mjs` (`pnpm lint:design`: quét `src/` tìm hex thô ngoài `goup-tokens.css`/`app.css`, emoji,
  cỡ chữ <12px, hit area <44px — đây là cách kiểm tiêu chí plan.md §3; phase 14 thêm `render-check` ba viewport
  <!-- Updated: Validation Session 1 - V7 giữ design-audit -->).
- `src/styles/goup-tokens.css`, `src/styles/app.css`.
- `src/app/{main.tsx,providers.tsx,router.tsx,middleware.ts,query-client.ts,route-announcer.tsx,not-found.tsx}`,
  `src/app/layouts/{AppShell,AuthLayout}.tsx`.
- `src/shared/api/{http.ts,errors.ts,schemas.ts}` (schemas chung: `userSchema`, `errorEnvelopeSchema`, `isoDate`),
  `src/shared/domain/{role.ts,status.ts,percent.ts,index.ts}`, `src/shared/lib/{format.ts,cn.ts}`,
  `src/shared/hooks/{use-media-query.ts,use-document-title.ts}`, `src/shared/ui/*` (shadcn đã chỉnh + component bổ
  sung), `src/shared/layout/{brand-mark.tsx,nav.tsx}`, `src/shared/test/{setup.ts,msw.ts,render.tsx}`.
-
  `src/features/auth/{api/auth-api.ts,model/schemas.ts,hooks/{use-me.ts,use-login.ts,use-logout.ts},pages/{login-page,forgot-page,reset-password-page,first-login-page}.tsx,routes.tsx,index.ts}`.
- Stub `routes.tsx` + `index.ts` cho `dashboard`, `stages`, `courses`, `classes`, `reports`, `learning` (chỉ export
  `routes = []`; phase 11–13 ghi đè).
- Không tạo `apps/web/README.md`; hợp đồng `routes.tsx` và quy tắc sở hữu file ghi ở comment đầu `src/app/router.tsx`
  và `docs/architecture.md` (Phase 1 sở hữu), lệnh dev/test/lint nằm trong `package.json` scripts + README gốc.
  <!-- Updated: Validation Session 1 - V7 bỏ README web -->

## Tasks & Steps

1. Scaffold: `pnpm create vite apps/web --template react-ts`, cài phụ thuộc đúng phiên bản ở trên, cấu hình
   `vite.config.ts`, alias, proxy, vitest; xác nhận `pnpm dev` và `pnpm build` chạy.
2. Tailwind 4 + token: viết `goup-tokens.css` (chép goup `:root`), `app.css` theo mục Design system; import font
   fontsource; kiểm tra bằng trang tạm render các token màu/font/radius so với `prototype/index.html` cạnh nhau.
3. shadcn init `-b radix` + add component; áp bảng chỉnh cva; viết component bổ sung; viết `scripts/design-audit.mjs`
   và nối `pnpm lint:design`; so trực quan với prototype bằng chính bốn trang auth + `AppShell` (không có route
   `/__ui`). <!-- Updated: Validation Session 1 - V7 bỏ /__ui -->
4. `shared/api/http.ts`, `errors.ts`, `schemas.ts`; unit test envelope lỗi, 204, và (MSW) mọi request đều mang
   `X-Requested-With: fetch`. <!-- Red Team: RT-05 -->
5. `shared/domain` + `shared/lib/format.ts` với test (`rel`, `fmtDate`, `percentOf`, `canTransition`).
6. `query-client.ts` (onError 401/403, retry), `providers.tsx` (`QueryClientProvider`, `Toaster`), `middleware.ts`,
   `router.tsx` gom routes theo hợp đồng; `RouteAnnouncer`, `NotFound`.
7. `AppShell`, `AuthLayout`, `BrandMark`, nav theo vai trò, drawer mobile, nút đăng xuất.
8. Feature `auth`: api + zod schema (`loginResponse`, `me`), hooks, bốn trang theo bảng; `routes.tsx` cho `auth`; stub
   `routes.tsx` sáu feature còn lại.
9. MSW handlers cho auth trong `shared/test/msw.ts`; test component cho `LoginPage` (lỗi 401 hiện đúng `message`
   server, redirect `next`, `mustChangePassword`; đăng nhập A → cache có query của A → 401 → đăng nhập B → không còn
   dữ liệu của A trong `queryClient`), `ResetPasswordPage` (đọc `#token=`, URL được `replaceState` sạch, body có
   `confirmPassword`), `FirstLoginPage` (hai lỗi client, body không có `currentPassword`), middleware (redirect theo
   vai trò). <!-- Red Team: RT-09 -->
10. ESLint + typecheck + `lint:design` + build sạch; chạy docker compose phase 1 để kiểm `nginx.conf` proxy `/api`,
    SPA fallback, và access log không chứa `?token=`/`#token=` khi mở link đặt lại mật khẩu. <!-- Red Team: RT-09 -->

## Verification

- `pnpm -C apps/web lint && pnpm -C apps/web lint:design && pnpm -C apps/web typecheck && pnpm -C apps/web test && pnpm -C apps/web build` đều xanh.
- Vitest: `http.ts` (envelope, 204, zod parse fail → lỗi rõ, header `X-Requested-With` trên GET lẫn POST), `format.ts`, `domain`, `middleware` (chưa đăng nhập →
  `/login?next=/admin/stages`; teacher vào `/admin` → `/teach`; `mustChangePassword` → `/first-login`), `LoginPage`,
  `FirstLoginPage` (hai field), `ResetPasswordPage` (fragment token), `ForgotPage` (luôn hiện Alert ok), `query-client`
  (401 → `clear()`; login thành công → `clear()` trước khi set `me`).
- Thủ công với API phase 4 chạy thật: đăng nhập `quan.tran@goup.vn` (seed) → `/admin`; đăng nhập học viên `invited`
  (`minh.bui@gmail.com`, mật khẩu tạm lấy từ Mailpit) → `/first-login` → đổi (hai field) → `/learn`; quên mật khẩu →
  mở link Mailpit → URL là `/reset-password#token=…`, sau khi trang mount thanh địa chỉ chỉ còn `/reset-password`; logout → `/login`; mở `/admin/stages` khi chưa đăng nhập → quay lại đúng trang sau
  login; teacher gõ `/admin` → `/teach`.
- a11y: axe DevTools 0 lỗi nghiêm trọng trên `/login`, `/first-login`, shell admin; Tab đi qua skip link → brand → nav
  → account → logout; `RouteAnnouncer` đọc tiêu đề khi đổi route; 320px không tràn ngang; `prefers-reduced-motion` tắt
  animation.
- Trực quan: bốn trang auth + `AppShell` so với prototype: button 44px pill uppercase, input 40px radius 2px ring 3px
  blue-100, badge vuông 11px, card shadow `0 0 7px rgba(0,0,0,.15)`, topbar navy-700, link current viền orange;
  `pnpm lint:design` báo 0 vi phạm. <!-- Updated: Validation Session 1 - V7 -->

## Security notes

- Cookie phiên `HttpOnly` do server đặt; JS không đọc token, không lưu `localStorage`. `fetch` dùng `credentials:
  'same-origin'` + `X-Requested-With: fetch`; không gọi cross-origin.
- Token đặt lại mật khẩu chỉ tồn tại trong fragment rồi được xóa bằng `replaceState`; không đưa vào query string,
  `Referer`, hay React Router state có thể serialize. <!-- Red Team: RT-09 -->
- `queryClient.clear()` ở 401 và sau login thành công: không rò dữ liệu giữa hai người dùng trên cùng tab.
- `next` sau login chỉ chấp nhận path tương đối bắt đầu bằng một `/` (từ chối `//`, `http:`), mặc định `homeOf(role)`.
- Thông điệp lỗi login/forgot không tiết lộ email có tồn tại hay không (forgot luôn hiện Alert ok).
- `MarkdownContent` sanitize bằng DOMPurify (`USE_PROFILES:{html:true}`, hook thêm `rel="noopener noreferrer"` cho
  `target=_blank`); không render HTML thô ở đâu khác.
- Không log body request/response chứa mật khẩu; React Query devtools chỉ ở DEV.

## Risks & Rollback

- **Tailwind 4 + goup `:root` xung đột** (font/radius/shadow): giảm thiểu bằng giá trị trùng khớp, `lint:design` và
  so trực quan các trang auth; rollback: đổi `goup-tokens.css` thành `@layer base` nếu phát hiện thứ tự cascade sai.
- **React Router v8 API đổi** (middleware, `react-router/dom`): pin `8.4.x`; nếu middleware gây lỗi chưa rõ, fallback
  tạm `loader` kiểm auth trên route cha, giữ nguyên hợp đồng `routes.tsx`.
- **shadcn CLI v4 sinh code khác kỳ vọng**: commit riêng bước `init/add` trước khi chỉnh để diff rõ; rollback bằng
  revert commit đó.
- **MSW 3 ESM-only** lỗi với jsdom: pin `3.0.2`, bật `onUnhandledRequest: 'error'`; fallback msw 2.x nếu không tương
  thích vitest 5.
- **Thiếu `routes.tsx` stub** làm phase 11–13 chặn nhau: tạo stub trong task 8 và kiểm `pnpm build` trước khi bàn
  giao.
- Rollback toàn phase: `apps/web` là thư mục mới, xóa thư mục và entry trong `pnpm-workspace.yaml`/compose là đủ.

## Success Criteria

- [x] `apps/web` build, lint, typecheck, test xanh; dev server proxy được tới API phase 4. <!-- build/lint/lint:design/typecheck/test xanh; dev server proxy tới API thật đã kiểm bằng Playwright -->
- [x] Token GoUp phủ 100% `goup.css`; mọi component `shared/ui` khớp bảng chỉnh và `docs/design.md` (không hex thô ngoài
  `goup-tokens.css`/`app.css`, không chữ <12px, hit area 44px, một gradient mỗi trang).
- [x] Bốn trang auth chạy thật với API: login, forgot, reset, first-login; redirect theo vai trò và `mustChangePassword`
  đúng; 401 từ bất kỳ query nào `clear()` cache rồi đẩy về `/login?next=`; mọi request mang `X-Requested-With: fetch`;
  reset-password đọc token từ fragment và access log nginx không chứa token. <!-- đã kiểm bằng MSW và bằng Playwright với API thật; access log nginx (profile full) không chứa token -->
- [x] `AppShell` đúng nav theo vai trò, `aria-current`, skip link, `RouteAnnouncer`, drawer ≤720.
- [x] Bảy file `features/<f>/routes.tsx` tồn tại và được `app/router.tsx` import; comment đầu `router.tsx` và
  `docs/architecture.md` ghi rõ hợp đồng và quy tắc sở hữu file để phase 11–13 chạy song song không đụng `src/app`,
  `src/shared`, `src/styles`. <!-- Updated: Validation Session 1 - V7 -->
