# Nghiên cứu: stack frontend React cho LMS MVP

Ngày 2026-10-05. Phiên bản lấy trực tiếp từ npm registry hôm nay [S1]. Mọi khuyến nghị đã đối chiếu `prototype/goup.css`, `prototype/styles.css`, `docs/design.md`, spec và báo cáo backend Go.

## Kết luận chính

- **Ba giả định của đề bài đã cũ.** React Router đã lên v8 (2026-06-17), ESLint lên v10, MSW lên v3. Khuyến nghị dùng bản mới vì cả ba tương thích với nhau.
- **Không dùng TypeScript 7.** typescript-eslint 8.71 yêu cầu `typescript >=4.8.4 <6.1.0` [S2], issue hỗ trợ tsgo vẫn mở [S3]. Template chính thức `create-vite` react-ts cũng ghim `~6.0.2` [S4]. Dùng `typescript ~6.0.3`.
- **shadcn đổi mặc định sang Base UI từ 07/2026.** Phải chạy `shadcn init -b radix` để giữ Radix như dự án sidecup và dùng gói gộp `radix-ui` [S5][S6].
- **Design system ánh xạ trọn vẹn được** qua `:root` + `@theme inline`. Thang spacing 4px của prototype trùng với spacing mặc định của Tailwind, nên không cần token spacing riêng.
- **Video không thấy được mã 403.** `<video>` chỉ báo `MediaError` mã 2 hoặc 4 [S7]. Client phải coi mọi lỗi phát là "có thể URL hết hạn", xin URL mới và tua lại vị trí cũ.

## 1. Phiên bản (registry ngày 2026-10-05)

| Gói | Mới nhất | Ghim đề xuất | Ghi chú |
|---|---|---|---|
| react, react-dom, @types/react(-dom) | 19.3.0 | `^19.3.0` | RR v8 yêu cầu React >=19.2.7 |
| vite, @vitejs/plugin-react | 8.3.2, 6.1.1 | `^8.3.2`, `^6.1.1` | Vite 8 ra 2026-03-12; plugin có peer `vite ^8`. React Compiler bản Rust còn experimental, bỏ qua cho MVP [S8] |
| typescript | 7.0.2 | **`~6.0.3`** | TS 7 chưa được typescript-eslint hỗ trợ |
| tailwindcss, @tailwindcss/vite | 4.3.3 | `^4.3.3` | Peer `vite ^5.2–^8`. Cú pháp `bg-(image:--var)` đã có trong docs [S28] |
| shadcn (CLI) | 4.21.1 | dùng `pnpm dlx shadcn@latest` | CLI v4 từ 2026-03 [S9] |
| radix-ui, cn | 1.6.7, 0.4.0 | do CLI cài | `radix-ui` gộp thay `@radix-ui/react-*` từ 02/2026 [S6]. Từ 09/2026 component import `cn` từ gói `cn`, `lib/utils.ts` vẫn re-export [S10] |
| react-router | 8.4.0 | `^8.4.0` | v7 còn ở tag `version-7` (7.18.4). Gói `react-router-dom` đã bị xóa [S11] |
| @tanstack/react-query (+devtools) | 5.104.1 | `^5.104.1` | |
| react-hook-form, zod, @hookform/resolvers | 7.89.0, 4.6.5, 5.9.1 | `^` theo bản mới nhất | resolvers có peer `zod ^3.25 \|\| ^4` |
| vitest, jsdom | 5.0.3, 30.1.2 | `^5.0.3`, `^30.1.2` | Vitest 5 ra 2026-09-03, peer `vite ^6–^8` |
| @testing-library/react / user-event / jest-dom, @playwright/test | 16.3.3 / 14.6.7 / 7.0.1, 1.63.0 | như cột trái | |
| eslint, @eslint/js, typescript-eslint | 10.12.0, 10.0.1, 8.71.0 | `^` theo bản mới nhất | ESLint 9 chỉ còn tag `maintenance` (9.39.5); v10 chỉ có flat config. typescript-eslint có peer `eslint ^8.57–^10`. Template `create-vite` đã chuyển sang oxlint [S4], nhưng vẫn giữ ESLint vì cần luật có kiểu |
| eslint-plugin-react-hooks / -react-refresh | 7.1.1 / 0.5.7 | như cột trái | Dùng `reactHooks.configs.flat.recommended` [S12] |
| msw | 3.0.2 | `3.0.2` (ghim cứng) | v3 ra 2026-09-28, ESM-only, Node >=22, `onUnhandledRequest` đổi thành `onUnhandledFrame` [S13]. Dự phòng: 2.15.0 |
| lucide-react, sonner, tw-animate-css, dompurify | 1.52.0, 2.0.8, 1.4.0, 3.4.16 | `^` theo bản mới nhất | lucide-react đã qua 1.0 từ 2026-03 |
| @fontsource/inter-tight, manrope, roboto | 5.3.0 | `^5.3.0` | Bản tĩnh, family trùng tên trong goup.css; cả ba có subset `vietnamese` [S14] |

