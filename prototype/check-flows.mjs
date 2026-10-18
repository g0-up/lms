// Functional and accessibility checks for the prototype, run in headless Chrome over the DevTools protocol.
// Usage: node prototype/check-flows.mjs [--out <dir>]   (Node 18+, Google Chrome installed, no dependencies)
// Covers spec scenario 7.3 (clone, edit, publish, apply, class version), invites, first login, report filters,
// bài học grouping (editor, clone, roadmap, prev/next),
// lesson ticks, one-question-at-a-time quiz taking, quiz attempts, teacher quiz review and confirmation, admin-only
// quiz authoring, homework submission and grading, stage summaries, deadlines, extra attempts, daily email jobs and publish blockers, then 320px reflow, reduced motion, keyboard and ARIA probes. Exits 1 when any check fails.
import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

const CHROME = process.env.CHROME || ['/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', '/usr/bin/google-chrome', '/usr/bin/google-chrome-stable', '/usr/bin/chromium'].find((p) => existsSync(p));
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
const state = () => ev(`JSON.parse(localStorage.getItem('goup-lms-prototype-v2'))`);
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
  check('draft v2 copies 5 lessons with same keys', draft && draft.no === 2 && draft.lessons.length === 5 && draft.lessons.map((l) => l.key).join() === st.stageVersions.find((v) => v.id === 'sv-db-1').lessons.map((l) => l.key).join());
  const v1 = st.stageVersions.find((v) => v.id === 'sv-db-1');
  check('draft v2 copies bài học with the same keys, new ids, and học liệu remapped', draft.units.map((u) => u.key).join() === v1.units.map((u) => u.key).join()
    && draft.units.every((u) => !v1.units.some((x) => x.id === u.id)) && draft.lessons.every((l) => draft.units.some((u) => u.id === l.unitId)), draft.units.map((u) => u.key + ':' + u.id).join(' '));
  check('second clone is blocked (one draft max)', !(await ev(`!!document.querySelector('[data-action="clone-sv"]')`)) );

  // step 2: edit draft (add a học liệu to the "Thiết kế bảng" bài học, try an empty bài học, move a học liệu across bài học)
  check('stage editor groups học liệu under each bài học', (await ev(`[...document.querySelectorAll('.unit-head h3')].map((h)=>h.textContent).join('|')`)) === 'Nhập môn SQL|Thiết kế bảng|Kiểm tra cuối chặng');
  const design = draft.units.find((u) => u.key === 'ux-db-design');
  await click(`[data-action="new-lesson"][data-unit="${design.id}"]`);
  check('the add form preselects the bài học of the clicked button', (await ev(`document.querySelector('#modal form').elements.unitId.value`)) === design.id);
  await fill({ title: 'JOIN và subquery', type: 'video', video: 'db-04-join.mp4', duration: '12:40', required: true });
  await submitModal();
  st = await state(); let d2 = st.stageVersions.find((v) => v.id === draft.id); const join_ = d2.lessons.find((l) => l.title === 'JOIN và subquery');
  const inDesign = (d) => d.lessons.filter((l) => l.unitId === design.id).map((l) => l.key);
  check('lesson added to draft', d2.lessons.length === 6 && !!join_, JSON.stringify(join_));
  check('new học liệu goes last in the chosen bài học', join_.unitId === design.id && inDesign(d2).join() === 'db-table,db-index,' + join_.key, inDesign(d2).join());
  await click(`[data-action="new-unit"][data-sv="${draft.id}"]`); await fill({ title: 'Truy vấn nâng cao' }); await submitModal();
  st = await state(); d2 = st.stageVersions.find((v) => v.id === draft.id); const extraUnit = d2.units.at(-1);
  check('bài học is added at the end of the stage', extraUnit.title === 'Truy vấn nâng cao' && d2.units.length === 4);
  check('an empty bài học blocks publishing', (await text()).includes('Bài học "Truy vấn nâng cao" chưa có học liệu.'));
  await click(`[data-action="move-unit"][data-id="${extraUnit.id}"][data-dir="-1"]`);
  st = await state(); d2 = st.stageVersions.find((v) => v.id === draft.id);
  check('bài học can be moved up', d2.units.map((u) => u.title).join('|') === 'Nhập môn SQL|Thiết kế bảng|Truy vấn nâng cao|Kiểm tra cuối chặng');
  await click(`[data-action="move-lesson"][data-id="${join_.id}"][data-dir="1"]`);
  st = await state(); d2 = st.stageVersions.find((v) => v.id === draft.id);
  check('moving the last học liệu of a bài học down puts it first in the next bài học', d2.lessons.find((l) => l.id === join_.id).unitId === extraUnit.id);
  await click(`[data-action="edit-lesson"][data-id="${join_.id}"]`); await fill({ unitId: design.id }); await submitModal();
  st = await state(); d2 = st.stageVersions.find((v) => v.id === draft.id);
  check('the edit form moves a học liệu to another bài học', d2.lessons.find((l) => l.id === join_.id).unitId === design.id && inDesign(d2).at(-1) === join_.key);
  await click(`[data-action="delete-unit"][data-id="${extraUnit.id}"]`); await submitModal();
  st = await state(); d2 = st.stageVersions.find((v) => v.id === draft.id);
  check('the empty bài học is deleted and publishing unblocks', d2.units.length === 3 && !(await text()).includes('Chưa phát hành được'));
  check('published v1 untouched', st.stageVersions.find((v) => v.id === 'sv-db-1').lessons.length === 5);
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
  const lessonHref = await ev(`[...document.querySelectorAll('.lesson-row:not(.is-done)[data-kind=video], .lesson-row:not(.is-done)[data-kind=markdown]')].filter(r=>!/Không bắt buộc|tùy chọn/i.test(r.innerText)).map(r=>r.querySelector('a.lesson-link').getAttribute('href'))[0]`);
  check('roadmap groups each stage by bài học with progress per bài học', (await ev(`document.querySelectorAll('.roadmap .unit').length`)) === 16
    && /Bài 3\.1 · Cài đặt và Hello World\s*1\/1 bắt buộc\s*Hoàn thành/i.test(await ev(`[...document.querySelectorAll('.roadmap .unit-head')].map((h)=>h.innerText).join('\\n')`)), await ev(`[...document.querySelectorAll('.roadmap .unit-head')].slice(6,8).map((h)=>h.innerText.replace(/\\s+/g,' ')).join(' / ')`));
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
  // prev and next walk the học liệu of a bài học, then cross into the next bài học
  await go('#/learn/classes/cl-basic01/lessons/l9');
  const navOf = () => ev(`[...document.querySelectorAll('.complete-bar a.un-btn')].map((a)=>a.innerText.trim()+'>'+a.getAttribute('href').split('/').pop()).join(' ')`);
  check('lesson breadcrumb shows the bài học', (await ev(`document.querySelector('.crumbs').innerText`)).includes('Golang basic') && (await ev(`document.querySelector('.crumbs').innerText`)).includes('Cài đặt và Hello World'));
  check('next stays inside the bài học', /Học liệu tiếp>l36/i.test(await navOf()), await navOf());
  check('lesson side list groups học liệu by bài học', (await ev(`[...document.querySelectorAll('.side-unit')].map((h)=>h.textContent).join('|')`)) === 'Cài đặt và Hello World|Kiểu dữ liệu và hàm|Goroutine và channel|Kiểm tra cuối chặng');
  await go('#/learn/classes/cl-basic01/lessons/l12');
  check('next crosses into the next bài học from its last học liệu', /Học liệu trước>l10 Sang bài học tiếp>l11/i.test(await navOf()), await navOf());

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


  // --- quiz, homework, grading and stage summary flows
  await go('#/admin/stages/st-db?as=u-admin'); await click('[data-action="reset-data"]'); await submitModal(); await sleep(300);
  const anM = (await state()).members.find((m) => m.classId === 'cl-basic01' && m.userId === 'u-an').id;
  // assessment quiz: start, answer every question, submit
  await go('#/learn/classes/cl-basic01/lessons/l14?as=u-an');
  check('assessment quiz intro shows attempts used', /1\/2/.test(await text()));
  await click('[data-action="start-quiz"]'); await sleep(300);
  check('starting a quiz shows questions and a running timer', (await ev(`document.querySelectorAll('.quiz-take .quiz-q').length`)) === 5 && /\d\d:\d\d/.test(await ev(`document.querySelector('[data-timer-text]').textContent`)));
  const visibleQ = `[...document.querySelectorAll('.quiz-take .quiz-q')].filter((f) => !f.hidden)`;
  check('focus mode shows one question at a time without the lesson list', (await ev(`${visibleQ}.length`)) === 1 && (await ev(`!!document.querySelector('.viewer.is-focus') && !document.querySelector('.viewer .side-list')`)) && /Câu 1\/5/.test(await ev(`document.querySelector('.quiz-pos').textContent`)), await ev(`document.querySelector('.quiz-pos')?.textContent`));
  check('previous is disabled on the first question', (await ev(`document.querySelector('[data-action="quiz-prev"]').disabled`)) === true);
  await click('[data-action="quiz-next"]');
  check('next moves to question 2 and focuses it', (await ev(`document.querySelector('[data-pos]').textContent`)) === '2' && (await ev(`${visibleQ}[0] === document.activeElement && ${visibleQ}[0] === document.querySelectorAll('.quiz-take .quiz-q')[1]`)) && (await ev(`document.querySelector('.qm-btn[aria-current="step"]').dataset.idx`)) === '1');
  await shot('10b-quiz-focus');
  await click('.qm-btn[data-idx="4"]');
  check('question map jumps to the last question and shows submit', (await ev(`document.querySelector('[data-pos]').textContent`)) === '5' && (await ev(`document.querySelector('[data-action="quiz-next"]').hidden && !document.querySelector('[data-step-submit]').hidden`)) === true);
  await ev(`[...document.querySelectorAll('.quiz-take .quiz-q')].slice(0, 4).forEach((f) => f.querySelector('input').click())`); await sleep(200);
  st = await state(); let open = st.quizAttempts.find((a) => a.memberId === anM && a.status === 'open');
  check('answers are saved as they are picked', open && Object.keys(open.answers).length === 4 && /4\/5/.test(await ev(`document.querySelector('[data-answered]').textContent`)), JSON.stringify(open?.answers));
  check('question map marks answered questions', (await ev(`document.querySelectorAll('.qm-btn.is-answered').length`)) === 4 && /đã trả lời/.test(await ev(`document.querySelector('.qm-btn[data-idx="0"]').getAttribute('aria-label')`)));
  await click('.quiz-bar [data-action="submit-quiz"]');
  check('submit dialog warns about the unanswered question', /1 câu chưa trả lời/.test(await ev(`document.querySelector('#modal').innerText`)));
  await submitModal(); await sleep(300);
  st = await state(); const sub2 = st.quizAttempts.find((a) => a.id === open.id);
  check('submitted attempt is scored and shows the result', sub2.status === 'submitted' && /result=/.test(await ev('location.hash')) && !!(await ev(`document.querySelector('.quiz-score')`)), `${sub2.score}/${sub2.max}`);
  check('assessment result hides the answer key', (await ev(`document.querySelectorAll('.quiz-q.is-review').length`)) === 0);
  check('new assessment attempt waits for teacher confirmation and supersedes the older unconfirmed one', sub2.review?.status === 'pending' && st.quizAttempts.find((a) => a.id === 'qa7').review?.status === 'superseded' && /Chờ giảng viên xác nhận/.test(await text()), JSON.stringify(sub2.review));
  await go('#/learn/classes/cl-basic01/lessons/l14');
  check('start button is disabled when attempts run out', (await ev(`document.querySelector('[data-action="start-quiz"]')?.getAttribute('aria-disabled')`)) === 'true');
  // practice quiz: review shows correct answers and explanations
  await go('#/learn/classes/cl-basic01/lessons/l7?result=qa1');
  check('practice result reviews every question with the key', (await ev(`document.querySelectorAll('.quiz-q.is-review').length`)) === 5 && (await ev(`document.querySelectorAll('.quiz-opt.is-correct').length`)) >= 5 && (await ev(`document.querySelectorAll('.quiz-explain').length`)) > 0);
  check('practice result shows the teacher comment and per-question notes', !!(await ev(`document.querySelector('.review-box')`)) && (await ev(`document.querySelectorAll('.quiz-q.is-review .qnote-view').length`)) === 2);
  await shot('11-quiz-review');
  // teacher confirms the assessment attempt from the shared queue
  await go('#/teach/grading?as=u-gv');
  check('grading queue lists the quiz attempt waiting for confirmation', (await ev(`!!document.querySelector('[data-href*="/grading/quiz/${sub2.id}"], a[href*="/grading/quiz/${sub2.id}"]')`)) && /chờ xác nhận/i.test(await text()), await ev(`[...document.querySelectorAll('table tbody tr[data-href]')].map((r) => r.dataset.href).join(' ')`));
  await go('#/teach/grading?type=quiz');
  check('queue type filter keeps only quiz rows', (await ev(`[...document.querySelectorAll('table tbody tr[data-href]')].every((r) => r.dataset.href.includes('/grading/quiz/'))`)) === true && (await ev(`document.querySelectorAll('table tbody tr[data-href]').length`)) >= 1);
  await go('#/teach/grading/quiz/qa7');
  check('superseded attempt links to the newest one and cannot be confirmed', (await ev(`document.querySelector('form[data-form=quiz-review] fieldset').disabled`)) === true && (await ev(`!!document.querySelector('.alert a[href*="/grading/quiz/${sub2.id}"]')`)));
  await go(`#/teach/grading/quiz/${sub2.id}`);
  check('review page marks every answer against the key', (await ev(`document.querySelectorAll('.quiz-q.is-review').length`)) === 5 && (await ev(`document.querySelectorAll('.quiz-opt.is-correct').length`)) >= 5 && (await ev(`document.querySelectorAll('input[name^="qnote-"]').length`)) === 5);
  await ev(`document.querySelector('form[data-form=quiz-review]').requestSubmit()`); await sleep(200);
  check('confirming without an overall comment is refused', !(await ev(`document.querySelector('form[data-form=quiz-review] [data-form-error]').hidden`)));
  await shot('11b-quiz-confirm');
  await ev(`(()=>{const f=document.querySelector('form[data-form=quiz-review]'); document.querySelector('input[name^="qnote-"]').value='Xem lại cách khai báo slice.'; f.comment.value='Nắm vững cú pháp cơ bản, cần ôn goroutine.'; f.requestSubmit(); return true})()`); await sleep(400);
  st = await state(); const conf = st.quizAttempts.find((a) => a.id === sub2.id);
  check('confirmation stores the comment and note and emails the student', conf.review?.status === 'confirmed' && conf.review.by === 'u-gv' && conf.review.comment.includes('goroutine') && Object.keys(conf.review.notes).length === 1 && st.emails[0].kind === 'quizReview' && st.emails[0].to === 'an.nguyen@gmail.com', JSON.stringify(conf.review));
  hash = await ev('location.hash');
  check('confirming moves on in the queue', hash.startsWith('#/teach/grading') && !hash.includes(sub2.id), hash);
  await go(`#/learn/classes/cl-basic01/lessons/l14?result=${sub2.id}&as=u-an`);
  check('student result shows the confirmation and notes without the key', /cần ôn goroutine/.test(await ev(`document.querySelector('.review-box')?.innerText || ''`)) && (await ev(`document.querySelectorAll('.qnote-list li').length`)) === 1 && (await ev(`document.querySelectorAll('.quiz-q.is-review').length`)) === 0);
  await go('#/teach/grading/quiz/qa1?as=u-gv');
  check('practice attempt review goes back to class quiz stats, not the queue', (await ev(`document.querySelector('form[data-form=quiz-review] a[href*="?tab=quiz"]')?.textContent || ''`)).includes('thống kê quiz'));
  // homework: invalid then valid submission
  await go('#/learn/classes/cl-basic01/lessons/l13?as=u-an');
  await ev(`(()=>{const f=document.querySelector('form[data-form=submit-homework]'); f.repoUrl.value='not a url'; f.requestSubmit(); return true})()`); await sleep(200);
  check('bad repository link is refused next to the form', !(await ev(`document.querySelector('form[data-form=submit-homework] [data-form-error]').hidden`)));
  await ev(`(()=>{const f=document.querySelector('form[data-form=submit-homework]'); f.repoUrl.value='https://github.com/hoangan-dev/go-csv-stats'; f.commitRef.value='9f2c1ab'; f.requestSubmit(); return true})()`); await sleep(300);
  st = await state(); const anSub = st.submissions.filter((x) => x.memberId === anM && x.lessonId === 'l13').pop();
  check('homework submission is stored as pending', anSub && anSub.status === 'pending' && anSub.no === 1 && anSub.repoUrl.includes('go-csv-stats'), JSON.stringify(anSub));
  await shot('12-homework-submitted');
  // teacher grading queue and rubric
  await go('#/teach/grading?as=u-gv'); t = await text();
  const queueRows = await ev(`document.querySelectorAll('table tbody tr[data-href]').length`);
  check('grading queue lists pending submissions oldest first', queueRows >= 3 && t.includes('Nguyễn Hoàng An'), `${queueRows} rows`);
  await go(`#/teach/grading/${anSub.id}`);
  await ev(`(()=>{const f=document.querySelector('form[data-form=grade]'); const s=[...f.querySelectorAll('[data-grade-score]')]; s[0].value='40'; s[0].dispatchEvent(new Event('input',{bubbles:true})); return true})()`);
  check('running total waits for every criterion', /Còn 2 tiêu chí/.test(await ev(`document.querySelector('[data-grade-total]').innerText`)));
  await ev(`(()=>{const f=document.querySelector('form[data-form=grade]'); const s=[...f.querySelectorAll('[data-grade-score]')]; s[1].value='35'; s[2].value='15'; s[2].dispatchEvent(new Event('input',{bubbles:true})); return true})()`);
  check('running total shows score and pass state', /90\/100[\s\S]*Đạt/.test(await ev(`document.querySelector('[data-grade-total]').innerText`)), await ev(`document.querySelector('[data-grade-total]').innerText`));
  await shot('13-grading');
  await ev(`(()=>{const f=document.querySelector('form[data-form=grade]'); f.comment.value='Đúng yêu cầu, xử lý lỗi rõ ràng.'; f.requestSubmit(); return true})()`); await sleep(400);
  st = await state(); const graded = st.submissions.find((x) => x.id === anSub.id);
  check('grading stores the result and emails the student', graded.status === 'passed' && graded.grades.length === 1 && st.emails.some((e) => e.kind === 'graded'), graded.status);
  check('saving a grade moves to the next queued item', /\/grading\//.test(await ev('location.hash')) && !(await ev('location.hash')).includes(anSub.id), await ev('location.hash'));
  // stage summary matrix and teacher comment
  await go('#/teach/classes/cl-basic01?tab=summary&as=u-gv');
  check('class matrix shows a cell per student and stage', (await ev(`document.querySelectorAll('.matrix-cell').length`)) >= 10);
  await click(`.matrix-cell[data-member="${anM}"][data-sv="sv-go-1"]`);
  await ev(`(()=>{const f=document.querySelector('form[data-form=stage-review]'); f.text.value='Tiến bộ tốt ở chặng Go.'; f.requestSubmit(); return true})()`); await sleep(300);
  st = await state(); check('stage review is saved from the drawer', st.stageReviews[`${anM}:sv-go-1`]?.text === 'Tiến bộ tốt ở chặng Go.');
  await go('#/teach/classes/cl-basic01?tab=summary&rework=1');
  check('matrix filter narrows to students with rework', (await ev(`document.querySelectorAll('.matrix tbody tr').length`)) >= 1 && /rework=1/.test(await ev('location.hash')));
  // daily email jobs
  await go('#/admin?as=u-admin'); await click('[data-action="run-daily-jobs"]'); await sleep(200);
  st = await state(); check('daily jobs send digests and reminders', st.emails.some((e) => e.kind === 'digest') && st.emails.some((e) => e.kind === 'reminder'), st.emails.map((e) => e.kind).join());
  // deadline and extra attempts
  await go('#/admin/classes/cl-basic01?tab=homework&as=u-admin');
  await click('[data-action="set-deadline"][data-lesson="l13"]'); await fill({ due: '2026-12-01T17:00' }); await submitModal();
  st = await state(); check('deadline is saved for the class', !!st.deadlines['cl-basic01:l13'] && st.deadlines['cl-basic01:l13'].startsWith('2026-12-01'), st.deadlines['cl-basic01:l13']);
  await go('#/admin/classes/cl-basic01?tab=quiz');
  await click(`[data-action="grant-attempts"][data-member="${anM}"]`); await fill({ extra: '1', reason: 'Mất kết nối khi làm bài' }); await submitModal();
  st = await state(); check('extra attempt is granted with a reason', st.attemptGrants.length === 1 && st.attemptGrants[0].reason.includes('Mất kết nối'));
  // quizzes are authored by admin only; teachers review attempts but never open an authoring screen
  await go('#/teach?as=u-gv');
  check('teacher navigation has no quiz authoring entry', (await ev(`!document.querySelector('a[href="#/teach/quizzes"]') && !document.querySelector('[data-action="new-lesson"]')`)) === true);
  await go('#/teach/quizzes');
  check('teacher quiz authoring route is gone', (await ev('location.hash')) === '#/teach', await ev('location.hash'));
  await go('#/admin/stages/st-go/lessons/l14');
  check('teacher cannot open the admin quiz editor', (await ev('location.hash')) === '#/teach', await ev('location.hash'));
  // publish blockers for an empty quiz in a draft
  await go('#/admin/stages/st-ds?as=u-admin'); await click('[data-action="clone-sv"][data-id="sv-ds-1"]'); await sleep(300);
  st = await state(); const dsDraft = st.stageVersions.find((v) => v.stageId === 'st-ds' && v.status === 'draft');
  await click(`[data-action="new-lesson"][data-sv="${dsDraft.id}"]`);
  check('lesson type select lists every kind, both quizzes included', (await ev(`[...document.querySelector('#modal form').elements.type.options].map((o)=>o.value).join()`)) === 'video,markdown,quiz:practice,quiz:assessment,homework');
  await fill({ title: 'Kiểm tra chặng DS', type: 'quiz:assessment' });
  check('picking Quiz bài tập shows attempt and time limits', (await ev(`!document.querySelector('#modal [data-quiz-limits]').hidden && !document.querySelector('#modal [data-lesson-section="quiz"]').hidden`)) === true); await submitModal(); await sleep(300);
  check('new quiz lesson opens the question editor', /\/admin\/stages\/st-ds\/lessons\/l/.test(await ev('location.hash')), await ev('location.hash'));
  st = await state(); const dsQuiz = st.stageVersions.find((v) => v.id === dsDraft.id).lessons.find((l) => l.title === 'Kiểm tra chặng DS');
  check('the combined kind saves an assessment quiz', dsQuiz.type === 'quiz' && dsQuiz.quiz.mode === 'assessment' && dsQuiz.quiz.attempts === 2, JSON.stringify(dsQuiz.quiz));
  await go(`#/admin/stages/st-ds?v=${dsDraft.id}`); await click(`[data-action="publish-sv"][data-id="${dsDraft.id}"]`);
  check('publishing a quiz without questions is blocked', !(await ev(`document.querySelector('#modal').open`)) && /Chưa phát hành được/.test(await ev(`document.querySelector('#toasts').innerText`)));
  st = await state(); const newQuiz = st.stageVersions.find((v) => v.id === dsDraft.id).lessons.find((l) => l.title === 'Kiểm tra chặng DS');
  await go(`#/admin/stages/st-ds/lessons/${newQuiz.id}`);
  await click('[data-action="new-question"]'); await fill({ text: 'Stack là LIFO?', opt0: 'Đúng', opt1: 'Sai', correct0: true }); await submitModal();
  st = await state(); check('question is added to the draft quiz', st.stageVersions.find((v) => v.id === dsDraft.id).lessons.find((l) => l.id === newQuiz.id).quiz.questions.length === 1);
  await go(`#/admin/stages/st-ds?v=${dsDraft.id}`); await click(`[data-action="publish-sv"][data-id="${dsDraft.id}"]`);
  check('publish opens once the quiz has a keyed question', await ev(`document.querySelector('#modal').open`));
  await click('[data-action="close-modal"]');
  await go('#/admin/stages/st-db?as=u-admin'); await click('[data-action="reset-data"]'); await submitModal(); await sleep(300);

  // --- accessibility, reflow and keyboard probes
  const ROUTES = ['#/login', '#/admin?as=u-admin', '#/admin/stages/st-db?as=u-admin', '#/admin/stages/st-go?v=sv-go-2&as=u-admin', '#/admin/courses/co-basic?as=u-admin', '#/admin/classes/cl-basic01?as=u-admin', '#/admin/classes/cl-basic01?tab=report&as=u-admin', '#/teach?as=u-gv', '#/learn?as=u-an', '#/learn/classes/cl-basic01?as=u-an', '#/learn/classes/cl-basic01/lessons/l1?as=u-an',
    '#/admin/classes/cl-basic01?tab=summary&as=u-admin', '#/admin/classes/cl-basic01?tab=homework&as=u-admin', '#/admin/classes/cl-basic01?tab=quiz&as=u-admin',
    '#/teach/grading?as=u-gv', '#/teach/grading/sub4?as=u-gv', '#/admin/stages/st-ds/lessons/l7?as=u-admin', '#/admin/stages/st-go/lessons/l13?as=u-admin',
    '#/learn/classes/cl-basic01/summary?as=u-an', '#/learn/classes/cl-basic01/lessons/l14?as=u-an', '#/learn/classes/cl-basic01/lessons/l13?as=u-an', '#/learn/classes/cl-basic01/lessons/l7?as=u-an&result=qa1',
    '#/teach/grading/quiz/qa7?as=u-gv', '#/teach/grading/quiz/qa9?as=u-gv', '#/teach/grading/quiz/qa1?as=u-gv', '#/learn/classes/cl-basic01/lessons/l14?as=u-cuong&result=qa9'];
  await send('Emulation.setDeviceMetricsOverride', { width: 320, height: 812, deviceScaleFactor: 1, mobile: true }, sessionId);
  for (const h of ROUTES) {
    await go(h);
    const r = await ev(`({ inner: window.innerWidth, scroll: document.documentElement.scrollWidth })`);
    check(`320px reflow ${h.split('?')[0]}`, r.inner === 320 && r.scroll <= 320, `innerWidth ${r.inner}, scrollWidth ${r.scroll}`);
  }
  await go('#/learn/classes/cl-basic01/lessons/l7?as=u-an'); await click('[data-action="start-quiz"]'); await sleep(300);
  const rf = await ev(`({ scroll: document.documentElement.scrollWidth, map: [...document.querySelectorAll('.qm-btn')].every((b) => b.getBoundingClientRect().width >= 44 && b.getBoundingClientRect().height >= 44) })`);
  check('320px reflow focus-mode quiz with 44px map buttons', rf.scroll <= 320 && rf.map, JSON.stringify(rf));
  await go('#/admin/stages/st-db?as=u-admin'); await click('[data-action="reset-data"]'); await submitModal(); await sleep(300);
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
