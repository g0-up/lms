// Functional and accessibility checks for the prototype, run in headless Chrome over the DevTools protocol.
// Usage: node prototype/check-flows.mjs [--out <dir>]   (Node 18+, Google Chrome installed, no dependencies)
// Covers spec scenario 7.3 (clone, edit, publish, apply, class version), invites, first login, report filters,
// lesson ticks, then 320px reflow, reduced motion, keyboard and ARIA probes. Exits 1 when any check fails.
import { spawn } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const URL0 = 'file://' + join(dirname(fileURLToPath(import.meta.url)), 'index.html');
const outIdx = process.argv.indexOf('--out'); const OUT = outIdx > -1 ? process.argv[outIdx + 1] : null; if (OUT) mkdirSync(OUT, { recursive: true });
const profile = mkdtempSync(join(tmpdir(), 'lms-walk-'));
const proc = spawn(CHROME, ['--headless=new', '--remote-debugging-port=0', `--user-data-dir=${profile}`, '--no-first-run', '--disable-gpu', '--hide-scrollbars', '--window-size=1440,900', 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] });
const wsUrl = await new Promise((ok, fail) => { let b = ''; proc.stderr.on('data', (d) => { b += d; const m = b.match(/DevTools listening on (ws:\/\/\S+)/); if (m) ok(m[1]); }); setTimeout(() => fail(new Error('no devtools')), 20000); });
const ws = new WebSocket(wsUrl);
let id = 1; const pend = new Map(); const listeners = [];
ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && pend.has(m.id)) { const p = pend.get(m.id); pend.delete(m.id); m.error ? p.fail(new Error(m.error.message)) : p.ok(m.result); } else if (m.method) listeners.forEach((l) => l(m)); };
await new Promise((ok) => (ws.onopen = ok));
const send = (method, params = {}, sessionId) => new Promise((ok, fail) => { const i = id++; pend.set(i, { ok, fail }); ws.send(JSON.stringify({ id: i, method, params, sessionId })); });
const { targetId } = await send('Target.createTarget', { url: 'about:blank' });
const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
await send('Runtime.enable', {}, sessionId); await send('Page.enable', {}, sessionId);
const consoleIssues = [];
listeners.push((m) => {
  if (m.sessionId !== sessionId) return;
  if (m.method === 'Runtime.exceptionThrown') consoleIssues.push('exception: ' + (m.params.exceptionDetails.exception?.description || m.params.exceptionDetails.text));
  if (m.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(m.params.type)) consoleIssues.push(m.params.type + ': ' + m.params.args.map((a) => a.value ?? a.description).join(' '));
});
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function ev(expr) { const r = await send('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true }, sessionId); if (r.exceptionDetails) throw new Error('page error: ' + (r.exceptionDetails.exception?.description || r.exceptionDetails.text)); return r.result.value; }
async function go(hash) { await send('Page.navigate', { url: URL0 + hash }, sessionId); await sleep(500); }
async function shot(name) { if (!OUT) return; const { data } = await send('Page.captureScreenshot', { format: 'png' }, sessionId); writeFileSync(join(OUT, name + '.png'), Buffer.from(data, 'base64')); }
const text = () => ev('document.body.innerText');
const click = async (sel) => { const okc = await ev(`(()=>{const b=document.querySelector(${JSON.stringify(sel)}); if(!b) return false; b.click(); return true;})()`); if (!okc) throw new Error('no element ' + sel); await sleep(250); };
const fill = (values) => ev(`(()=>{const f=document.querySelector('#modal form'); for (const [k,v] of Object.entries(${JSON.stringify(values)})) { const el=f.elements[k]; if(!el) throw new Error('no field '+k); if(el.type==='checkbox') el.checked=!!v; else { el.value=v; el.dispatchEvent(new Event('change',{bubbles:true})); } } return true;})()`);
const submitModal = async () => { await ev(`document.querySelector('#modal form').requestSubmit()`); await sleep(300); };
const state = () => ev(`JSON.parse(localStorage.getItem('goup-lms-prototype-v1'))`);
const results = [];
const check = (name, cond, detail = '') => { results.push({ name, pass: !!cond, detail }); console.log((cond ? 'PASS ' : 'FAIL ') + name + (detail ? ' — ' + detail : '')); };

try {
  // fresh state
  await go('#/login'); await ev(`localStorage.clear()`); await go('#/admin/stages/st-db?as=u-admin');
  check('stage page shows v1 published', (await text()).includes('Học liệu của v1'));

  // 7.3 step 1: clone v1 -> v2 draft
  await click('[data-action="clone-sv"][data-id="sv-db-1"]'); await sleep(400);
  let hash = await ev('location.hash');
  check('clone creates draft and navigates', /\?v=sv-/.test(hash) && (await text()).includes('Học liệu của v2'), hash);
  let st = await state(); const draft = st.stageVersions.find((v) => v.stageId === 'st-db' && v.status === 'draft');
  check('draft v2 copies 3 lessons with same keys', draft && draft.no === 2 && draft.lessons.length === 3 && draft.lessons.map((l) => l.key).join() === st.stageVersions.find((v) => v.id === 'sv-db-1').lessons.map((l) => l.key).join());
  check('second clone is blocked (one draft max)', !(await ev(`!!document.querySelector('[data-action="clone-sv"]')`)) );

  // step 2: edit draft (add lesson)
  await click(`[data-action="new-lesson"][data-sv="${draft.id}"]`);
  await fill({ title: 'JOIN và subquery', type: 'video', video: 'db-04-join.mp4', duration: '12:40', required: true });
  await submitModal();
  st = await state(); const d2 = st.stageVersions.find((v) => v.id === draft.id);
  check('lesson added to draft', d2.lessons.length === 4 && d2.lessons[3].title === 'JOIN và subquery', JSON.stringify(d2.lessons[3]));
  check('published v1 untouched', st.stageVersions.find((v) => v.id === 'sv-db-1').lessons.length === 3);
  await shot('01-draft-v2');

  // step 3: publish v2
  await click(`[data-action="publish-sv"][data-id="${draft.id}"]`); await submitModal(); await sleep(300);
  st = await state(); const v2 = st.stageVersions.find((v) => v.id === draft.id);
  check('v2 published', v2.status === 'published' && !!v2.publishedAt);
  let t = await text();
  check('FR-18 warning lists Lập trình cơ bản v1 on stage page', t.includes('Khóa học đang dùng phiên bản cũ của chặng này') && t.includes('Lập trình cơ bản v1'));
  await shot('02-v2-published-warning');
  await go('#/admin'); t = await text();
  check('dashboard shows outdated warning', /phiên bản cũ|Database v2/.test(t), t.match(/.*(phiên bản cũ|Database v2).*/)?.[0]);
  await shot('03-dashboard-warning');

  // step 4: apply v2 to course (FR-17)
  await go(`#/admin/stages/st-db?v=${v2.id}`);
  await click(`[data-action="apply-sv"][data-course="co-basic"]`); await sleep(200);
  const modalText = await ev(`document.querySelector('#modal').innerText`);
  check('apply modal explains atomic transaction', /Nhân bản Lập trình cơ bản v1 thành v2[\s\S]*Phát hành v2/.test(modalText), modalText.replace(/\s+/g,' ').slice(0,300));
  await submitModal(); await sleep(300);
  st = await state();
  const cvs = st.courseVersions.filter((v) => v.courseId === 'co-basic');
  const cv2 = cvs.find((v) => v.no === 2);
  check('course v2 created and published', cv2 && cv2.status === 'published' && cvs.length === 2);
  check('course v2 uses Database v2, other stages unchanged', cv2 && cv2.stages.includes(v2.id) && !cv2.stages.includes('sv-db-1') && cv2.stages.length === cvs[0].stages.length);
  check('course v1 still published and unchanged', cvs[0].status === 'published' && cvs[0].stages.includes('sv-db-1'));
  check('basic01/02/03 remain on course v1', st.classes.every((c) => c.courseVersionId === 'cv-basic-1'), st.classes.map((c) => c.code + ':' + c.courseVersionId).join(' '));
  t = await text();
  check('warning disappears after apply', !t.includes('Khóa học đang dùng phiên bản cũ') && t.includes('Mọi khóa học đã phát hành đều dùng phiên bản mới nhất'));
  await shot('04-after-apply');
  await go('#/admin'); check('dashboard warning gone', !/phiên bản cũ/.test(await text()));
  await go('#/admin/courses/co-basic'); t = await text(); check('course page lists v1 and v2', t.includes('v2') && t.includes('v1')); await shot('05-course-v2');

  // student flow: open + tick a lesson
  await go('#/learn/classes/cl-basic01?as=u-an'); t = await text();
  const pctBefore = Number((await ev(`document.querySelector('.progress-lg .pct, .progress .pct')?.textContent`) || '').replace('%', ''));
  const lessonHref = await ev(`[...document.querySelectorAll('.lesson-row:not(.is-done)')].filter(r=>!/Không bắt buộc|tùy chọn/i.test(r.innerText)).map(r=>r.querySelector('a.lesson-link').getAttribute('href'))[0]`);
  check('student roadmap renders with an unfinished lesson', !!lessonHref && !Number.isNaN(pctBefore), `course ${pctBefore}% first open lesson ${lessonHref}`);
  await go(lessonHref); await sleep(300);
  st = await state(); const lessonId = lessonHref.split('/lessons/')[1]; const mem = st.members.find((m) => m.classId === 'cl-basic01' && m.userId === 'u-an');
  const prOpen = st.progress[`${mem.id}:${lessonId}`] || st.progress.find?.((p) => p.memberId === mem.id && p.lessonId === lessonId);
  check('opening lesson records firstOpenedAt', !!(prOpen && prOpen.firstOpenedAt) , JSON.stringify(prOpen));
  await shot('06-lesson-open');
  await click('[data-action="complete"][data-done="1"]'); await sleep(300);
  st = await state(); const prDone = st.progress[`${mem.id}:${lessonId}`] || st.progress.find?.((p) => p.memberId === mem.id && p.lessonId === lessonId);
  check('tick records completedAt', !!(prDone && prDone.completedAt), JSON.stringify(prDone));
  t = await text(); check('lesson page shows done state', /bỏ tích/i.test(t) && /đã tích/i.test(t), t.split('\n').filter(l=>/tích|học xong/i.test(l)).join(' | '));
  await go('#/learn/classes/cl-basic01');
  const pctAfter = Number((await ev(`document.querySelector('.progress-lg .pct, .progress .pct')?.textContent`) || '').replace('%', ''));
  check('course % increases after tick', pctAfter > pctBefore, `${pctBefore}% -> ${pctAfter}%`);
  await shot('07-roadmap-after-tick');
  // untick
  await go(lessonHref); await click('[data-action="complete"][data-done="0"]'); await go('#/learn/classes/cl-basic01');
  const pctBack = Number((await ev(`document.querySelector('.progress-lg .pct, .progress .pct')?.textContent`) || '').replace('%', ''));
  check('untick restores %', pctBack === pctBefore, `${pctBack}%`);

  // report filters / sort (admin)
  await go('#/admin/classes/cl-basic01?tab=report&as=u-admin'); t = await text();
  const rowsAll = await ev(`document.querySelectorAll('table tbody tr').length`);
  await go('#/admin/classes/cl-basic01?tab=report&below=30'); const rowsBelow = await ev(`document.querySelectorAll('table tbody tr').length`);
  const pctsBelow = await ev(`[...document.querySelectorAll('table tbody tr')].map(r=>r.innerText)`);
  check('report filter below 30% narrows rows', rowsBelow < rowsAll && rowsBelow > 0, `${rowsAll} -> ${rowsBelow}`);
  await go('#/admin/classes/cl-basic01?tab=report&sort=pct-desc'); const firstDesc = await ev(`document.querySelector('table tbody tr')?.innerText`);
  await go('#/admin/classes/cl-basic01?tab=report&sort=pct'); const firstAsc = await ev(`document.querySelector('table tbody tr')?.innerText`);
  check('report sort changes order', firstDesc && firstAsc && firstDesc !== firstAsc, `desc first: ${firstDesc?.split('\n')[0]} | asc first: ${firstAsc?.split('\n')[0]}`);
  await go('#/admin/classes/cl-basic01?tab=report&notlogged=1'); const rowsNL = await ev(`[...document.querySelectorAll('table tbody tr')].map(r=>r.innerText.split('\\n')[0])`);
  check('notlogged filter shows only never-logged students', rowsNL.length >= 1 && rowsNL.join().includes('Dũng'), rowsNL.join(', '));
  await go('#/admin/classes/cl-basic01?tab=report&inactive=10'); const rowsIn = await ev(`[...document.querySelectorAll('table tbody tr')].map(r=>r.innerText.split('\\n')[0])`);
  check('inactive>10 days filter', rowsIn.length >= 1 && rowsIn.length < rowsAll, rowsIn.join(', '));
  // form submit path for the filter (UI, not just hash)
  await go('#/admin/classes/cl-basic01?tab=report');
  await ev(`(()=>{const f=document.querySelector('form[data-form=report-filter]'); f.elements.below.value='50'; f.requestSubmit(); return true;})()`); await sleep(300);
  check('filter form writes query to hash', /below=50/.test(await ev('location.hash')), await ev('location.hash'));
  await shot('08-report-filtered');
  // teacher view
  await go('#/teach/classes/cl-basic01?as=u-gv'); t = await text();
  check('teacher sees class report', t.includes('basic01') && t.includes('%'));
  await go('#/admin?as=u-gv'); check('teacher cannot open admin route', !(await text()).includes('Tổng quan hệ thống') || /không có quyền|Không tìm thấy|403/i.test(await text()), (await ev('location.hash')));

  // invite flow
  await go('#/admin/classes/cl-basic01?as=u-admin');
  await click('[data-action="invite"]'); await fill({ email: '  AN.NGUYEN@gmail.com ', name: '' }); await submitModal(); await sleep(200);
  const dup = await ev(`document.querySelector('#modal [data-modal-error] .alert-body')?.textContent || document.querySelector('#toasts')?.innerText`);
  check('inviting existing member is refused', /đã có trong lớp/i.test(dup || ''), dup);
  await click('[data-action="close-modal"]');
  await click('[data-action="invite"]'); await fill({ email: 'moi.hocvien@gmail.com', name: 'Học Viên Mới' }); await submitModal(); await sleep(100);
  st = await state(); const nu = st.users.find((u) => u.email === 'moi.hocvien@gmail.com');
  check('new email creates invited account with temp password', nu && nu.mustChangePassword && st.members.some((m) => m.userId === nu.id && m.classId === 'cl-basic01'), JSON.stringify(nu));
  await sleep(1800); st = await state(); const inv = st.invitations.filter((i) => i.userId === nu.id).pop();
  check('invitation email transitions queued -> sent', inv && inv.status === 'sent', JSON.stringify(inv));
  await click('[data-action="invite"]'); await fill({ email: 'bounce.me@gmail.com', name: 'Thư Hỏng' }); await submitModal(); await sleep(2000);
  st = await state(); const bu = st.users.find((u) => u.email === 'bounce.me@gmail.com'); const binv = st.invitations.filter((i) => i.userId === bu?.id).pop();
  check('bounce address ends as failed', binv && binv.status === 'failed', JSON.stringify(binv));
  await shot('09-students-after-invite');

  // first login flow (u-minh: must change password)
  await click('[data-action="logout"]'); await sleep(300);
  await ev(`(()=>{const f=document.querySelector('form[data-form=login]'); f.email.value='minh.bui@gmail.com'; f.password.value='tam-thoi-123'; f.requestSubmit(); return true})()`); await sleep(400);
  check('temp-password login forces first-login page', (await ev('location.hash')).includes('first-login'), await ev('location.hash'));
  await ev(`(()=>{const f=document.querySelector('form[data-form=first-login]'); f.password.value='short'; f.confirm.value='short'; f.requestSubmit(); return true})()`); await sleep(200);
  check('short password rejected', /8 ký tự/.test(await text()));
  await ev(`(()=>{const f=document.querySelector('form[data-form=first-login]'); f.password.value='mat-khau-moi-2026'; f.confirm.value='mat-khau-moi-2026'; f.requestSubmit(); return true})()`); await sleep(400);
  check('valid new password lands on learn home', (await ev('location.hash')).startsWith('#/learn'), await ev('location.hash'));
  await shot('10-first-login-done');
  // expired temp password + disabled account
  await click('[data-action="logout"]'); await sleep(300);
  await ev(`(()=>{const f=document.querySelector('form[data-form=login]'); f.email.value='dung.pham@gmail.com'; f.password.value='x'; f.requestSubmit(); return true})()`); await sleep(300);
  check('expired temp password login refused', /hết hạn/i.test(await text()));
  await ev(`(()=>{const f=document.querySelector('form[data-form=login]'); f.email.value='thao.vo@gmail.com'; f.password.value='x'; f.requestSubmit(); return true})()`); await sleep(300);
  check('disabled account login refused', /vô hiệu/i.test(await text()));

  // reset data
  await go('#/admin/stages/st-db?as=u-admin'); await click('[data-action="reset-data"]'); await submitModal(); await sleep(400);
  st = await state(); check('reset restores seed', st.stageVersions.filter((v) => v.stageId === 'st-db').length === 1 && st.courseVersions.length === 1);

  // --- accessibility, reflow and keyboard probes
  const ROUTES = ['#/login', '#/admin?as=u-admin', '#/admin/stages/st-db?as=u-admin', '#/admin/courses/co-basic?as=u-admin', '#/admin/classes/cl-basic01?as=u-admin', '#/admin/classes/cl-basic01?tab=report&as=u-admin', '#/teach?as=u-gv', '#/learn?as=u-an', '#/learn/classes/cl-basic01?as=u-an', '#/learn/classes/cl-basic01/lessons/db-intro?as=u-an'];
  await send('Emulation.setDeviceMetricsOverride', { width: 320, height: 812, deviceScaleFactor: 1, mobile: true }, sessionId);
  for (const h of ROUTES) {
    await go(h);
    const r = await ev(`({ inner: window.innerWidth, scroll: document.documentElement.scrollWidth })`);
    check(`320px reflow ${h.split('?')[0]}`, r.inner === 320 && r.scroll <= 320, `innerWidth ${r.inner}, scrollWidth ${r.scroll}`);
  }
  await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] }, sessionId);
  await go('#/admin?as=u-admin');
  check('reduced motion removes transitions', (await ev(`getComputedStyle(document.querySelector('.un-btn')).transitionDuration`)) === '0s');
  await send('Emulation.setEmulatedMedia', { features: [] }, sessionId);
  await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false }, sessionId);
  await go('#/admin?as=u-admin');
  check('route change is announced once', (await ev(`document.querySelector('#announcer').textContent`)) === 'Tổng quan' && !(await ev(`document.querySelector('#app').hasAttribute('aria-live')`)));
  check('dashboard has the guided scenario with deep links', (await ev(`(()=>{const d=document.querySelector('details.guide'); return d && d.querySelectorAll('ol.guide-steps li').length===5 && d.querySelectorAll('a[href^="#/admin/"]').length>=3})()`)) === true);
  await ev(`document.querySelector('details.guide > summary').focus()`);
  await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', text: '\r', windowsVirtualKeyCode: 13 }, sessionId); await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 }, sessionId); await sleep(150);
  check('guide opens with Enter on its summary', (await ev(`document.querySelector('details.guide').open`)) === true);
  await go('#/admin/classes/cl-basic01?tab=report&as=u-admin');
  check('class tabs are a labelled nav, not an ARIA tablist', (await ev(`!document.querySelector('[role=tablist]') && !!document.querySelector('nav.tabs[aria-label]') && document.querySelector('nav.tabs a[aria-current=page]').textContent.includes('Tiến độ')`)) === true);
  await go('#/admin/stages/st-db?as=u-admin');
  await click('[data-action="reset-data"]');
  check('modal has a resolvable accessible name', (await ev(`(()=>{const d=document.querySelector('#modal'); const h=document.getElementById(d.getAttribute('aria-labelledby')); return !!h && d.open && h.textContent.length>0})()`)) === true);
  await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 }, sessionId); await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 }, sessionId); await sleep(200);
  check('Escape closes the modal', !(await ev(`document.querySelector('#modal').open`)));
  await click('[data-action="clone-sv"]'); await sleep(300);
  check('toast has a focusable close button', (await ev(`(()=>{const b=document.querySelector('#toasts .toast button[aria-label="Đóng thông báo"]'); if(!b) return false; b.focus(); return document.activeElement===b})()`)) === true);
  await click('#toasts .toast button[aria-label="Đóng thông báo"]');
  check('close button removes the toast', (await ev(`document.querySelectorAll('#toasts .toast').length`)) === 0);
  await go('#/admin/stages/st-db?as=u-admin'); await click('[data-action="reset-data"]'); await submitModal(); await sleep(300);
  st = await state(); check('seed restored after probes', st.stageVersions.filter((v) => v.stageId === 'st-db').length === 1);
  await go('#/teach?as=u-gv');
  check('draft class shows no inactivity warning', (await ev(`[...document.querySelectorAll('.grid-3 > a')].map(a=>a.textContent.includes('basic03')+':'+a.textContent.includes('lâu không hoạt động')).join(',')`)) === 'false:true,true:false');
} catch (e) { console.log('WALK ERROR', e.message); results.push({ name: 'script error', pass: false, detail: e.message }); }

console.log('\nconsole issues:', consoleIssues.length ? consoleIssues : 'none');
console.log(`\n${results.filter((r) => r.pass).length}/${results.length} checks passed`);
ws.close(); proc.kill('SIGTERM'); try { rmSync(profile, { recursive: true, force: true }); } catch {}
process.exitCode = results.every((r) => r.pass) && !consoleIssues.length ? 0 : 1;