## 2. Design system trong Tailwind v4 + shadcn

**Khuyến nghị: ba lớp CSS, sửa variant `cva` trong `src/shared/ui/*`, cấm override ở nơi gọi.**

1. `src/styles/goup-tokens.css`: chép nguyên khối `:root` của `goup.css`, không sửa (file gốc ghi "Do not edit here").
2. `src/styles/index.css`: token sản phẩm của `styles.css` và biến ngữ nghĩa shadcn trong `:root`.
3. `@theme inline` ánh xạ sang utility. Phải dùng `inline` vì giá trị tham chiếu biến khác; không có `inline` thì biến có thể phân giải sai phạm vi [S15]. shadcn theming hướng dẫn đúng mẫu này, kể cả thêm màu mới như `--warning` [S16].

```css
@import "tailwindcss";
@import "tw-animate-css";
@import "./goup-tokens.css";
:root {
  --ink: var(--navy-900); --ink-2: var(--gray-800); --ink-3: var(--gray-600);
  --ok: #1f7a3f; --warn: #a35a00; --danger: #b42318;
  --ok-bg: #e8f5ec; --warn-bg: #fff4e5; --danger-bg: #fdeceb;
  --background: var(--surface-50); --foreground: var(--ink-2);   /* body của prototype */
  --card: var(--surface-0); --card-foreground: var(--ink-2);
  --popover: var(--surface-0); --popover-foreground: var(--ink-2);
  --primary: var(--navy-700); --primary-foreground: var(--surface-0);
  --secondary: var(--surface-blue-50); --secondary-foreground: var(--navy-700);
  --muted: var(--surface-100); --muted-foreground: var(--ink-3);
  --accent: var(--surface-blue-50); --accent-foreground: var(--navy-700);
  --destructive: var(--danger);
  --border: var(--gray-200); --input: var(--gray-300); --ring: var(--navy-700);
}
@theme inline {
  /* giữ nguyên các dòng --color-background … --color-ring do shadcn sinh, thêm: */
  --color-navy-900: var(--navy-900); --color-navy-700: var(--navy-700); --color-navy-deep: var(--navy-deep);
  --color-accent-orange: var(--accent-orange); --color-accent-crimson: var(--accent-crimson);
  --color-ink: var(--ink); --color-ink-2: var(--ink-2); --color-ink-3: var(--ink-3);
  --color-ok: var(--ok); --color-ok-bg: var(--ok-bg); --color-warn: var(--warn); --color-warn-bg: var(--warn-bg);
  --color-danger-bg: var(--danger-bg); --color-on-accent: var(--on-accent);
  --color-surface-50: var(--surface-50); --color-surface-100: var(--surface-100);
  --color-surface-blue-50: var(--surface-blue-50); --color-surface-blue-100: var(--surface-blue-100);
  --color-surface-dark: var(--surface-dark);
  /* tên trùng goup.css: dùng giá trị literal y hệt, KHÔNG viết var(--font-sans) (tự tham chiếu) */
  --font-sans: "Inter Tight", sans-serif; --font-display: Manrope, sans-serif; --font-ui: Roboto, sans-serif;
  /* thay thang calc(var(--radius)…) của shadcn: góc gần vuông */
  --radius-xs: 2px; --radius-sm: 3px; --radius-md: 3px; --radius-lg: 3px; --radius-xl: 3px; --radius-pill: 50px;
  --shadow-card: 0 0 7px 0 rgba(0,0,0,.15); --shadow-card-strong: 0 0 7px 0 rgba(0,0,0,.2); --shadow-float: 0 5px 15px 0 rgba(0,0,0,.15);
  --text-xs: 12px; --text-sm: 13px; --text-md: 15px; --text-lg: 18px; --text-xl: 24px; --text-2xl: 32px;
  --ease-brand: cubic-bezier(.2,.7,.2,1);
}
@theme {
  --animate-pop: pop 180ms cubic-bezier(.2,.7,.2,1);   /* tương tự slide, rise */
  @keyframes pop { from { opacity: 0; transform: translateY(6px) scale(.985) } to { opacity: 1; transform: none } }
}
```

