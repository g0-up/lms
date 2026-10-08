# Web auth live verification

Date: 2026-10-05. Scope: the Phase 10 checks that waited for the real auth API (Phase 04).

## Result

All checks pass against the real API. No web or API defects were found.

## Setup

- API `lms serve` on :8080 and `lms worker`, both against the dev database (compose project `lms`, Postgres on host port 5433).
- Web `vite` dev server on :5173 with `VITE_API_PROXY=http://localhost:8080`.
- Mailpit on :8025 to read the reset emails.
- Headless Chromium driven by Playwright. Scripts were kept outside the repo, and the seed password was read from the environment without being printed.

## Checks

| Check | Result |
|---|---|
| Admin `quan.tran@goup.vn` logs in and lands on `/admin` | Pass |
| Invited `minh.bui@gmail.com` lands on `/first-login`, changes password, reaches `/learn` | Pass |
| Forgot password: mail arrives, link uses `#token=`, address bar shows only `/reset-password`, new password logs in | Pass |
| Logout goes to `/login`; `/auth/me` returns 401 afterwards | Pass |
| Logged-out `/admin/stages` goes to `/login?next=%2Fadmin%2Fstages` and returns there after login | Pass |
| Teacher `huong.le@goup.vn` typing `/admin` lands on `/teach` | Pass |
| All 9 mutations (login, logout, change-password, forgot, reset) carry `X-Requested-With: fetch` | Pass |
| axe on `/login`, `/forgot`, `/first-login`: no violations once the page has rendered | Pass |
| axe on the admin shell: no serious or critical violations | Pass, with one note below |
| Full profile (nginx web on :8081, images rebuilt from current sources): reset link opened through nginx, `docker compose logs web \| grep -c token` printed 0 | Pass |

## Notes

- **Admin shell heading.** axe reports a moderate `page-has-heading-one` because the admin index page is still a stub. It goes away once the admin content pages land.
- **Dev accounts changed by the run:** `minh.bui` and `linh.do` now have random passwords, and `an.nguyen` has an unused reset token. A dev `lms seed --reset --upload-sample` restores them.
- **Earlier draft of this report.** A first pass by a test agent marked several checks as passed from code review alone, without logging in. This report replaces that draft with the live results above.