Lớp `@layer base` giữ các quy tắc của prototype: `body` dùng `font-sans text-md leading-normal tracking-[.2px]`, `h1` dùng `font-display`, `:focus-visible` là outline 2px navy-700 lệch 2px, và khối `prefers-reduced-motion` tắt mọi animation. **Bẫy trùng tên:** `goup.css` khai báo `--font-sans`, `--radius-*`, `--shadow-*` ở `:root` không có layer, nên thắng biến cùng tên Tailwind sinh trong `@layer theme`. Vì vậy giá trị trong `@theme` phải trùng y hệt goup. sidecup dùng `"Inter Tight Variable"`; nếu làm vậy ở đây, `var(--font-sans)` sẽ rơi về font dự phòng. Font import trong `main.tsx` qua `@fontsource/*` với đúng các weight prototype tải: Inter Tight 400/500/600/700 và 400 italic, Manrope 600/700, Roboto 400/700. Cách này tự host và bỏ request tới Google Fonts. Spacing: `--sp-1…--sp-8` (4, 8, 12, 16, 24, 32, 48, 64px) tương ứng `1, 2, 3, 4, 6, 8, 12, 16` của Tailwind, nên giữ `--spacing` mặc định. Không reset `--color-*: initial`, vì shadcn dùng `bg-black/50` và `text-white`.

**Sửa variant `cva` theo bảng dưới** (giá trị lấy từ prototype):

| Component | Thay đổi trong file `ui/` |
|---|---|
| button | Base: `rounded-full uppercase font-medium text-[13px] leading-[13px] tracking-[1.3px] min-h-11 px-[35px] py-[18px] active:translate-y-px disabled:opacity-45`. Thay `focus-visible:ring-[3px]` bằng `focus-visible:outline-2 outline-offset-3 outline-navy-700`. Variant `default` (chính là navy): `bg-navy-700 text-white hover:bg-navy-900`. Thêm `gradient`: `bg-navy-700 bg-(image:--gradient-accent) text-on-accent hover:brightness-106`. `outline` (= secondary): `border border-navy-700 bg-white text-navy-700 hover:bg-surface-blue-50`. `ghost`: `px-3 text-navy-700 hover:bg-surface-blue-50`. `destructive`: `bg-destructive text-white hover:brightness-92`. Size `sm`: `min-h-9 px-4 py-2.5 text-xs tracking-[1px]`. Size `icon`: `size-11 rounded-sm` (icon-btn vuông, không phải pill) |
| badge | Base vuông, `text-[11px] leading-[14px] tracking-[.5px] px-1.5 py-0.5`. Variant theo hình thức: `solid`, `outline`, `muted`, `warn`, `subtle`. Ánh xạ `VersionStatus → variant` đặt trong `features/*/model`, không đặt tên variant theo trạng thái nghiệp vụ |
| alert | Variant `info`, `warn`, `danger`, `ok` theo `.alert-*`; vuông, `border text-sm px-4 py-3` |
| input, textarea, select trigger | `h-10 rounded-xs border-input bg-white text-[16px]` (16px chống zoom iOS). Focus: `border-navy-700 ring-3 ring-surface-blue-100`. `aria-invalid:border-destructive`. Textarea: mono 14px, `min-h-35` |
| dialog | Content `rounded-none shadow-float w-[min(560px,calc(100vw-32px))]`. Overlay `bg-navy-900/45`. Đổi `zoom-in-95` sang `animate-pop` |
| sheet | `side="right"`, `w-[min(520px,100vw)]`, overlay navy/45, `animate-slide`; full width khi ≤960px |
| progress | Track `h-1.5 bg-surface-blue-100 rounded-none`, indicator `bg-navy-700 transition-[width] duration-300`. Size `lg` cao `h-2.5` |
| dropdown-menu, select content, tooltip | `rounded-sm shadow-float`; tooltip nền `surface-dark` |
| table | Giữ wrapper `overflow-x-auto` sẵn có; header theo `.table` của prototype |
| sonner | Xem mục 5. Bỏ `next-themes` khỏi wrapper vì app Vite không dùng |

Chỉ một nút `gradient` trên mỗi trang (design.md). Ràng buộc này thuộc review, không ép bằng code.

## 3. Cấu trúc thư mục

**Khuyến nghị: theo Bulletproof React, thêm segment `model` mượn từ FSD.** Bulletproof dùng `features/*` phẳng và luồng một chiều shared → features → app [S17]. FSD có 6 tầng (app, pages, widgets, features, entities, shared) và segment `ui/api/model/lib` [S18]. Với khoảng 8 feature, FSD đầy đủ là quá nặng. Chỉ segment `model` là đáng mượn.

```text
src/
  app/        router.tsx, providers.tsx, layouts/{admin,teach,learn,auth}, middleware/
  features/<feature>/   # auth, users, courses, stages, versions, classes, enrollments, lessons, progress
    api/        zod DTO schema, mapper DTO→model, queryOptions + key factory, mutation fn
    model/      TS thuần, không React/fetch: Percent, VersionStatus, canTransition…
    hooks/      useXxxQuery / useXxxMutation bọc api/
    components/ UI riêng của feature
    pages/      mỏng: đọc params, gọi hooks, ghép components
    index.ts    public API duy nhất
  shared/{ui,lib,hooks}   # khớp aliases components.json của sidecup: @/shared/ui, @/shared/lib
  styles/  test/
```

```ts
// features/versions/model/version-status.ts
export type VersionStatus = "draft" | "published" | "archived";
const next: Record<VersionStatus, readonly VersionStatus[]> = { draft: ["published"], published: ["archived"], archived: [] };
export const canTransition = (from: VersionStatus, to: VersionStatus) => next[from].includes(to);
// features/progress/model/percent.ts
export type Percent = number & { readonly __brand: "Percent" };
export const percentOf = (done: number, total: number): Percent =>
  (total === 0 ? 0 : Math.round((done * 100) / total)) as Percent;
```

Luật: dùng ESLint `no-restricted-imports` cấm `@/features/*/*`, chỉ import qua `index.ts`, và `model/` không import ra ngoài. TanStack Query là tầng application. Zod parse response tại biên `api/`. `model` ở client chỉ dùng để bật/tắt UI, backend vẫn là nơi quyết định.

## 4. Markdown HTML và video

**Markdown: một component `MarkdownContent` duy nhất, sanitize lại bằng DOMPurify rồi mới `dangerouslySetInnerHTML`.** React chỉ cho phép dữ liệu "trusted and sanitized" [S19]. Lọc lại ở client là phòng thủ nhiều lớp, phòng khi server có lỗi hoặc dữ liệu cũ, với chi phí nhỏ. Dùng `DOMPurify.sanitize(html, { USE_PROFILES: { html: true } })` và hook `afterSanitizeAttributes` để thêm `rel="noopener noreferrer"` cho link `target=_blank` [S20]. Bọc trong `useMemo` theo `html`. Không sửa HTML sau khi sanitize, vì DOMPurify cảnh báo việc này làm mất tác dụng lọc [S20]. Backend nên gửi thêm CSP `script-src 'self'`.

**Video: `<video controls preload="metadata" playsInline controlsList="nodownload">`, URL lấy qua query.** `metadata` chỉ tải độ dài và thông số [S21]. Không đặt thuộc tính `crossorigin`, nếu không storage phải bật CORS cho GET.

1. API trả `{ url, expiresAt }`. Query key là `['lesson-video', lessonId]`, `staleTime` đến `expiresAt − 60s`, `gcTime: 0`.
2. Mỗi lần tua là một request Range mới dùng cùng URL. URL hết hạn thì request đó lỗi (báo cáo backend, mục 6).
3. Sự kiện `error` không lộ HTTP status [S7]. Khi có lỗi: lưu `currentTime` và trạng thái đang phát, `refetch()` URL, rồi ở `loadedmetadata` đặt lại `currentTime` và gọi `play()` nếu cần. Thử tối đa 2 lần, sau đó hiện alert "Không phát được video. Tải lại trang để thử lại."
4. File MP4 phải có moov atom ở đầu (faststart) để `preload="metadata"` và tua hoạt động mà không phải tải hết. Đây là việc của pipeline upload; điểm này dựa trên kinh nghiệm, chưa trích nguồn.

## 5. Mẫu truy cập (a11y): prototype và bản React

| Prototype | Bản React | Ghi chú |
|---|---|---|
| `<dialog>` + `aria-labelledby` | shadcn `Dialog` (Radix) | Luôn có `DialogTitle`; Radix tự bẫy focus, đóng bằng Esc và trả focus |
| Drawer học viên | `Sheet side="right"` | Bắt buộc có `SheetTitle` |
| `#toasts` `aria-live=polite`, tự tắt sau 4s, tối đa 3 | `<Toaster position="bottom-right" duration={4000} visibleToasts={3} closeButton containerAriaLabel="Thông báo" offset={24} mobileOffset={16} toastOptions={{ unstyled: true, classNames: {...} }} />` | Sonner tự render `section aria-live="polite" aria-relevant="additions text"` và tôn trọng `prefers-reduced-motion` [S22]. Mặc định `visibleToasts=3`, `closeButton=false` [S23]. `classNames` lấy `bg-surface-dark text-white shadow-float border-l-3`, viền crimson cho lỗi |
| Tabs là `<nav aria-label>` gồm link | `<nav aria-label="…">` + `NavLink end` | `NavLink` tự đặt `aria-current="page"` [S24]. Không dùng Radix Tabs, vì tab phải có URL riêng |
| Skip link `#main` | `<a href="#main" className="sr-only focus:not-sr-only …">Bỏ qua điều hướng</a>`, `<main id="main" tabIndex={-1}>` | |
| `#announcer` nhận tiêu đề trang | `RouteAnnouncer`: `useMatches()` đọc `handle.title`, ghi vào `<p className="sr-only" aria-live="polite">`, đặt `document.title`, chuyển focus tới `h1` | Thử nghiệm người dùng cho thấy focus vào heading là trải nghiệm tốt nhất [S25] |
| `prefers-reduced-motion` tắt hết | Giữ quy tắc toàn cục trong `@layer base` | Một chỗ duy nhất, phủ cả class của tw-animate-css |
| Vùng chạm 44px | `min-h-11` / `size-11` trong cva | |

## 6. Routing theo vai trò (React Router v8, data mode)

**Khuyến nghị: dùng `createBrowserRouter` với route middleware.** Ở v8 middleware luôn bật. Middleware cha chạy trước loader con, và chạy ở mọi lần điều hướng [S26]. Loader dùng `queryClient.ensureQueryData`, component dùng `useQuery` [S27]. Import từ `react-router`, riêng `RouterProvider` từ `react-router/dom` [S11].

```ts
const authMiddleware: MiddlewareFunction = async ({ request, context }) => {
  const me = await queryClient.ensureQueryData(meQuery); // queryFn trả null khi 401
  const url = new URL(request.url);
  if (!me) throw redirect(`/login?next=${encodeURIComponent(url.pathname + url.search)}`);
  if (me.mustChangePassword && url.pathname !== "/first-login") throw redirect("/first-login");
  context.set(userContext, me);
};
const requireRole = (role: Role): MiddlewareFunction => ({ context }) => {
  if (context.get(userContext)?.role !== role) throw redirect(homeOf(context.get(userContext)!));
};
createBrowserRouter([
  { path: "/login", Component: LoginPage },
  { middleware: [authMiddleware], Component: AppShell, children: [
    { path: "first-login", Component: FirstLoginPage },
    { path: "admin", middleware: [requireRole("admin")], Component: AdminLayout, children: [/*…*/] },
    { path: "teach", middleware: [requireRole("teacher")], Component: TeachLayout, children: [/*…*/] },
    { path: "learn", middleware: [requireRole("student")], Component: LearnLayout, children: [/*…*/] },
    { index: true, loader: ({ context }) => redirect(homeOf(context.get(userContext)!)) },
  ]},
  { path: "*", Component: NotFoundPage },
]);
```

**Lớp fetch dùng chung (`shared/lib/http.ts`):** dùng `fetch('/api' + path, { credentials: 'same-origin' })`, parse lỗi thành `ApiError { status, code }`, và không tự điều hướng. Xử lý tập trung đặt trong `new QueryClient({ queryCache: new QueryCache({ onError }), mutationCache: new MutationCache({ onError }) })`:

- **401:** `setQueryData(meKey, null)` rồi `router.navigate('/login?next=…')`. Bỏ qua khi đang ở `/login` để tránh vòng lặp.
- **403 với `code === 'PASSWORD_CHANGE_REQUIRED'`:** spec quy định mã này, xem FR đổi mật khẩu. Điều hướng tới `/first-login`.
- **Retry:** không retry lỗi 4xx (`retry: (n, e) => !(e instanceof ApiError && e.status < 500) && n < 2`).
- **Đổi mật khẩu, đăng nhập, đăng xuất:** `invalidateQueries(meKey)` sau mỗi thao tác.

Backend dùng session cookie cùng origin và chặn CSRF bằng `Sec-Fetch-Site`, nên client không cần token. Lúc dev, dùng proxy `/api` của Vite để giữ cùng origin.

## 7. Rủi ro khi áp dụng

| Hạng mục | Độ chín | Rủi ro | Giảm thiểu |
|---|---|---|---|
| React Router 8 | Ra 4 tháng, chu kỳ major hằng năm | Thấp | Data mode giữ nguyên API của v7; chỉ đổi import |
| MSW 3 | Ra 1 tuần | Trung bình, nhưng chỉ dùng khi dev/test | Ghim `3.0.2`; nếu vướng thì lùi về 2.15.0 |
| shadcn Base UI làm mặc định | Radix "không bị deprecate" [S5] | Thấp | Luôn `init -b radix` |
| TypeScript 7 | Ổn định nhưng hệ lint chưa theo kịp | Cao nếu dùng | Dùng 6.0.x |

## Giới hạn

- Chưa scaffold thật để kiểm `shadcn init -b radix` với Vite 8 + TS 6. Các class trong bảng cva cần kiểm bằng mắt khi dựng.
- Chưa thử hành vi `<video>` khi URL hết hạn giữa chừng trên Safari và Chrome. Mã lỗi thực tế có thể khác nhau giữa trình duyệt.
- Chưa kiểm cookie `__Host-` (bắt buộc `Secure`) trên `http://localhost` ở Safari khi dev.

## Câu hỏi chưa giải quyết

1. **Viền input:** prototype dùng `#c4c4c4` (1.74:1, không đạt WCAG 1.4.11). sidecup nâng lên `#8a8a8a`. Chọn giống hệt prototype hay đạt AA?
2. **Nền và chữ thân:** prototype dùng nền `surface-50` và chữ `ink-2`, còn sidecup dùng nền trắng và chữ navy-900. Báo cáo này theo prototype.
3. **Quyền chéo vai trò:** admin có được vào `/teach/*` không? Mỗi user một hay nhiều vai trò? Câu trả lời quyết định `requireRole`.
4. **Ảnh trong markdown:** URL ký sẵn bên trong HTML sẽ hết hạn khi HTML được cache. Đề xuất backend đổi `src` ảnh thành `/api/media/{id}`, endpoint này 302 tới URL ký mới.
5. **Video:** API có trả `expiresAt` không, và hạn bao lâu? Pipeline upload có chạy faststart không?
6. **Dark mode:** design không nhắc tới, nên báo cáo bỏ `.dark`.

## Nguồn

- [S1] npm registry, `npm view <pkg> version|time|dist-tags|peerDependencies`, truy vấn 2026-10-05
- [S2] https://www.npmjs.com/package/typescript-eslint (peerDependencies)
- [S3] https://github.com/typescript-eslint/typescript-eslint/issues/10940
- [S4] https://github.com/vitejs/vite/blob/main/packages/create-vite/template-react-ts/package.json
- [S5] https://ui.shadcn.com/docs/changelog/2026-07-base-ui-default
- [S6] https://ui.shadcn.com/docs/changelog/2026-02-radix-ui
- [S7] https://developer.mozilla.org/en-US/docs/Web/API/MediaError/code
- [S8] https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react/README.md
- [S9] https://ui.shadcn.com/docs/changelog/2026-03-cli-v4
- [S10] https://ui.shadcn.com/docs/changelog/2026-09-cn
- [S11] https://github.com/remix-run/react-router/blob/main/CHANGELOG.md (v8.0.0)
- [S12] https://github.com/facebook/react/blob/main/packages/eslint-plugin-react-hooks/README.md
- [S13] https://github.com/mswjs/msw/releases/tag/v3.0.0
- [S14] https://api.fontsource.org/v1/fonts/inter-tight (tương tự manrope, roboto)
- [S15] https://tailwindcss.com/docs/theme
- [S16] https://ui.shadcn.com/docs/theming
- [S17] https://github.com/alan2207/bulletproof-react/blob/master/docs/project-structure.md
- [S18] https://feature-sliced.design/docs/get-started/overview
- [S19] https://react.dev/reference/react-dom/components/common#dangerously-setting-the-inner-html
- [S20] https://github.com/cure53/DOMPurify
- [S21] https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/video
- [S22] https://github.com/emilkowalski/sonner/blob/main/src/index.tsx, src/styles.css
- [S23] https://sonner.emilkowal.ski/toaster
- [S24] https://reactrouter.com/api/components/NavLink
- [S25] https://www.gatsbyjs.com/blog/2019-07-11-user-testing-accessible-client-routing/
- [S26] https://reactrouter.com/how-to/middleware
- [S27] https://tkdodo.eu/blog/react-query-meets-react-router
- [S28] https://tailwindcss.com/docs/background-image
