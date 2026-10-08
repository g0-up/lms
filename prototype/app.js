/* GoUp LMS prototype — single-page app.
   State lives in memory and is mirrored to localStorage so reloads keep your walk-through.
   Nothing here talks to a server; every rule from the spec is enforced in the functions
   under "Domain rules" so the prototype behaves like the real system would. */
(function () {
  'use strict';

  const STORE_KEY = 'goup-lms-prototype-v2';
  const TEMP_PASSWORD_HOURS = 72;
  const GRADING_SLA_DAYS = 3;          // a submission waiting longer than this is overdue for grading
  const SUBMIT_GRACE_MS = 30e3;        // quiz answers are still accepted this long after the timer ends
  const MAX_FILES = 5, MAX_FILE_MB = 20;
  const FILE_EXTS = ['zip', 'pdf', 'md', 'txt', 'sql', 'png', 'jpg', 'jpeg'];
  const REPO_RE = /^https:\/\/(www\.)?(github\.com|gitlab\.com|bitbucket\.org)\/[^/\s]+\/[^/\s]+/i;
  let S = null;

  /* ---------------------------------------------------------------- utils */
  const $ = (sel, root = document) => root.querySelector(sel);
  const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const uid = (p) => p + Math.random().toString(36).slice(2, 9);
  const nowISO = () => new Date().toISOString();
  const pad = (n) => String(n).padStart(2, '0');
  const fmtDate = (iso) => { if (!iso) return '—'; const d = new Date(iso); return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`; };
  const fmtDateTime = (iso) => { if (!iso) return '—'; const d = new Date(iso); return `${fmtDate(iso)} ${pad(d.getHours())}:${pad(d.getMinutes())}`; };
  const daysSince = (iso) => iso ? (Date.now() - new Date(iso).getTime()) / 864e5 : Infinity;
  function rel(iso) {
    if (!iso) return 'Chưa có';
    const m = (Date.now() - new Date(iso).getTime()) / 6e4;
    if (m < 1) return 'Vừa xong';
    if (m < 60) return `${Math.round(m)} phút trước`;
    if (m < 60 * 24) return `${Math.round(m / 60)} giờ trước`;
    const d = Math.round(m / 1440);
    if (d < 30) return `${d} ngày trước`;
    return fmtDate(iso);
  }
  const initials = (name) => name.split(/\s+/).filter(Boolean).slice(-2).map((w) => w[0]).join('').toUpperCase();
  const normEmail = (e) => String(e || '').trim().toLowerCase();
  const plural = (n, w) => `${n} ${w}`;

  /* ------------------------------------------------------------ icons */
  const ICONS = {
    compass: '<circle cx="12" cy="12" r="10"/><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"/>',
    layers: '<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>',
    book: '<path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>',
    play: '<polygon points="6 3 20 12 6 21 6 3"/>',
    'file-text': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    'check-circle': '<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>',
    'chevron-up': '<path d="m18 15-6-6-6 6"/>',
    'chevron-down': '<path d="m6 9 6 6 6-6"/>',
    'chevron-right': '<path d="m9 18 6-6-6-6"/>',
    plus: '<path d="M5 12h14"/><path d="M12 5v14"/>',
    x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>',
    mail: '<rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>',
    pencil: '<path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/>',
    trash: '<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>',
    menu: '<path d="M4 12h16"/><path d="M4 6h16"/><path d="M4 18h16"/>',
    'arrow-right': '<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>',
    rotate: '<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>',
    'log-out': '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/>',
    copy: '<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>',
    archive: '<rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/>',
    clock: '<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>',
    sort: '<path d="m21 16-4 4-4-4"/><path d="M17 20V4"/><path d="m3 8 4-4 4 4"/><path d="M7 4v16"/>',
    lock: '<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
    grid: '<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/>',
    upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/>',
    'help-circle': '<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>',
    clipboard: '<rect width="8" height="4" x="8" y="2" rx="1" ry="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>',
    'clipboard-check': '<rect width="8" height="4" x="8" y="2" rx="1" ry="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><path d="m9 14 2 2 4-4"/>',
    timer: '<line x1="10" x2="14" y1="2" y2="2"/><line x1="12" x2="15" y1="14" y2="11"/><circle cx="12" cy="14" r="8"/>',
    download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/>',
    link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
    paperclip: '<path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48"/>',
    message: '<path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/>',
    'x-circle': '<circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/>',
    circle: '<circle cx="12" cy="12" r="10"/>',
    calendar: '<rect width="18" height="18" x="3" y="4" rx="2" ry="2"/><line x1="16" x2="16" y1="2" y2="6"/><line x1="8" x2="8" y1="2" y2="6"/><line x1="3" x2="21" y1="10" y2="10"/>',
    inbox: '<polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>',
    send: '<path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/>',
    'bar-chart': '<path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/>'
  };
  const TYPE_ICON = { video: 'play', markdown: 'file-text', quiz: 'help-circle', homework: 'clipboard' };
  const TYPE_VI = { video: 'Video', markdown: 'Bài đọc', quiz: 'Quiz', homework: 'Bài tập' };
  const typeIcon = (l, cls = '') => icon(TYPE_ICON[l.type] || 'file-text', cls);
  const icon = (name, cls = '') => `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ''}</svg>`;
  const brandMark = `<svg class="brand-mark" viewBox="0 0 28 28" aria-hidden="true"><rect width="12" height="28" fill="currentColor"/><rect x="16" y="4" width="12" height="8" rx="4" fill="#f2295b"/><rect x="16" y="16" width="12" height="12" fill="#d8e2ec"/></svg>`;

  /* ------------------------------------------------------- persistence */
  function load() {
    try { const raw = localStorage.getItem(STORE_KEY); if (raw) { S = JSON.parse(raw); return; } } catch (e) { /* storage unavailable: run in memory */ }
    S = window.LMS_SEED(Date.now());
  }
  function save() { try { localStorage.setItem(STORE_KEY, JSON.stringify(S)); } catch (e) { /* ignore */ } }
  function resetData() { S = window.LMS_SEED(Date.now()); save(); }

  /* ---------------------------------------------------------- lookups */
  const byId = (list, id) => list.find((x) => x.id === id);
  const user = (id) => byId(S.users, id);
  const stage = (id) => byId(S.stages, id);
  const sv = (id) => byId(S.stageVersions, id);
  const course = (id) => byId(S.courses, id);
  const cv = (id) => byId(S.courseVersions, id);
  const cls = (id) => byId(S.classes, id);
  const me = () => (S.session ? user(S.session.userId) : null);
  const stageVersionsOf = (stageId) => S.stageVersions.filter((v) => v.stageId === stageId).sort((a, b) => a.no - b.no);
  const courseVersionsOf = (courseId) => S.courseVersions.filter((v) => v.courseId === courseId).sort((a, b) => a.no - b.no);
  const latestPublished = (versions) => versions.filter((v) => v.status === 'published').sort((a, b) => b.no - a.no)[0] || null;
  const draftOf = (versions) => versions.find((v) => v.status === 'draft') || null;
  const membersOf = (classId) => S.members.filter((m) => m.classId === classId);
  const activeMembersOf = (classId) => membersOf(classId).filter((m) => m.status === 'active');
  const memberOf = (classId, userId) => S.members.find((m) => m.classId === classId && m.userId === userId);
  const progressOf = (memberId, lessonId) => S.progress[memberId + ':' + lessonId] || null;
  const latestInvitation = (classId, userId) => S.invitations.filter((i) => i.classId === classId && i.userId === userId).sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0] || null;
  const cvsUsingSV = (svId) => S.courseVersions.filter((v) => v.stages.includes(svId));
  const classesUsingCV = (cvId) => S.classes.filter((c) => c.courseVersionId === cvId);
  const versionLabel = (v) => `v${v.no}`;
  const STATUS_VI = { draft: 'Nháp', published: 'Đã phát hành', archived: 'Lưu trữ', active: 'Đang chạy', ended: 'Đã kết thúc', queued: 'Đang chờ gửi', sent: 'Đã gửi', failed: 'Gửi thất bại', invited: 'Chưa đăng nhập', disabled: 'Vô hiệu hóa', dropped: 'Đã rời lớp' };
  const classStatusVi = { draft: 'Nháp', active: 'Đang chạy', ended: 'Đã kết thúc' };

  function memberProgress(member) {
    const c = cls(member.classId); const v = cv(c.courseVersionId);
    const stages = v.stages.map((svId) => {
      const s = sv(svId); const req = s.lessons.filter((l) => l.required);
      const done = req.filter((l) => lessonStatus(member, l).done).length;
      return { stageId: s.stageId, svId, done, total: req.length, pct: req.length ? Math.round((done / req.length) * 100) : 0 };
    });
    const total = stages.reduce((a, s) => a + s.total, 0); const done = stages.reduce((a, s) => a + s.done, 0);
    return { stages, done, total, pct: total ? Math.round((done / total) * 100) : 0 };
  }
  function classAvgProgress(classId) {
    const ms = activeMembersOf(classId); if (!ms.length) return 0;
    return Math.round(ms.reduce((a, m) => a + memberProgress(m).pct, 0) / ms.length);
  }
  // FR-18: courses whose latest published version still carries an older version of this stage.
  function outdatedCourses(stageId) {
    const latest = latestPublished(stageVersionsOf(stageId)); if (!latest) return [];
    const out = [];
    S.courses.forEach((co) => {
      const lcv = latestPublished(courseVersionsOf(co.id)); if (!lcv) return;
      const usedId = lcv.stages.find((id) => sv(id).stageId === stageId); if (!usedId) return;
      const used = sv(usedId); if (used.no >= latest.no) return;
      const draft = draftOf(courseVersionsOf(co.id));
      out.push({ course: co, cv: lcv, used, latest, blocked: draft ? `Khóa học đang có bản nháp ${versionLabel(draft)}. Phát hành hoặc xóa bản nháp trước.` : null });
    });
    return out;
  }
  function log(action, target) { S.audit.unshift({ at: nowISO(), by: me()?.id || 'u-admin', action, target }); }
  const ok = (data) => ({ ok: true, ...data });
  const err = (error) => ({ ok: false, error });

  /* ------------------------------------------------------ domain rules */
  // Stages and stage versions (FR-10 .. FR-13, FR-19)
  function createStage(code, name) {
    code = code.trim().toUpperCase(); name = name.trim();
    if (!code || !name) return err('Nhập mã và tên chặng.');
    if (S.stages.some((s) => s.code === code)) return err(`Mã chặng ${code} đã tồn tại.`);
    const st = { id: uid('st-'), code, name }; S.stages.push(st);
    S.stageVersions.push({ id: uid('sv-'), stageId: st.id, no: 1, status: 'draft', clonedFrom: null, publishedAt: null, lessons: [] });
    log('Tạo chặng', `${name} (${code})`); save(); return ok({ id: st.id });
  }
  function cloneStage(fromId) {
    const from = sv(fromId); if (from.status === 'draft') return err('Chỉ nhân bản từ phiên bản đã phát hành.');
    const versions = stageVersionsOf(from.stageId); const draft = draftOf(versions);
    if (draft) return err(`Chặng đã có bản nháp ${versionLabel(draft)}.`, draft.id);
    const v = { id: uid('sv-'), stageId: from.stageId, no: Math.max(...versions.map((x) => x.no)) + 1, status: 'draft', clonedFrom: from.id, publishedAt: null,
      lessons: from.lessons.map((l) => { // same lesson_key; video reference copied, not the file
        const c = JSON.parse(JSON.stringify(l)); c.id = uid('l-');
        if (c.quiz) c.quiz.questions.forEach((x) => { x.id = uid('q-'); x.options.forEach((o) => { o.id = uid('o-'); }); }); // question keys stay stable
        if (c.homework) c.homework.criteria.forEach((x) => { x.id = uid('c-'); });
        return c; }) };
    S.stageVersions.push(v); log('Nhân bản chặng', `${stage(from.stageId).name} ${versionLabel(from)} → ${versionLabel(v)}`); save(); return ok({ id: v.id });
  }
  function publishStage(svId) {
    const v = sv(svId); if (v.status !== 'draft') return err('Chỉ phát hành được bản nháp.');
    const blockers = stagePublishBlockers(v); if (blockers.length) return err(blockers.join(' '));
    v.status = 'published'; v.publishedAt = nowISO(); log('Phát hành chặng', `${stage(v.stageId).name} ${versionLabel(v)}`); save(); return ok();
  }
  function archiveStageVersion(svId) { const v = sv(svId); if (v.status !== 'published') return err('Chỉ lưu trữ phiên bản đã phát hành.'); v.status = 'archived'; log('Lưu trữ chặng', `${stage(v.stageId).name} ${versionLabel(v)}`); save(); return ok(); }
  function deleteStageVersion(svId) {
    const v = sv(svId); const refs = cvsUsingSV(svId);
    if (v.status !== 'draft' && refs.length) return err(`Đang được dùng trong ${refs.map((c) => `${course(c.courseId).name} ${versionLabel(c)}`).join(', ')}. Hãy lưu trữ thay vì xóa.`);
    S.stageVersions = S.stageVersions.filter((x) => x.id !== svId);
    const left = stageVersionsOf(v.stageId); if (!left.length) S.stages = S.stages.filter((s) => s.id !== v.stageId);
    log('Xóa phiên bản chặng', `${stage(v.stageId)?.name || ''} ${versionLabel(v)}`); save(); return ok({ stageGone: !left.length });
  }
  function assertDraft(v) { return v.status === 'draft' ? null : 'Phiên bản đã phát hành không sửa được. Nhân bản thành bản nháp mới để sửa.'; }
  function saveLesson(svId, lessonId, data) {
    const v = sv(svId); const e = assertDraft(v); if (e) return err(e);
    const title = String(data.title || '').trim(); if (!title) return err('Nhập tiêu đề học liệu.');
    const prev = lessonId ? v.lessons.find((x) => x.id === lessonId) : null;
    const base = { title, type: data.type, required: !!data.required };
    if (data.type === 'video') {
      if (!String(data.video || '').trim()) return err('Học liệu video bắt buộc có file video.');
      Object.assign(base, { video: data.video.trim(), duration: data.duration || '—' });
    } else if (data.type === 'markdown') {
      if (!String(data.content || '').trim()) return err('Học liệu markdown bắt buộc có nội dung.');
      base.content = data.content;
    } else if (data.type === 'quiz') {
      const mode = data.mode === 'assessment' ? 'assessment' : 'practice'; const threshold = Number(data.quizThreshold || 70);
      if (!Number.isInteger(threshold) || threshold < 1 || threshold > 100) return err('Ngưỡng đạt là số nguyên từ 1 đến 100%.');
      let attempts = null, timeLimit = null;
      if (mode === 'assessment') {
        attempts = Number(data.attempts); timeLimit = Number(data.timeLimit);
        if (!Number.isInteger(attempts) || attempts < 1 || attempts > 10) return err('Số lượt làm bài từ 1 đến 10.');
        if (!Number.isInteger(timeLimit) || timeLimit < 1 || timeLimit > 180) return err('Thời gian làm bài từ 1 đến 180 phút.');
      }
      base.quiz = { mode, attempts, timeLimit, threshold, shuffle: !!data.shuffle, questions: prev?.quiz?.questions || [] };
    } else if (data.type === 'homework') {
      const threshold = Number(data.hwThreshold || 70);
      if (!Number.isInteger(threshold) || threshold < 1 || threshold > 100) return err('Ngưỡng đạt là số nguyên từ 1 đến 100%.');
      base.homework = { brief: String(data.brief || ''), threshold, attachments: String(data.attachments || '').split(',').map((x) => x.trim()).filter(Boolean), criteria: prev?.homework?.criteria || [] };
    } else return err('Chọn loại học liệu.');
    if (prev) { ['video', 'duration', 'content', 'quiz', 'homework'].forEach((k) => delete prev[k]); Object.assign(prev, base); save(); return ok({ id: prev.id }); }
    const l = { id: uid('l-'), key: uid('k-'), ...base }; v.lessons.push(l); save(); return ok({ id: l.id });
  }
  function deleteLesson(svId, lessonId) { const v = sv(svId); const e = assertDraft(v); if (e) return err(e); v.lessons = v.lessons.filter((l) => l.id !== lessonId); save(); return ok(); }
  function moveLesson(svId, lessonId, dir) {
    const v = sv(svId); const e = assertDraft(v); if (e) return err(e);
    const i = v.lessons.findIndex((l) => l.id === lessonId); const j = i + dir; if (j < 0 || j >= v.lessons.length) return ok();
    [v.lessons[i], v.lessons[j]] = [v.lessons[j], v.lessons[i]]; save(); return ok();
  }

  // Courses and course versions (FR-14 .. FR-17, FR-19)
  function createCourse(code, name) {
    code = code.trim().toUpperCase(); name = name.trim();
    if (!code || !name) return err('Nhập mã và tên khóa học.');
    if (S.courses.some((c) => c.code === code)) return err(`Mã khóa học ${code} đã tồn tại.`);
    const co = { id: uid('co-'), code, name }; S.courses.push(co);
    S.courseVersions.push({ id: uid('cv-'), courseId: co.id, no: 1, status: 'draft', clonedFrom: null, publishedAt: null, stages: [] });
    log('Tạo khóa học', `${name} (${code})`); save(); return ok({ id: co.id });
  }
  function cloneCourse(fromId) {
    const from = cv(fromId); const versions = courseVersionsOf(from.courseId); const draft = draftOf(versions);
    if (draft) return err(`Khóa học đã có bản nháp ${versionLabel(draft)}.`);
    const v = { id: uid('cv-'), courseId: from.courseId, no: Math.max(...versions.map((x) => x.no)) + 1, status: 'draft', clonedFrom: from.id, publishedAt: null, stages: from.stages.slice() };
    S.courseVersions.push(v); log('Nhân bản khóa học', `${course(from.courseId).name} ${versionLabel(from)} → ${versionLabel(v)}`); save(); return ok({ id: v.id });
  }
  function addStageToCourse(cvId, svId) {
    const v = cv(cvId); const e = assertDraft(v); if (e) return err(e);
    const s = sv(svId); if (s.status !== 'published') return err('Chỉ gắn được phiên bản chặng đã phát hành.');
    if (v.stages.some((id) => sv(id).stageId === s.stageId)) return err('Khóa học đã chứa một phiên bản của chặng này.');
    v.stages.push(svId); save(); return ok();
  }
  function replaceStageInCourse(cvId, oldSvId, newSvId) {
    const v = cv(cvId); const e = assertDraft(v); if (e) return err(e);
    const i = v.stages.indexOf(oldSvId); if (i < 0) return err('Không tìm thấy chặng.');
    if (sv(newSvId).status !== 'published') return err('Chỉ gắn được phiên bản chặng đã phát hành.');
    v.stages[i] = newSvId; save(); return ok();
  }
  function removeStageFromCourse(cvId, svId) { const v = cv(cvId); const e = assertDraft(v); if (e) return err(e); v.stages = v.stages.filter((id) => id !== svId); save(); return ok(); }
  function moveStageInCourse(cvId, svId, dir) {
    const v = cv(cvId); const e = assertDraft(v); if (e) return err(e);
    const i = v.stages.indexOf(svId); const j = i + dir; if (j < 0 || j >= v.stages.length) return ok();
    [v.stages[i], v.stages[j]] = [v.stages[j], v.stages[i]]; save(); return ok();
  }
  function coursePublishBlocker(v) {
    if (!v.stages.length) return 'Khóa học cần ít nhất một chặng.';
    const notPub = v.stages.map(sv).filter((s) => s.status !== 'published');
    if (notPub.length) return `Chặng chưa phát hành: ${notPub.map((s) => `${stage(s.stageId).name} ${versionLabel(s)}`).join(', ')}.`;
    return null;
  }
  function publishCourse(cvId) {
    const v = cv(cvId); if (v.status !== 'draft') return err('Chỉ phát hành được bản nháp.');
    const b = coursePublishBlocker(v); if (b) return err(b);
    v.status = 'published'; v.publishedAt = nowISO(); log('Phát hành khóa học', `${course(v.courseId).name} ${versionLabel(v)}`); save(); return ok();
  }
  function archiveCourseVersion(cvId) { const v = cv(cvId); if (v.status !== 'published') return err('Chỉ lưu trữ phiên bản đã phát hành.'); v.status = 'archived'; log('Lưu trữ khóa học', `${course(v.courseId).name} ${versionLabel(v)}`); save(); return ok(); }
  function deleteCourseVersion(cvId) {
    const v = cv(cvId); const refs = classesUsingCV(cvId);
    if (v.status !== 'draft' && refs.length) return err(`Đang được dùng bởi lớp ${refs.map((c) => c.code).join(', ')}. Hãy lưu trữ thay vì xóa.`);
    S.courseVersions = S.courseVersions.filter((x) => x.id !== cvId);
    const left = courseVersionsOf(v.courseId); if (!left.length) S.courses = S.courses.filter((c) => c.id !== v.courseId);
    log('Xóa phiên bản khóa học', `${course(v.courseId)?.name || ''} ${versionLabel(v)}`); save(); return ok({ courseGone: !left.length });
  }
  // FR-17: one transaction — clone latest published course version, swap the stage, publish.
  function applyStageVersion(svId, courseId) {
    const s = sv(svId); if (s.status !== 'published') return err('Phiên bản chặng chưa phát hành.');
    const versions = courseVersionsOf(courseId); const latest = latestPublished(versions);
    if (!latest) return err('Khóa học chưa có phiên bản phát hành.');
    const idx = latest.stages.findIndex((id) => sv(id).stageId === s.stageId);
    if (idx < 0) return err('Khóa học không chứa chặng này.');
    if (latest.stages[idx] === svId) return err('Khóa học đã dùng phiên bản này.');
    const draft = draftOf(versions); if (draft) return err(`Khóa học đang có bản nháp ${versionLabel(draft)}. Phát hành hoặc xóa bản nháp trước.`);
    const v = { id: uid('cv-'), courseId, no: Math.max(...versions.map((x) => x.no)) + 1, status: 'published', clonedFrom: latest.id, publishedAt: nowISO(), stages: latest.stages.slice() };
    v.stages[idx] = svId; S.courseVersions.push(v);
    log('Áp dụng chặng cho khóa học', `${stage(s.stageId).name} ${versionLabel(s)} → ${course(courseId).name} ${versionLabel(v)}`); save();
    return ok({ id: v.id, no: v.no, from: latest.no });
  }

  // Classes (FR-20 .. FR-23)
  function createClass(d) {
    const code = d.code.trim().toLowerCase(); const name = d.name.trim();
    if (!code || !name) return err('Nhập mã và tên lớp.');
    if (S.classes.some((c) => c.code === code)) return err(`Mã lớp ${code} đã tồn tại.`);
    const v = cv(d.courseVersionId); if (!v || v.status !== 'published') return err('Chọn một phiên bản khóa học đã phát hành.');
    if (!d.startDate || !d.endDate) return err('Nhập ngày bắt đầu và kết thúc dự kiến.');
    if (d.endDate < d.startDate) return err('Ngày kết thúc phải sau ngày bắt đầu.');
    if (!d.teacherId) return err('Chọn giảng viên phụ trách.');
    const c = { id: uid('cl-'), code, name, courseVersionId: v.id, status: 'draft', startDate: d.startDate, endDate: d.endDate, teacherId: d.teacherId, createdAt: nowISO() };
    S.classes.push(c); log('Tạo lớp', code); save(); return ok({ id: c.id });
  }
  function updateClass(classId, d) {
    const c = cls(classId); const name = d.name.trim(); if (!name) return err('Nhập tên lớp.');
    if (d.endDate < d.startDate) return err('Ngày kết thúc phải sau ngày bắt đầu.');
    if (d.courseVersionId !== c.courseVersionId) {
      if (c.status !== 'draft') return err(`Lớp ${classStatusVi[c.status].toLowerCase()} không đổi được phiên bản khóa học. Lớp chạy trọn đời trên một phiên bản.`);
      const v = cv(d.courseVersionId); if (!v || v.status !== 'published') return err('Chỉ đổi sang phiên bản đã phát hành.');
      log('Đổi phiên bản khóa học của lớp', `${c.code}: ${versionLabel(cv(c.courseVersionId))} → ${versionLabel(v)}`);
      c.courseVersionId = v.id;
    }
    Object.assign(c, { name, startDate: d.startDate, endDate: d.endDate, teacherId: d.teacherId }); save(); return ok();
  }
  function setClassStatus(classId, status) {
    const c = cls(classId);
    const next = { draft: 'active', active: 'ended' }[c.status];
    if (status !== next) return err('Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc.');
    c.status = status; log(status === 'active' ? 'Kích hoạt lớp' : 'Kết thúc lớp', c.code); save(); return ok();
  }

  // Accounts and invitations (FR-01 .. FR-06)
  function queueEmail(inv) {
    inv.status = 'queued'; inv.attempts = 0; inv.lastError = null;
    setTimeout(() => {
      const u = user(inv.userId); const bounce = /bounce|fail/.test(u.email);
      inv.attempts = bounce ? 3 : 1; inv.status = bounce ? 'failed' : 'sent'; inv.lastError = bounce ? 'Mailbox không tồn tại (550 5.1.1)' : null;
      save(); render();
    }, 1500);
  }
  function invite(classId, emailRaw, nameRaw) {
    const c = cls(classId); const email = normEmail(emailRaw); const name = String(nameRaw || '').trim();
    if (c.status === 'ended') return err('Không mời được vào lớp đã kết thúc.');
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return err('Email không hợp lệ.');
    let u = S.users.find((x) => x.email === email); let kind;
    if (u && u.role !== 'student') return err('Email này thuộc tài khoản nội bộ, không mời làm học viên được.');
    if (u && u.status === 'disabled') return err('Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời.');
    if (u) {
      const m = memberOf(classId, u.id);
      if (m && m.status !== 'dropped') return err('Học viên đã có trong lớp.');
      if (m) m.status = 'active'; else S.members.push({ id: uid('m-'), classId, userId: u.id, status: 'active', joinedAt: nowISO() });
      kind = u.mustChangePassword ? 'invite' : 'added';
      if (u.mustChangePassword) u.tempPasswordExpiresAt = new Date(Date.now() + TEMP_PASSWORD_HOURS * 36e5).toISOString();
    } else {
      if (!name) return err('Nhập họ tên học viên.');
      u = { id: uid('u-'), role: 'student', name, email, status: 'invited', mustChangePassword: true, lastLoginAt: null, lastActiveAt: null, tempPasswordExpiresAt: new Date(Date.now() + TEMP_PASSWORD_HOURS * 36e5).toISOString() };
      S.users.push(u); S.members.push({ id: uid('m-'), classId, userId: u.id, status: 'active', joinedAt: nowISO() }); kind = 'invite';
    }
    const inv = { id: uid('inv-'), classId, userId: u.id, sentBy: me().id, status: 'queued', attempts: 0, lastError: null, createdAt: nowISO(), kind };
    S.invitations.push(inv); queueEmail(inv); log('Mời học viên', `${c.code} · ${email}`); save();
    return ok({ kind, user: u });
  }
  function resendInvite(classId, userId) {
    const u = user(userId); if (!u.mustChangePassword) return err('Học viên đã đổi mật khẩu; không cần gửi lại lời mời.');
    u.tempPasswordExpiresAt = new Date(Date.now() + TEMP_PASSWORD_HOURS * 36e5).toISOString();
    const inv = { id: uid('inv-'), classId, userId, sentBy: me().id, status: 'queued', attempts: 0, lastError: null, createdAt: nowISO(), kind: 'invite' };
    S.invitations.push(inv); queueEmail(inv); log('Gửi lại lời mời', u.email); save(); return ok();
  }
  function removeMember(memberId) { const m = byId(S.members, memberId); m.status = 'dropped'; log('Gỡ học viên khỏi lớp', `${cls(m.classId).code} · ${user(m.userId).email}`); save(); return ok(); }
  function disableUser(userId) {
    const u = user(userId); u.status = 'disabled'; if (S.session?.userId === userId) S.session = null;
    log('Vô hiệu hóa tài khoản', u.email); save(); return ok();
  }
  function enableUser(userId) { const u = user(userId); u.status = u.mustChangePassword ? 'invited' : 'active'; log('Kích hoạt lại tài khoản', u.email); save(); return ok(); }

  function login(emailRaw, password) {
    const email = normEmail(emailRaw); const u = S.users.find((x) => x.email === email);
    if (!u || !password) return err('Email hoặc mật khẩu không đúng.');
    if (u.status === 'disabled') return err('Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên.');
    if (u.mustChangePassword && u.tempPasswordExpiresAt && new Date(u.tempPasswordExpiresAt) < new Date()) return err('Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên.');
    S.session = { userId: u.id }; u.lastLoginAt = nowISO(); u.lastActiveAt = nowISO(); save(); return ok({ mustChange: u.mustChangePassword });
  }
  function changePassword(pw, confirm) {
    const u = me(); if (pw.length < 8) return err('Mật khẩu mới cần tối thiểu 8 ký tự.');
    if (pw !== confirm) return err('Hai mật khẩu không khớp.');
    if (pw === 'tam-thoi-123') return err('Mật khẩu mới phải khác mật khẩu tạm.');
    u.mustChangePassword = false; u.status = 'active'; u.tempPasswordExpiresAt = null; save(); return ok();
  }
  function switchRole(userId) { const u = user(userId); S.session = { userId }; u.lastActiveAt = nowISO(); save(); }

  // Learning (FR-30 .. FR-33)
  function openLesson(classId, lessonId) {
    const u = me(); const m = memberOf(classId, u.id); if (!m || m.status !== 'active') return err('Bạn không còn là thành viên đang học của lớp.');
    const key = m.id + ':' + lessonId;
    if (!S.progress[key]) S.progress[key] = { firstOpenedAt: nowISO(), completedAt: null };
    u.lastActiveAt = nowISO(); save(); return ok();
  }
  function setComplete(classId, lessonId, done) {
    const u = me(); const c = cls(classId); const m = memberOf(classId, u.id);
    if (!m || m.status !== 'active') return err('Bạn không còn là thành viên đang học của lớp.');
    if (c.status !== 'active') return err(c.status === 'draft' ? 'Lớp chưa bắt đầu.' : 'Lớp đã kết thúc, chỉ xem được học liệu.');
    if (!cv(c.courseVersionId).stages.some((id) => sv(id).lessons.some((l) => l.id === lessonId))) return err('Học liệu không thuộc phiên bản khóa học của lớp.');
    const l = lessonById(lessonId); if (l.type === 'quiz' || l.type === 'homework') return err('Quiz và bài tập được tính hoàn thành theo kết quả, không tích tay được.');
    const p = S.progress[m.id + ':' + lessonId]; if (!p) return err('Mở học liệu trước khi tích hoàn thành.');
    p.completedAt = done ? (p.completedAt || nowISO()) : null; u.lastActiveAt = nowISO(); save(); return ok();
  }

  // Lesson lookups shared by quiz, homework, grading and summaries
  function lessonById(id) { for (const v of S.stageVersions) { const l = v.lessons.find((x) => x.id === id); if (l) return l; } return null; }
  const svOfLesson = (id) => S.stageVersions.find((v) => v.lessons.some((l) => l.id === id)) || null;
  const classLessons = (c) => cv(c.courseVersionId).stages.map(sv).flatMap((s) => s.lessons);
  const lessonInClass = (c, lessonId) => classLessons(c).find((l) => l.id === lessonId) || null;
  const shortTitle = (l) => l.title.replace(/^(Bài tập|Luyện tập|Kiểm tra):\s*/, '');
  const deadlineOf = (classId, lessonId) => S.deadlines[classId + ':' + lessonId] || null;
  const subsOf = (memberId, lessonId) => S.submissions.filter((s) => s.memberId === memberId && s.lessonId === lessonId).sort((a, b) => a.no - b.no);
  const latestSub = (memberId, lessonId) => { const l = subsOf(memberId, lessonId); return l[l.length - 1] || null; };
  const attemptsOf = (memberId, lessonId) => S.quizAttempts.filter((a) => a.memberId === memberId && a.lessonId === lessonId).sort((a, b) => a.no - b.no);
  const doneAttempts = (memberId, lessonId) => attemptsOf(memberId, lessonId).filter((a) => a.status === 'submitted');
  const openAttempt = (memberId, lessonId) => S.quizAttempts.find((a) => a.memberId === memberId && a.lessonId === lessonId && a.status === 'open') || null;
  const grantsOf = (memberId, lessonId) => S.attemptGrants.filter((g) => g.memberId === memberId && g.lessonId === lessonId);
  // null means unlimited (practice quizzes); granted extra attempts add to the authored limit
  const attemptLimit = (memberId, l) => l.quiz.attempts == null ? null : l.quiz.attempts + grantsOf(memberId, l.id).reduce((a, g) => a + g.extra, 0);
  const pctOf = (score, max) => max ? Math.round((score / max) * 100) : 0;
  const quizBest = (memberId, lessonId) => doneAttempts(memberId, lessonId).reduce((b, a) => (!b || a.score / a.max > b.score / b.max ? a : b), null);
  const canGrade = (u, c) => !!u && !!c && (u.role === 'admin' || (u.role === 'teacher' && c.teacherId === u.id));
  const isOverdueGrading = (s) => s.status === 'pending' && daysSince(s.submittedAt) > GRADING_SLA_DAYS;
  const hwMax = (hw) => hw.criteria.reduce((a, c) => a + c.max, 0);

  // One status per lesson, used by progress %, the roadmap, summaries and reports.
  // Video and reading: the student ticks it. Practice quiz: any submitted attempt.
  // Assessment quiz: best attempt reaches the threshold. Homework: latest submission graded "Đạt".
  function lessonStatus(m, l) {
    const S_ = (done, state, label, tone) => ({ done, state, label, tone });
    if (l.type === 'quiz') {
      const done = doneAttempts(m.id, l.id); const open = openAttempt(m.id, l.id);
      if (l.quiz.mode === 'practice') return done.length ? S_(true, 'done', 'Đã làm', 'ok') : open ? S_(false, 'progress', 'Đang làm', 'info') : S_(false, 'todo', 'Chưa làm', 'optional');
      const best = quizBest(m.id, l.id); const pct = best ? pctOf(best.score, best.max) : 0;
      if (best && pct >= l.quiz.threshold) return S_(true, 'passed', `Đạt ${pct}%`, 'ok');
      if (open) return S_(false, 'progress', 'Đang làm', 'info');
      if (best) { const left = attemptLimit(m.id, l) - done.length; return left > 0 ? S_(false, 'failed', `Chưa đạt ${pct}%`, 'warn') : S_(false, 'failed', `Chưa đạt ${pct}% · hết lượt`, 'danger'); }
      return S_(false, 'todo', 'Chưa làm', 'optional');
    }
    if (l.type === 'homework') {
      const s = latestSub(m.id, l.id); const due = deadlineOf(m.classId, l.id);
      if (s?.status === 'passed') return S_(true, 'passed', 'Đạt', 'ok');
      if (s?.status === 'pending') return S_(false, 'pending', 'Chờ chấm', 'info');
      if (s?.status === 'rework') return S_(false, 'rework', 'Cần làm lại', 'warn');
      if (due && new Date(due) < new Date()) return S_(false, 'overdue', 'Quá hạn', 'danger');
      return S_(false, 'todo', 'Chưa nộp', 'optional');
    }
    const pr = progressOf(m.id, l.id);
    return pr?.completedAt ? S_(true, 'done', 'Đã học xong', 'ok') : pr ? S_(false, 'opened', 'Đã mở', 'optional') : S_(false, 'todo', 'Chưa mở', 'optional');
  }
  function stageSummary(m, svId) {
    const s = sv(svId); const rows = s.lessons.map((l) => ({ l, st: lessonStatus(m, l) }));
    const missing = rows.filter((r) => r.l.required && !r.st.done);
    return { sv: s, stage: stage(s.stageId), rows, missing, complete: !missing.length, review: S.stageReviews[m.id + ':' + svId] || null,
      rework: rows.some((r) => r.st.state === 'rework'), overdue: rows.some((r) => r.st.state === 'overdue') };
  }
  const courseSummary = (m) => cv(cls(m.classId).courseVersionId).stages.map((id) => stageSummary(m, id));

  // Quizzes
  function shuffled(arr) { const a = arr.slice(); for (let i = a.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [a[i], a[j]] = [a[j], a[i]]; } return a; }
  function learnerGate(classId) {
    const u = me(); const c = cls(classId); const m = c && memberOf(classId, u.id);
    if (!m || m.status !== 'active') return { error: 'Bạn không còn là thành viên đang học của lớp.' };
    if (c.status !== 'active') return { error: c.status === 'draft' ? 'Lớp chưa bắt đầu.' : 'Lớp đã kết thúc, chỉ xem được học liệu.' };
    return { u, c, m };
  }
  function startAttempt(classId, lessonId) {
    const g = learnerGate(classId); if (g.error) return err(g.error);
    const l = lessonInClass(g.c, lessonId); if (!l || l.type !== 'quiz') return err('Bài quiz không thuộc lớp này.');
    const open = openAttempt(g.m.id, l.id); if (open) return ok({ id: open.id, resumed: true });
    const used = attemptsOf(g.m.id, l.id).length; const limit = attemptLimit(g.m.id, l);
    if (limit != null && used >= limit) return err(`Bạn đã dùng hết ${limit} lượt làm bài.`);
    const qs = l.quiz.shuffle ? shuffled(l.quiz.questions) : l.quiz.questions; const start = Date.now();
    const a = { id: uid('qa-'), memberId: g.m.id, lessonId: l.id, no: used + 1, startedAt: new Date(start).toISOString(),
      deadlineAt: l.quiz.timeLimit ? new Date(start + l.quiz.timeLimit * 60e3).toISOString() : null, submittedAt: null, savedAt: null,
      answers: {}, order: qs.map((q) => q.id), optOrder: Object.fromEntries(qs.map((q) => [q.id, (l.quiz.shuffle ? shuffled(q.options) : q.options).map((o) => o.id)])),
      score: null, max: null, status: 'open' };
    S.quizAttempts.push(a); openLesson(classId, l.id); save(); return ok({ id: a.id });
  }
  const pastGrace = (a) => !!a.deadlineAt && Date.now() > new Date(a.deadlineAt).getTime() + SUBMIT_GRACE_MS;
  function saveAnswer(attemptId, qid, oids) {
    const a = byId(S.quizAttempts, attemptId); if (!a || a.status !== 'open') return err('Lượt làm bài này đã được nộp.');
    if (pastGrace(a)) return err('Đã hết giờ làm bài; câu trả lời không được lưu.');
    a.answers[qid] = oids; a.savedAt = nowISO(); save(); return ok({ savedAt: a.savedAt });
  }
  // All-or-nothing per question: the chosen set must equal the set of correct options.
  function scoreAttempt(a) {
    const l = lessonById(a.lessonId); let score = 0, max = 0;
    l.quiz.questions.forEach((q) => { max += q.points; const right = q.options.filter((o) => o.correct).map((o) => o.id).sort().join(); const given = (a.answers[q.id] || []).slice().sort().join(); if (given && given === right) score += q.points; });
    return { score, max };
  }
  // A submit that arrives after deadline + grace is not accepted as-is: the attempt closes with the answers saved in time.
  function submitAttempt(attemptId, { auto = false } = {}) {
    const a = byId(S.quizAttempts, attemptId); if (!a || a.status !== 'open') return err('Lượt làm bài này đã được nộp.');
    const late = pastGrace(a);
    Object.assign(a, scoreAttempt(a), { status: 'submitted', submittedAt: late ? a.deadlineAt : nowISO(), autoSubmitted: auto || late });
    save(); return ok({ id: a.id, late });
  }
  function sweepExpiredAttempts() { S.quizAttempts.filter((a) => a.status === 'open' && pastGrace(a)).forEach((a) => submitAttempt(a.id, { auto: true })); }
  function grantAttempts(memberId, lessonId, extra, reason) {
    const u = me(); const m = byId(S.members, memberId); const c = m && cls(m.classId);
    if (!canGrade(u, c)) return err('Bạn chỉ cấp thêm lượt cho lớp mình phụ trách.');
    const l = lessonInClass(c, lessonId); if (!l || l.type !== 'quiz' || l.quiz.mode !== 'assessment') return err('Chỉ cấp thêm lượt cho bài kiểm tra.');
    extra = Number(extra); if (!Number.isInteger(extra) || extra < 1 || extra > 3) return err('Số lượt cấp thêm từ 1 đến 3.');
    reason = String(reason || '').trim(); if (!reason) return err('Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử.');
    S.attemptGrants.push({ id: uid('gr-'), memberId, lessonId, extra, reason, by: u.id, at: nowISO() });
    log('Cấp thêm lượt làm bài', `${user(m.userId).name} · ${shortTitle(l)} · +${extra}`); save(); return ok();
  }

  // Homework, grading and deadlines
  function submitHomework(classId, lessonId, d) {
    const g = learnerGate(classId); if (g.error) return err(g.error);
    const l = lessonInClass(g.c, lessonId); if (!l || l.type !== 'homework') return err('Bài tập không thuộc lớp này.');
    const prev = latestSub(g.m.id, l.id); if (prev?.status === 'passed') return err('Bài đã Đạt nên không nộp lại được.');
    const repoUrl = String(d.repoUrl || '').trim(); const commitRef = String(d.commitRef || '').trim(); const files = d.files || [];
    if (!repoUrl && !files.length) return err('Nộp đường dẫn repo hoặc ít nhất một file.');
    if (repoUrl && !REPO_RE.test(repoUrl)) return err('Đường dẫn repo phải là https trên GitHub, GitLab hoặc Bitbucket, ví dụ https://github.com/ten-ban/ten-repo.');
    if (commitRef && !repoUrl) return err('Commit hoặc tag chỉ đi kèm đường dẫn repo.');
    if (files.length > MAX_FILES) return err(`Tối đa ${MAX_FILES} file mỗi lần nộp.`);
    const bad = files.find((f) => !FILE_EXTS.includes(f.name.split('.').pop().toLowerCase())); if (bad) return err(`File ${bad.name} không đúng định dạng. Chấp nhận: ${FILE_EXTS.join(', ')}.`);
    const big = files.find((f) => f.size > MAX_FILE_MB * 1048576); if (big) return err(`File ${big.name} lớn hơn ${MAX_FILE_MB} MB.`);
    const due = deadlineOf(classId, l.id); const at = nowISO(); const replaced = prev?.status === 'pending' ? prev.no : null;
    if (replaced) prev.status = 'superseded';
    const s = { id: uid('sub-'), memberId: g.m.id, lessonId: l.id, no: (prev?.no || 0) + 1, submittedAt: at, repoUrl: repoUrl || null, commitRef: commitRef || null,
      files: files.map((f) => ({ name: f.name, size: f.size })), late: !!due && new Date(at) > new Date(due), status: 'pending', grades: [] };
    S.submissions.push(s); openLesson(classId, l.id); save(); return ok({ no: s.no, late: s.late, replaced });
  }
  function gradeSubmission(subId, d) {
    const u = me(); const s = byId(S.submissions, subId); if (!s) return err('Không tìm thấy bài nộp.');
    const m = byId(S.members, s.memberId); const c = cls(m.classId);
    if (!canGrade(u, c)) return err('Bạn chỉ chấm được bài của lớp mình phụ trách.');
    if (latestSub(s.memberId, s.lessonId).id !== s.id) return err('Chỉ chấm bài nộp mới nhất. Học viên đã nộp lần sau.');
    const l = lessonById(s.lessonId); const hw = l.homework; const scores = [];
    for (const cr of hw.criteria) {
      const raw = d['score-' + cr.id]; const n = Number(raw);
      if (raw == null || raw === '' || !Number.isFinite(n)) return err(`Nhập điểm cho tiêu chí "${cr.text}".`);
      if (n < 0 || n > cr.max) return err(`Điểm tiêu chí "${cr.text}" phải từ 0 đến ${cr.max}.`);
      scores.push({ criterionId: cr.id, score: n, comment: String(d['note-' + cr.id] || '').trim() });
    }
    const comment = String(d.comment || '').trim(); if (!comment) return err('Nhập nhận xét chung cho học viên.');
    const total = scores.reduce((a, x) => a + x.score, 0); const max = hwMax(hw);
    const result = total * 100 >= hw.threshold * max ? 'passed' : 'rework'; const edit = s.grades.length > 0;
    s.grades.push({ by: u.id, at: nowISO(), scores, total, max, result, comment }); s.status = result;
    const stu = user(m.userId);
    S.emails.unshift({ id: uid('em-'), to: stu.email, kind: 'graded', subject: `${edit ? 'Điểm đã được cập nhật' : 'Bài đã được chấm'}: ${shortTitle(l)} · ${result === 'passed' ? 'Đạt' : 'Cần làm lại'}`, at: nowISO() });
    log(edit ? 'Sửa điểm' : 'Chấm bài', `${stu.name} · ${shortTitle(l)} #${s.no} · ${result === 'passed' ? 'Đạt' : 'Cần làm lại'}`); save();
    return ok({ result, total, max, edit });
  }
  function setDeadline(classId, lessonId, value) {
    const u = me(); const c = cls(classId); if (!canGrade(u, c)) return err('Bạn chỉ đặt hạn nộp cho lớp mình phụ trách.');
    if (c.status === 'ended') return err('Lớp đã kết thúc, không đổi hạn nộp được.');
    const l = lessonInClass(c, lessonId); if (!l || l.type !== 'homework') return err('Bài tập không thuộc lớp này.');
    if (value && Number.isNaN(new Date(value).getTime())) return err('Hạn nộp không hợp lệ.');
    const key = classId + ':' + lessonId; const old = S.deadlines[key] || null; const next = value ? new Date(value).toISOString() : null;
    if (old === next) return ok();
    if (next) S.deadlines[key] = next; else delete S.deadlines[key];
    log(!old ? 'Đặt hạn nộp' : next ? 'Đổi hạn nộp' : 'Bỏ hạn nộp', `${c.code} · ${shortTitle(l)}${old ? ` · ${fmtDateTime(old)}` : ''}${next ? ` → ${fmtDateTime(next)}` : ''}`); save(); return ok();
  }
  function saveStageReview(memberId, svId, text) {
    const u = me(); const m = byId(S.members, memberId); if (!canGrade(u, cls(m.classId))) return err('Bạn chỉ nhận xét học viên của lớp mình phụ trách.');
    text = String(text || '').trim(); if (!text) return err('Nhập nội dung nhận xét.');
    const key = memberId + ':' + svId; const prev = S.stageReviews[key];
    S.stageReviews[key] = { text, by: u.id, at: nowISO(), history: prev ? [...(prev.history || []), { text: prev.text, by: prev.by, at: prev.at }] : [] };
    log(prev ? 'Sửa nhận xét tổng kết chặng' : 'Nhận xét tổng kết chặng', `${user(m.userId).name} · ${stage(sv(svId).stageId).name}`); save(); return ok();
  }
  function gradingQueue(u, f = {}) {
    return S.submissions.filter((s) => s.status === 'pending').map((s) => {
      const m = byId(S.members, s.memberId); const c = cls(m.classId);
      return { s, m, c, student: user(m.userId), lesson: lessonById(s.lessonId), svx: svOfLesson(s.lessonId), waited: daysSince(s.submittedAt), overdue: isOverdueGrading(s) };
    }).filter((r) => canGrade(u, r.c) && (!f.classId || r.c.id === f.classId) && (!f.lessonId || r.lesson.id === f.lessonId))
      .sort((a, b) => a.s.submittedAt.localeCompare(b.s.submittedAt));
  }
  // Per teacher: queue size, items past the grading SLA, and mean hours from submission to first grade.
  function teacherBacklog() {
    return S.users.filter((u) => u.role === 'teacher').map((t) => {
      const q = gradingQueue(t); const firsts = S.submissions.filter((s) => s.grades[0]?.by === t.id);
      const avgH = firsts.length ? firsts.reduce((a, s) => a + (new Date(s.grades[0].at) - new Date(s.submittedAt)) / 36e5, 0) / firsts.length : null;
      return { t, pending: q.length, overdue: q.filter((r) => r.overdue).length, avgH };
    });
  }
  // Simulated daily email job: grading digest for teachers with overdue items, and a reminder
  // to students who have not submitted homework due within 24 hours. Each email is sent once.
  function runDailyJobs() {
    const day = new Date().toISOString().slice(0, 10); let digests = 0, reminders = 0;
    S.users.filter((t) => t.role === 'teacher' && t.status !== 'disabled').forEach((t) => {
      const items = gradingQueue(t).filter((r) => r.overdue); const key = `digest:${t.id}:${day}`;
      if (!items.length || S.jobRuns[key]) return;
      S.jobRuns[key] = nowISO(); digests++;
      S.emails.unshift({ id: uid('em-'), to: t.email, kind: 'digest', subject: `${items.length} bài chờ chấm quá ${GRADING_SLA_DAYS} ngày: ${items.map((r) => r.student.name).join(', ')}`, at: nowISO() });
    });
    Object.entries(S.deadlines).forEach(([k, due]) => {
      const left = new Date(due) - Date.now(); if (left <= 0 || left > 864e5) return;
      const [classId, lessonId] = k.split(':'); const c = cls(classId); if (!c || c.status !== 'active') return; const l = lessonById(lessonId);
      activeMembersOf(classId).forEach((m) => {
        const stu = user(m.userId); if (stu.status === 'disabled' || subsOf(m.id, lessonId).length) return;
        const rk = `remind:${m.id}:${lessonId}:${due}`; if (S.jobRuns[rk]) return;
        S.jobRuns[rk] = nowISO(); reminders++;
        S.emails.unshift({ id: uid('em-'), to: stu.email, kind: 'reminder', subject: `Nhắc hạn nộp: ${shortTitle(l)} · hạn ${fmtDateTime(due)}`, at: nowISO() });
      });
    });
    if (digests || reminders) log('Chạy tác vụ email hằng ngày', `${digests} bản tổng hợp chấm bài · ${reminders} nhắc hạn nộp`);
    save(); return ok({ digests, reminders });
  }

  // Quiz and homework authoring
  function lessonBlockers(l) {
    if (l.type === 'quiz') {
      const qs = l.quiz?.questions || []; if (!qs.length) return [`"${l.title}" chưa có câu hỏi.`];
      const bad = qs.filter((x) => !x.options.some((o) => o.correct)).length;
      return bad ? [`"${l.title}" có ${bad} câu chưa đánh dấu đáp án đúng.`] : [];
    }
    if (l.type === 'homework') {
      const out = []; if (!String(l.homework?.brief || '').trim()) out.push(`"${l.title}" chưa có đề bài.`);
      if (!(l.homework?.criteria || []).length) out.push(`"${l.title}" chưa có tiêu chí chấm.`); return out;
    }
    return [];
  }
  const stagePublishBlockers = (v) => v.lessons.length ? v.lessons.flatMap(lessonBlockers) : ['Chặng cần ít nhất một học liệu trước khi phát hành.'];
  function editableLesson(svId, lessonId) {
    const v = sv(svId); const e = assertDraft(v); if (e) return { error: e };
    const l = v.lessons.find((x) => x.id === lessonId); return l ? { v, l } : { error: 'Không tìm thấy học liệu.' };
  }
  function moveIn(list, id, dir) { const i = list.findIndex((x) => x.id === id); const j = i + dir; if (i < 0 || j < 0 || j >= list.length) return; [list[i], list[j]] = [list[j], list[i]]; }
  function saveQuestion(svId, lessonId, qid, d) {
    const x = editableLesson(svId, lessonId); if (x.error) return err(x.error); const quiz = x.l.quiz;
    const text = String(d.text || '').trim(); if (!text) return err('Nhập nội dung câu hỏi.');
    const type = d.type === 'multi' ? 'multi' : 'single'; const points = Number(d.points || 1);
    if (!(points > 0)) return err('Điểm của câu hỏi phải lớn hơn 0.');
    const options = [];
    for (let i = 0; i < 6; i++) { const t = String(d['opt' + i] || '').trim(); if (t) options.push({ id: d['oid' + i] || uid('o-'), text: t, correct: d['correct' + i] === 'on' }); }
    if (options.length < 2) return err('Câu hỏi cần ít nhất 2 phương án.');
    const right = options.filter((o) => o.correct).length;
    if (!right) return err('Đánh dấu ít nhất một đáp án đúng.');
    if (type === 'single' && right > 1) return err('Câu một đáp án chỉ có một phương án đúng. Đổi sang "Nhiều đáp án" nếu cần.');
    const data = { text, type, points, options, explanation: String(d.explanation || '').trim() };
    const prev = qid && quiz.questions.find((q) => q.id === qid);
    if (prev) Object.assign(prev, data); else quiz.questions.push({ id: uid('q-'), key: uid('qk-'), ...data });
    save(); return ok();
  }
  function deleteQuestion(svId, lessonId, qid) { const x = editableLesson(svId, lessonId); if (x.error) return err(x.error); x.l.quiz.questions = x.l.quiz.questions.filter((q) => q.id !== qid); save(); return ok(); }
  function moveQuestion(svId, lessonId, qid, dir) { const x = editableLesson(svId, lessonId); if (x.error) return err(x.error); moveIn(x.l.quiz.questions, qid, dir); save(); return ok(); }
  function saveCriterion(svId, lessonId, cid, d) {
    const x = editableLesson(svId, lessonId); if (x.error) return err(x.error);
    const text = String(d.text || '').trim(); if (!text) return err('Nhập nội dung tiêu chí.');
    const max = Number(d.max); if (!Number.isInteger(max) || max < 1 || max > 100) return err('Điểm tối đa của tiêu chí là số nguyên từ 1 đến 100.');
    const prev = cid && x.l.homework.criteria.find((c) => c.id === cid);
    if (prev) Object.assign(prev, { text, max }); else x.l.homework.criteria.push({ id: uid('c-'), text, max });
    save(); return ok();
  }
  function deleteCriterion(svId, lessonId, cid) { const x = editableLesson(svId, lessonId); if (x.error) return err(x.error); x.l.homework.criteria = x.l.homework.criteria.filter((c) => c.id !== cid); save(); return ok(); }
  function moveCriterion(svId, lessonId, cid, dir) { const x = editableLesson(svId, lessonId); if (x.error) return err(x.error); moveIn(x.l.homework.criteria, cid, dir); save(); return ok(); }

  /* ------------------------------------------------- markdown renderer */
  function renderMarkdown(src) {
    const lines = String(src || '').split('\n'); const out = []; let i = 0;
    const inline = (t) => esc(t).replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    while (i < lines.length) {
      const ln = lines[i];
      if (ln.startsWith('```')) { const buf = []; let j = i + 1; while (j < lines.length && !lines[j].startsWith('```')) buf.push(lines[j++]); out.push(`<pre><code>${esc(buf.join('\n'))}</code></pre>`); i = j + 1; continue; }
      const h = ln.match(/^(#{1,3})\s+(.*)/); if (h) { out.push(`<h${h[1].length}>${inline(h[2])}</h${h[1].length}>`); i++; continue; }
      if (/^>\s?/.test(ln)) { out.push(`<blockquote>${inline(ln.replace(/^>\s?/, ''))}</blockquote>`); i++; continue; }
      if (/^[-*]\s+/.test(ln)) { const items = []; while (i < lines.length && /^[-*]\s+/.test(lines[i])) items.push(`<li>${inline(lines[i++].replace(/^[-*]\s+/, ''))}</li>`); out.push(`<ul>${items.join('')}</ul>`); continue; }
      if (/^\d+\.\s+/.test(ln)) { const items = []; while (i < lines.length && /^\d+\.\s+/.test(lines[i])) items.push(`<li>${inline(lines[i++].replace(/^\d+\.\s+/, ''))}</li>`); out.push(`<ol>${items.join('')}</ol>`); continue; }
      if (!ln.trim()) { i++; continue; }
      const buf = [ln]; i++; while (i < lines.length && lines[i].trim() && !/^(#{1,3}\s|```|[-*]\s|\d+\.\s|>)/.test(lines[i])) buf.push(lines[i++]);
      out.push(`<p>${inline(buf.join(' '))}</p>`);
    }
    return out.join('\n');
  }

  /* ------------------------------------------------------ UI primitives */
  function toast(msg, type = 'ok') {
    const host = $('#toasts'); const el = document.createElement('div');
    el.className = `toast toast-${type}`; el.setAttribute('role', 'status');
    el.innerHTML = `${icon(type === 'ok' ? 'check-circle' : 'alert')}<div class="grow">${esc(msg)}</div><button type="button" class="toast-close" data-action="close-toast" aria-label="Đóng thông báo">${icon('x')}</button>`;
    host.appendChild(el); while (host.children.length > 3) host.firstChild.remove();
    setTimeout(() => el.remove(), 4000);
  }
  let modalHandler = null;
  function openModal({ title, body, confirm = 'Lưu', cancel = 'Hủy', danger = false, primary = true, onSubmit }) {
    const dlg = $('#modal'); modalHandler = onSubmit;
    const btnCls = danger ? 'un-btn-danger' : (primary ? 'un-btn-primary' : 'un-btn-navy');
    dlg.innerHTML = `<form method="dialog" data-form="modal" novalidate>
      <div class="modal-head"><h2 id="modal-title">${esc(title)}</h2><button type="button" class="icon-btn" data-action="close-modal" aria-label="Đóng">${icon('x')}</button></div>
      <div class="modal-body">${body}<div class="alert alert-danger" data-modal-error hidden>${icon('alert')}<div class="alert-body"></div></div></div>
      <div class="modal-foot"><button type="button" class="un-btn un-btn-secondary un-btn-sm" data-action="close-modal">${esc(cancel)}</button>${confirm ? `<button type="submit" class="un-btn ${btnCls} un-btn-sm">${esc(confirm)}</button>` : ''}</div>
    </form>`;
    dlg.showModal(); const first = dlg.querySelector('input:not([type=hidden]), select, textarea'); if (first) first.focus();
  }
  function closeModal() { const dlg = $('#modal'); if (dlg.open) dlg.close(); modalHandler = null; }
  function modalError(msg) { const box = $('#modal [data-modal-error]'); box.hidden = false; box.querySelector('.alert-body').textContent = msg; }
  function openDrawer(html) { const d = $('#drawer'); d.querySelector('.panel').innerHTML = html; d.setAttribute('open', ''); document.body.style.overflow = 'hidden'; const f = d.querySelector('button'); if (f) f.focus(); }
  function closeDrawer() { const d = $('#drawer'); d.removeAttribute('open'); document.body.style.overflow = ''; }
  function confirmAction({ title, text, confirm, danger, onConfirm }) {
    openModal({ title, body: `<p>${text}</p>`, confirm, danger, primary: !danger, onSubmit: () => onConfirm() });
  }

  const badge = (status) => `<span class="badge badge-${status}">${esc(STATUS_VI[status] || status)}</span>`;
  const dot = (status, label) => `<span class="dot dot-${status}">${esc(label || STATUS_VI[status] || status)}</span>`;
  const progressBar = (pct, lg = false) => `<div class="progress${lg ? ' progress-lg' : ''}" role="img" aria-label="${pct}% hoàn thành"><div class="bar"><i style="width:${pct}%"></i></div><span class="pct">${pct}%</span></div>`;
  const vpill = (v, href, current) => `<a class="vpill is-${v.status}" href="${href}" ${current ? 'aria-current="true"' : ''} title="${esc(STATUS_VI[v.status])}">${versionLabel(v)}${v.status === 'draft' ? ' <span class="muted">nháp</span>' : ''}</a>`;
  const lineage = (versions, hrefFor, currentId) => `<div class="lineage">${versions.map((v, i) => `${i ? `<span class="arrow">${icon('chevron-right')}</span>` : ''}${vpill(v, hrefFor(v), v.id === currentId)}`).join('')}</div>`;
  const empty = (title, text, action = '') => `<div class="empty"><h3>${esc(title)}</h3><p>${esc(text)}</p>${action}</div>`;
  const field = (label, control, { help = '', required = false, error = '' } = {}) => `<div class="field${error ? ' has-error' : ''}"><label>${esc(label)}${required ? '<span class="req" aria-hidden="true">*</span>' : ''}${control}</label>${help ? `<div class="help">${help}</div>` : ''}${error ? `<div class="error">${esc(error)}</div>` : ''}</div>`;
  const selectOpts = (opts, value) => opts.map((o) => `<option value="${esc(o.value)}" ${o.value === value ? 'selected' : ''} ${o.disabled ? 'disabled' : ''}>${esc(o.label)}</option>`).join('');
  const stBadge = (st) => `<span class="badge badge-${st.tone}">${esc(st.label)}</span>`;
  const mdInline = (t) => esc(t).replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  function lessonMeta(l) {
    if (l.type === 'video') return `Video · ${esc(l.video)} · ${esc(l.duration || '')}`;
    if (l.type === 'markdown') return `Bài đọc · ${(l.content || '').length} ký tự`;
    if (l.type === 'quiz') { const z = l.quiz; return `Quiz ${z.mode === 'assessment' ? 'kiểm tra' : 'luyện tập'} · ${z.questions.length} câu · đạt từ ${z.threshold}%${z.mode === 'assessment' ? ` · ${z.attempts} lượt · ${z.timeLimit} phút` : ' · không giới hạn lượt'}`; }
    return `Bài tập · ${l.homework.criteria.length} tiêu chí · ${hwMax(l.homework)} điểm · đạt từ ${l.homework.threshold}%`;
  }
  const toLocalInput = (iso) => { if (!iso) return ''; const d = new Date(iso); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`; };
  const fmtSize = (b) => b >= 1048576 ? `${(b / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`;
  const fmtWait = (days) => days < 1 ? `${Math.max(1, Math.round(days * 24))} giờ` : `${Math.floor(days)} ngày ${Math.round((days % 1) * 24)} giờ`;
  function dueIn(iso) {
    const ms = new Date(iso) - Date.now(); const h = Math.abs(ms) / 36e5;
    const span = h < 1 ? `${Math.max(1, Math.round(h * 60))} phút` : h < 48 ? `${Math.round(h)} giờ` : `${Math.round(h / 24)} ngày`;
    return ms >= 0 ? `còn ${span}` : `quá hạn ${span}`;
  }
  const gradeHref = (subId) => `${me()?.role === 'admin' ? '#/admin' : '#/teach'}/grading/${subId}`;
  const go = (hash) => { if (location.hash === hash) render(); else location.hash = hash; };
  const EMAIL_KIND = { graded: 'Kết quả chấm', digest: 'Tổng hợp chấm bài', reminder: 'Nhắc hạn nộp' };
  let quizTimer = null;

  /* --------------------------------------------------------- routing */
  const homeFor = (u) => ({ admin: '#/admin', teacher: '#/teach', student: '#/learn' }[u.role]);
  function parseHash() {
    const h = location.hash || '#/'; const [path, qs] = h.slice(1).split('?');
    const query = {}; if (qs) qs.split('&').forEach((kv) => { const [k, v] = kv.split('='); query[decodeURIComponent(k)] = decodeURIComponent(v || ''); });
    return { path: path.replace(/\/+$/, '') || '/', query };
  }
  const q = (obj) => { const parts = Object.entries(obj).filter(([, v]) => v !== '' && v != null && v !== false).map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`); return parts.length ? '?' + parts.join('&') : ''; };
  const ROUTES = [
    { re: /^\/login$/, page: pageLogin, auth: false },
    { re: /^\/forgot$/, page: pageForgot, auth: false },
    { re: /^\/first-login$/, page: pageFirstLogin, roles: ['student', 'admin', 'teacher'] },
    { re: /^\/admin$/, page: pageAdminDashboard, roles: ['admin'] },
    { re: /^\/admin\/stages$/, page: pageStages, roles: ['admin'] },
    { re: /^\/admin\/stages\/([^/]+)$/, page: pageStageDetail, roles: ['admin'] },
    { re: /^\/admin\/stages\/([^/]+)\/lessons\/([^/]+)$/, page: pageLessonEditor, roles: ['admin'] },
    { re: /^\/admin\/grading$/, page: pageGrading, roles: ['admin'] },
    { re: /^\/admin\/grading\/([^/]+)$/, page: pageGradeSubmission, roles: ['admin'] },
    { re: /^\/admin\/courses$/, page: pageCourses, roles: ['admin'] },
    { re: /^\/admin\/courses\/([^/]+)$/, page: pageCourseDetail, roles: ['admin'] },
    { re: /^\/admin\/classes$/, page: pageClasses, roles: ['admin'] },
    { re: /^\/admin\/classes\/([^/]+)$/, page: pageClassDetail, roles: ['admin'] },
    { re: /^\/teach$/, page: pageTeachClasses, roles: ['teacher'] },
    { re: /^\/teach\/classes\/([^/]+)$/, page: pageTeachClass, roles: ['teacher'] },
    { re: /^\/teach\/grading$/, page: pageGrading, roles: ['teacher'] },
    { re: /^\/teach\/grading\/([^/]+)$/, page: pageGradeSubmission, roles: ['teacher'] },
    { re: /^\/learn$/, page: pageLearn, roles: ['student'] },
    { re: /^\/learn\/classes\/([^/]+)$/, page: pageLearnClass, roles: ['student'] },
    { re: /^\/learn\/classes\/([^/]+)\/summary$/, page: pageLearnSummary, roles: ['student'] },
    { re: /^\/learn\/classes\/([^/]+)\/lessons\/([^/]+)$/, page: pageLesson, roles: ['student'] }
  ];
  const NAV = {
    admin: [['#/admin', 'Tổng quan'], ['#/admin/stages', 'Chặng'], ['#/admin/courses', 'Khóa học'], ['#/admin/classes', 'Lớp học'], ['#/admin/grading', 'Chấm bài']],
    teacher: [['#/teach', 'Lớp của tôi'], ['#/teach/grading', 'Chấm bài']],
    student: [['#/learn', 'Lớp của tôi']]
  };

  function render() {
    closeModal(); closeDrawer(); clearInterval(quizTimer); quizTimer = null;
    sweepExpiredAttempts();
    const { path, query } = parseHash();
    if (query.as && user(query.as) && S.session?.userId !== query.as) { switchRole(query.as); } // prototype deep link: #/route?as=<userId>
    const u = me();
    let route = ROUTES.find((r) => r.re.test(path));
    if (!route) { location.hash = u ? homeFor(u) : '#/login'; return; }
    if (route.auth === false) { if (u) { location.hash = u.mustChangePassword ? '#/first-login' : homeFor(u); return; } }
    else {
      if (!u) { location.hash = '#/login'; return; }
      if (u.mustChangePassword && path !== '/first-login') { location.hash = '#/first-login'; return; }
      if (!u.mustChangePassword && path === '/first-login') { location.hash = homeFor(u); return; }
      if (route.roles && !route.roles.includes(u.role)) { location.hash = homeFor(u); return; }
    }
    const params = path.match(route.re).slice(1);
    const out = route.page(params, query);
    if (out === null) return; // page redirected
    const app = $('#app');
    app.innerHTML = (route.auth === false || path === '/first-login') ? `<main class="auth-main">${out.html}</main>` : shell(u, path, out.html);
    document.title = `${out.title} · GoUp LMS`;
    $('#announcer').textContent = out.title;
    window.scrollTo(0, 0);
    if (out.after) out.after();
  }
  function shell(u, path, content) {
    const pending = u.role === 'student' ? 0 : gradingQueue(u).length;
    const navItems = NAV[u.role].map(([href, label]) => { const cur = path === href.slice(1) || (href !== homeFor(u) && path.startsWith(href.slice(1) + '/'));
      const count = href.endsWith('/grading') && pending ? `<span class="nav-count">${pending}<span class="sr-only"> bài chờ chấm</span></span>` : '';
      return `<a href="${href}" ${cur ? 'aria-current="page"' : ''}>${label}${count}</a>`; }).join('');
    const roleOpts = [['u-admin', 'Admin · Trần Minh Quân'], ['u-gv', 'Giảng viên · Lê Thu Hương'], ['u-gv2', 'Giảng viên · Phạm Quốc Bảo'], ['u-an', 'Học viên · Nguyễn Hoàng An'], ['u-cuong', 'Học viên · Lê Văn Cường'], ['u-phong', 'Học viên · Đặng Hải Phong']]
      .filter(([id]) => user(id)).map(([id, l]) => `<option value="${id}" ${u.id === id ? 'selected' : ''}>${l}</option>`).join('');
    return `<header class="topbar"><div class="topbar-inner">
      <button class="icon-btn nav-toggle" data-action="toggle-nav" aria-label="Mở menu" aria-expanded="false">${icon('menu')}</button>
      <a class="brand" href="${homeFor(u)}">${brandMark}<span class="brand-name">GoUp<span>LMS</span></span></a>
      <nav class="nav" id="nav" aria-label="Chính">${navItems}</nav>
      <div class="topbar-end">
        <label class="role-switch"><span>Xem với vai trò</span><select data-change="switch-role" aria-label="Xem với vai trò (điều khiển của bản mẫu)">${roleOpts}</select></label>
        <div class="account"><span class="avatar" aria-hidden="true">${esc(initials(u.name))}</span><span class="account-name">${esc(u.name)}<small>${esc({ admin: 'Admin', teacher: 'Giảng viên', student: 'Học viên' }[u.role])}</small></span></div>
        <button class="icon-btn" style="color:#fff" data-action="reset-data" title="Đặt lại dữ liệu mẫu" aria-label="Đặt lại dữ liệu mẫu">${icon('rotate')}</button>
        <button class="icon-btn" style="color:#fff" data-action="logout" title="Đăng xuất" aria-label="Đăng xuất">${icon('log-out')}</button>
      </div></div></header>
      <main class="main" id="main">${content}</main>`;
  }
  const crumbs = (items) => `<nav class="crumbs" aria-label="Đường dẫn">${items.map(([label, href], i) => i === items.length - 1 ? `<span aria-current="page">${esc(label)}</span>` : (i ? `<span><a href="${href}">${esc(label)}</a></span>` : `<a href="${href}">${esc(label)}</a>`)).join('')}</nav>`;
  const pageHead = (title, { lede = '', actions = '', badges = '', crumbsHtml = '' } = {}) => `${crumbsHtml}<div class="page-head"><div><div class="title-row"><h1>${esc(title)}</h1>${badges}</div>${lede ? `<p class="lede">${lede}</p>` : ''}</div>${actions ? `<div class="actions">${actions}</div>` : ''}</div>`;

  /* ------------------------------------------------------ auth pages */
  function pageLogin() {
    const demo = [['quan.tran@goup.vn', 'Admin', 'Trần Minh Quân'], ['huong.le@goup.vn', 'Giảng viên · phụ trách basic01, basic03', 'Lê Thu Hương'], ['an.nguyen@gmail.com', 'Học viên · đang học basic01', 'Nguyễn Hoàng An'], ['minh.bui@gmail.com', 'Học viên · đăng nhập lần đầu bằng mật khẩu tạm', 'Bùi Quang Minh'], ['dung.pham@gmail.com', 'Học viên · mật khẩu tạm đã hết hạn', 'Phạm Minh Dũng'], ['thao.vo@gmail.com', 'Học viên · tài khoản đã vô hiệu hóa', 'Võ Phương Thảo']];
    return { title: 'Đăng nhập', html: `<div class="card auth-card">
      <div class="auth-brand">${brandMark}<span class="brand-name">GoUp LMS</span></div>
      <p class="auth-tagline">Nền tảng học nội bộ của GoUp: chặng → khóa học có phiên bản → lớp học → tiến độ học viên.</p>
      <h1>Đăng nhập</h1><p class="lede">Dùng email và mật khẩu trong thư mời của bạn.</p>
      <form data-form="login" class="stack-sm" novalidate>
        ${field('Email', '<input class="input" type="email" name="email" autocomplete="username" required>', { required: true })}
        ${field('Mật khẩu', '<input class="input" type="password" name="password" autocomplete="current-password" required>', { required: true })}
        <div class="alert alert-danger" data-form-error hidden>${icon('alert')}<div class="alert-body"></div></div>
        <div class="row-between" style="margin-top:8px"><a href="#/forgot" class="small">Quên mật khẩu?</a><button class="un-btn un-btn-primary" type="submit">Đăng nhập</button></div>
      </form>
      <div class="demo-accounts"><div class="section-label">Tài khoản mẫu của bản thử nghiệm</div>
        ${demo.map(([e, d, n]) => `<button type="button" data-action="demo-login" data-email="${e}"><span class="avatar">${initials(n)}</span><span class="who"><b>${esc(n)}</b><span>${esc(d)} · ${e}</span></span>${icon('arrow-right')}</button>`).join('')}
      </div>
      <p class="proto-note">Bản mẫu tĩnh: mọi mật khẩu đều được chấp nhận, không có email thật nào được gửi.</p>
    </div>` };
  }
  function pageForgot() {
    return { title: 'Quên mật khẩu', html: `<div class="card auth-card">
      <div class="auth-brand">${brandMark}<span class="brand-name">GoUp LMS</span></div>
      <h1>Quên mật khẩu</h1><p class="lede">Nhập email đăng nhập. Nếu email tồn tại, bạn sẽ nhận được đường dẫn đặt lại mật khẩu, hiệu lực 30 phút.</p>
      <form data-form="forgot" class="stack-sm" novalidate>
        ${field('Email', '<input class="input" type="email" name="email" required>', { required: true })}
        <div class="alert alert-ok" data-form-ok hidden>${icon('check-circle')}<div class="alert-body">Nếu email tồn tại trong hệ thống, chúng tôi đã gửi đường dẫn đặt lại mật khẩu. Vui lòng kiểm tra hộp thư.</div></div>
        <div class="row-between" style="margin-top:8px"><a href="#/login" class="small">Quay lại đăng nhập</a><button class="un-btn un-btn-primary" type="submit">Gửi đường dẫn</button></div>
      </form></div>` };
  }
  function pageFirstLogin() {
    const u = me(); const exp = u.tempPasswordExpiresAt ? `Mật khẩu tạm còn hiệu lực đến ${fmtDateTime(u.tempPasswordExpiresAt)}.` : '';
    return { title: 'Đặt mật khẩu mới', html: `<div class="card auth-card">
      <div class="auth-brand">${brandMark}<span class="brand-name">GoUp LMS</span></div>
      <h1>Đặt mật khẩu của bạn</h1><p class="lede">Xin chào ${esc(u.name)}. Trước khi vào lớp, hãy thay mật khẩu tạm bằng mật khẩu riêng của bạn. ${exp}</p>
      <form data-form="first-login" class="stack-sm" novalidate>
        ${field('Mật khẩu mới', '<input class="input" type="password" name="password" autocomplete="new-password" minlength="8" required>', { required: true, help: 'Tối thiểu 8 ký tự và khác mật khẩu tạm.' })}
        ${field('Nhập lại mật khẩu mới', '<input class="input" type="password" name="confirm" autocomplete="new-password" required>', { required: true })}
        <div class="alert alert-danger" data-form-error hidden>${icon('alert')}<div class="alert-body"></div></div>
        <div class="row-between" style="margin-top:8px"><button type="button" class="link-btn small" data-action="logout">Đăng xuất</button><button class="un-btn un-btn-primary" type="submit">Lưu mật khẩu</button></div>
      </form></div>` };
  }

  /* ----------------------------------------------------- admin pages */
  /* Guided walkthrough of the reference scenario (spec 7.3) for stakeholders opening the prototype cold. */
  function scenarioGuide() {
    const dbDraft = S.stageVersions.find((v) => v.stageId === 'st-db' && v.status === 'draft');
    const steps = [
      ['Nhân bản <b>Database v1</b> thành bản nháp v2.', dbDraft ? `#/admin/stages/st-db?v=${dbDraft.id}` : '#/admin/stages/st-db', dbDraft ? 'Mở bản nháp' : 'Mở chặng Database'],
      ['Sửa học liệu trong Database v2: thêm bài, đổi tên, bỏ bắt buộc.', null, null],
      ['Phát hành Database v2. Khóa <b>Lập trình cơ bản v1</b> sẽ bị cảnh báo dùng chặng cũ.', '#/admin', 'Xem cảnh báo trên Tổng quan'],
      ['Ở màn Database v2, chọn <b>Áp dụng cho khóa</b>: hệ thống tạo và phát hành Lập trình cơ bản v2 trong một thao tác.', '#/admin/courses/co-basic', 'Mở khóa học'],
      ['Gắn lớp nháp <b>basic03</b> sang Lập trình cơ bản v2; basic01 và basic02 giữ nguyên v1.', '#/admin/classes/cl-basic03?tab=settings', 'Mở cài đặt basic03'],
    ];
    return `<details class="card guide">
      <summary><span class="guide-title">${icon('compass')}Kịch bản tham chiếu (spec 7.3): sửa một bài trong chặng mà lớp đang chạy không bị ảnh hưởng</span><span class="guide-hint">5 bước · khoảng 3 phút</span></summary>
      <div class="guide-body">
        <ol class="guide-steps">${steps.map(([text, href, label]) => `<li><span>${text}</span>${href ? `<a class="un-btn un-btn-ghost un-btn-sm" href="${href}">${label}</a>` : ''}</li>`).join('')}</ol>
        <div class="guide-results"><span class="section-label">Kết quả mong đợi</span>
          <ul class="checks">
            <li>Học viên basic01, basic02 vẫn thấy Database v1 và tiến độ không đổi (<a href="#/learn/classes/cl-basic01?as=u-an">xem với vai trò học viên An</a>).</li>
            <li>Học viên basic03 thấy Database v2 khi lớp kích hoạt.</li>
            <li>Màn hình chặng Database không còn cảnh báo khóa dùng phiên bản cũ.</li>
          </ul>
          <p class="small muted">Muốn làm lại từ đầu: nút <b>Đặt lại dữ liệu mẫu</b> ở thanh trên.</p>
        </div>
      </div>
    </details>`;
  }

  /* Walkthrough of the homework, quiz and summary flows: each step opens the right screen as the right person. */
  function sprint2Guide() {
    const steps = [
      ['Học viên <b>An</b> làm quiz kiểm tra Go (15 phút, đồng hồ đếm ngược, tự lưu) và nộp bài tập Đọc CSV.', '#/learn/classes/cl-basic01?as=u-an', 'Mở lộ trình của An'],
      ['Giảng viên <b>Hương</b> mở hàng chờ chấm: bài quá 3 ngày nổi lên đầu. Chấm theo rubric, học viên nhận email kết quả.', '#/teach/grading?as=u-gv', 'Mở hàng chờ chấm'],
      ['<b>Cường</b> đã dùng hết 2 lượt quiz kiểm tra Go. Cấp thêm lượt kèm lý do; lý do vào lịch sử.', '#/teach/classes/cl-basic01?tab=quiz&as=u-gv', 'Mở thống kê quiz'],
      ['Đặt hoặc đổi hạn nộp bài tập cho riêng lớp basic01; thay đổi được ghi nhật ký.', '#/teach/classes/cl-basic01?tab=homework&as=u-gv', 'Mở bài tập của lớp'],
      ['Xem ma trận tổng kết học viên × chặng, lọc người cần làm lại, viết nhận xét chặng.', '#/teach/classes/cl-basic01?tab=summary&as=u-gv', 'Mở tổng kết lớp'],
      ['Chạy tác vụ email hằng ngày: giảng viên có bài quá hạn chấm nhận bản tổng hợp, học viên sắp hết hạn nộp nhận lời nhắc.', '#/admin?as=u-admin', 'Về Tổng quan']
    ];
    return `<details class="card guide">
      <summary><span class="guide-title">${icon('clipboard-check')}Kịch bản Sprint 2: quiz, bài tập, chấm bài và tổng kết chặng</span><span class="guide-hint">6 bước · khoảng 5 phút</span></summary>
      <div class="guide-body">
        <ol class="guide-steps">${steps.map(([text, href, label]) => `<li><span>${text}</span><a class="un-btn un-btn-ghost un-btn-sm" href="${href}">${label}</a></li>`).join('')}</ol>
        <p class="small muted">Các giá trị đánh dấu [đề xuất] trong bản mẫu là mặc định tạm cho câu hỏi còn mở của spec, cần chốt trước khi làm thật.</p>
      </div>
    </details>`;
  }
  function backlogCard() {
    const rows = teacherBacklog();
    return `<section class="card"><div class="card-head"><h2>Tồn đọng chấm bài</h2><a class="un-btn un-btn-ghost un-btn-sm" href="#/admin/grading">Hàng chờ chấm</a></div>
      <div class="table-wrap"><table><thead><tr><th>Giảng viên</th><th class="num">Chờ chấm</th><th class="num">Quá ${GRADING_SLA_DAYS} ngày</th><th class="num">TG chấm TB</th></tr></thead><tbody>
      ${rows.map((r) => `<tr><td>${esc(r.t.name)}</td><td class="num">${r.pending}</td><td class="num">${r.overdue ? `<span class="badge badge-danger">${r.overdue}</span>` : '0'}</td><td class="num">${r.avgH == null ? '—' : r.avgH < 24 ? `${Math.round(r.avgH)} giờ` : `${(r.avgH / 24).toFixed(1)} ngày`}</td></tr>`).join('')}
      </tbody></table></div></section>`;
  }
  function emailCard() {
    const list = S.emails.slice(0, 6);
    return `<section class="card"><div class="card-head"><h2>Email tự động <span class="muted small">(giả lập)</span></h2><button class="un-btn un-btn-secondary un-btn-sm" data-action="run-daily-jobs">${icon('send')} Chạy tác vụ hằng ngày</button></div>
      ${list.length ? `<ul class="list">${list.map((e) => `<li><span class="type-icon">${icon('mail')}</span><div class="grow"><div class="title">${esc(e.subject)}</div><div class="meta"><span class="badge badge-optional">${EMAIL_KIND[e.kind]}</span><span>${esc(e.to)}</span></div></div><span class="when">${rel(e.at)}</span></li>`).join('')}</ul>`
        : `<div class="card-body small muted">Chưa có email nào. Chấm một bài hoặc chạy tác vụ hằng ngày để xem email hệ thống gửi đi. Bản mẫu không gửi email thật.</div>`}</section>`;
  }

  function pageAdminDashboard() {
    const running = S.classes.filter((c) => c.status === 'active');
    const learners = new Set(running.flatMap((c) => activeMembersOf(c.id).map((m) => m.userId))).size;
    const failed = S.invitations.filter((i) => i.status === 'failed').length;
    const outdated = S.stages.flatMap((s) => outdatedCourses(s.id).map((o) => ({ ...o, stage: s })));
    const notLogged = S.members.filter((m) => m.status === 'active' && user(m.userId).status === 'invited').length;
    const lateGrading = gradingQueue(me()).filter((r) => r.overdue).length;
    const alerts = [
      ...outdated.map((o) => `<div class="alert alert-warn">${icon('alert')}<div class="alert-body"><strong>${esc(o.course.name)} ${versionLabel(o.cv)}</strong> vẫn dùng ${esc(o.stage.name)} ${versionLabel(o.used)} trong khi ${versionLabel(o.latest)} đã phát hành.<div class="alert-actions"><a class="un-btn un-btn-secondary un-btn-sm" href="#/admin/stages/${o.stage.id}?v=${o.latest.id}">Xem và áp dụng</a></div></div></div>`),
      ...(failed ? [`<div class="alert alert-danger">${icon('mail')}<div class="alert-body"><strong>${failed} lời mời gửi thất bại.</strong> Kiểm tra email học viên rồi gửi lại trong trang lớp.</div></div>`] : []),
      ...(lateGrading ? [`<div class="alert alert-danger">${icon('clock')}<div class="alert-body"><strong>${lateGrading} bài chờ chấm quá ${GRADING_SLA_DAYS} ngày.</strong> Học viên đang chờ phản hồi để làm tiếp.<div class="alert-actions"><a class="un-btn un-btn-secondary un-btn-sm" href="#/admin/grading?overdue=1">Xem bài quá hạn chấm</a></div></div></div>`] : [])
    ].join('');
    const audit = S.audit.slice().sort((a, b) => b.at.localeCompare(a.at)).slice(0, 8);
    return { title: 'Tổng quan', html: `${pageHead('Tổng quan', { lede: 'Tình trạng các lớp đang chạy và việc cần xử lý.' })}
      <div class="stack">
        ${alerts ? `<div class="stack-sm">${alerts}</div>` : ''}
        ${scenarioGuide()}
        ${sprint2Guide()}
        <div class="grid-4">
          <div class="card kpi"><div class="label">Lớp đang chạy</div><div class="value num">${running.length}</div><div class="hint">${S.classes.filter((c) => c.status === 'draft').length} lớp nháp</div></div>
          <div class="card kpi"><div class="label">Học viên đang học</div><div class="value num">${learners}</div><div class="hint">${notLogged} chưa đăng nhập lần đầu</div></div>
          <div class="card kpi"><div class="label">Lời mời thất bại</div><div class="value num">${failed}</div><div class="hint">Cần gửi lại</div></div>
          <div class="card kpi"><div class="label">Khóa học dùng chặng cũ</div><div class="value num">${outdated.length}</div><div class="hint">${outdated.length ? 'Có thể áp dụng một thao tác' : 'Mọi khóa học đã cập nhật'}</div></div>
        </div>
        <div class="grid-2">
          <section class="card"><div class="card-head"><h2>Lớp học</h2><a class="un-btn un-btn-ghost un-btn-sm" href="#/admin/classes">Tất cả lớp</a></div>
            <ul class="list">${S.classes.map((c) => `<li><div class="grow"><a class="title" href="#/admin/classes/${c.id}">${esc(c.code)} · ${esc(c.name)}</a><div class="meta"><span>${esc(course(cv(c.courseVersionId).courseId).name)} ${versionLabel(cv(c.courseVersionId))}</span><span>${activeMembersOf(c.id).length} học viên</span></div></div><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>${c.status !== 'draft' ? `<div style="width:120px">${progressBar(classAvgProgress(c.id))}</div>` : ''}</li>`).join('')}</ul></section>
          <section class="card"><div class="card-head"><h2>Nhật ký thao tác</h2></div>
            <ul class="list audit">${audit.map((a) => `<li><div class="grow"><div class="title">${esc(a.action)}</div><div class="meta">${esc(a.target)} · ${esc(user(a.by)?.name || '')}</div></div><span class="when">${rel(a.at)}</span></li>`).join('')}</ul></section>
        </div>
        <div class="grid-2">${backlogCard()}${emailCard()}</div>
      </div>` };
  }

  function pageStages() {
    const rows = S.stages.map((s) => {
      const vs = stageVersionsOf(s.id); const latest = latestPublished(vs); const used = latest ? cvsUsingSV(latest.id).length : 0; const outdated = outdatedCourses(s.id).length;
      const courses = new Set(vs.flatMap((v) => cvsUsingSV(v.id).map((c) => c.courseId))).size;
      return `<tr class="is-clickable" data-href="#/admin/stages/${s.id}"><td><div class="cell-2"><a class="primary" href="#/admin/stages/${s.id}">${esc(s.name)}</a><span class="secondary">${esc(s.code)}</span></div></td>
        <td>${lineage(vs, (v) => `#/admin/stages/${s.id}?v=${v.id}`)}</td>
        <td class="num">${courses}</td>
        <td>${outdated ? `<span class="badge badge-warn">${icon('alert')} ${outdated} khóa học dùng bản cũ</span>` : '<span class="muted">—</span>'}</td></tr>`;
    }).join('');
    return { title: 'Chặng', html: `${pageHead('Chặng', { lede: 'Chặng dùng chung giữa các khóa học. Sửa học liệu bằng cách nhân bản ra bản nháp mới; phiên bản đã phát hành không đổi.', actions: `<button class="un-btn un-btn-primary" data-action="new-stage">${icon('plus')} Tạo chặng</button>` })}
      <div class="card"><div class="table-wrap"><table><thead><tr><th>Chặng</th><th>Phiên bản</th><th class="num">Khóa học dùng</th><th>Cảnh báo</th></tr></thead><tbody>${rows || `<tr><td colspan="4">${empty('Chưa có chặng', 'Tạo chặng đầu tiên để bắt đầu soạn học liệu.')}</td></tr>`}</tbody></table></div></div>` };
  }

  function pageStageDetail([stageId], query) {
    const s = stage(stageId); if (!s) { location.hash = '#/admin/stages'; return null; }
    const vs = stageVersionsOf(s.id); const v = sv(query.v) && sv(query.v).stageId === s.id ? sv(query.v) : (latestPublished(vs) || vs[vs.length - 1]);
    const href = (x) => `#/admin/stages/${s.id}?v=${x.id}`;
    const isDraft = v.status === 'draft'; const latest = latestPublished(vs); const draft = draftOf(vs);
    const refs = cvsUsingSV(v.id); const classes = refs.flatMap((c) => classesUsingCV(c.id));
    const outdated = latest && v.id === latest.id ? outdatedCourses(s.id) : [];
    const deleteBlock = v.status !== 'draft' && refs.length ? `Đang dùng trong ${refs.map((c) => `${course(c.courseId).name} ${versionLabel(c)}`).join(', ')}` : '';
    const blockers = isDraft ? stagePublishBlockers(v) : [];
    const actions = isDraft
      ? `<button class="un-btn un-btn-secondary" data-action="delete-sv" data-id="${v.id}">${icon('trash')} Xóa bản nháp</button><button class="un-btn un-btn-primary" data-action="publish-sv" data-id="${v.id}" ${blockers.length ? `aria-disabled="true" title="${esc(blockers[0])}"` : ''}>${icon('check')} Phát hành ${versionLabel(v)}</button>`
      : `${v.status === 'published' ? `<button class="un-btn un-btn-secondary" data-action="archive-sv" data-id="${v.id}">${icon('archive')} Lưu trữ</button>` : ''}<button class="un-btn un-btn-secondary" data-action="delete-sv" data-id="${v.id}" ${deleteBlock ? `aria-disabled="true" title="${esc(deleteBlock)}"` : ''}>${icon('trash')} Xóa</button>${draft ? `<a class="un-btn un-btn-navy" href="${href(draft)}">${icon('pencil')} Mở bản nháp ${versionLabel(draft)}</a>` : `<button class="un-btn un-btn-navy" data-action="clone-sv" data-id="${v.id}">${icon('copy')} Nhân bản thành bản nháp</button>`}`;
    const lessons = v.lessons.length ? v.lessons.map((l, i) => { const graded = l.type === 'quiz' || l.type === 'homework'; const lb = isDraft ? lessonBlockers(l) : [];
      const edHref = `#/admin/stages/${s.id}/lessons/${l.id}?v=${v.id}`;
      return `<li>
        <span class="ord">${i + 1}</span><span class="type-icon" title="${TYPE_VI[l.type]}">${typeIcon(l)}</span>
        <div class="grow"><div class="title">${graded ? `<a href="${edHref}">${esc(l.title)}</a>` : esc(l.title)}</div><div class="meta"><span>${lessonMeta(l)}</span>${l.required ? '' : '<span class="badge badge-optional">Không bắt buộc</span>'}${lb.length ? `<span class="badge badge-warn">${icon('alert')} Chưa đủ để phát hành</span>` : ''}<span class="muted">key ${esc(l.key)}</span></div></div>
        ${graded ? `<a class="un-btn un-btn-ghost un-btn-sm" href="${edHref}">${isDraft ? (l.type === 'quiz' ? 'Soạn câu hỏi' : 'Soạn tiêu chí') : 'Xem chi tiết'}</a>` : ''}
        ${isDraft ? `<div class="inline-actions"><button class="icon-btn" data-action="move-lesson" data-sv="${v.id}" data-id="${l.id}" data-dir="-1" aria-label="Chuyển lên" ${i === 0 ? 'disabled' : ''}>${icon('chevron-up')}</button><button class="icon-btn" data-action="move-lesson" data-sv="${v.id}" data-id="${l.id}" data-dir="1" aria-label="Chuyển xuống" ${i === v.lessons.length - 1 ? 'disabled' : ''}>${icon('chevron-down')}</button><button class="icon-btn" data-action="edit-lesson" data-sv="${v.id}" data-id="${l.id}" aria-label="Sửa">${icon('pencil')}</button><button class="icon-btn" data-action="delete-lesson" data-sv="${v.id}" data-id="${l.id}" aria-label="Xóa">${icon('trash')}</button></div>` : ''}
      </li>`; }).join('') : `<li>${empty('Chưa có học liệu', isDraft ? 'Thêm video, bài đọc, quiz hoặc bài tập để phát hành chặng này.' : 'Phiên bản này không có học liệu.')}</li>`;
    const blockersHtml = blockers.length ? `<div class="alert alert-warn">${icon('alert')}<div class="alert-body"><strong>Chưa phát hành được ${versionLabel(v)}:</strong><ul class="alert-list">${blockers.map((b) => `<li>${esc(b)}</li>`).join('')}</ul></div></div>` : '';
    const outdatedHtml = outdated.length ? `<section class="card"><div class="card-head"><h2>Khóa học đang dùng phiên bản cũ của chặng này</h2></div><ul class="list">${outdated.map((o) => `<li><div class="grow"><a class="title" href="#/admin/courses/${o.course.id}?v=${o.cv.id}">${esc(o.course.name)} ${versionLabel(o.cv)}</a><div class="meta"><span>Đang dùng ${esc(s.name)} ${versionLabel(o.used)}</span>${o.blocked ? `<span class="badge badge-warn">${esc(o.blocked)}</span>` : ''}</div></div><button class="un-btn un-btn-primary un-btn-sm" data-action="apply-sv" data-sv="${v.id}" data-course="${o.course.id}" ${o.blocked ? 'aria-disabled="true"' : ''}>Áp dụng ${versionLabel(v)}</button></li>`).join('')}</ul></section>` : (latest && v.id === latest.id && S.courses.length ? `<div class="alert alert-ok">${icon('check-circle')}<div class="alert-body">Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này.</div></div>` : '');
    const usedHtml = `<section class="card"><div class="card-head"><h2>Đang được dùng ở</h2></div>${refs.length ? `<ul class="list">${refs.map((c) => `<li><div class="grow"><a class="title" href="#/admin/courses/${c.courseId}?v=${c.id}">${esc(course(c.courseId).name)} ${versionLabel(c)}</a><div class="meta">${badge(c.status)}<span>${classesUsingCV(c.id).map((x) => x.code).join(', ') || 'chưa có lớp'}</span></div></div></li>`).join('')}</ul>` : `<div class="card-body muted small">Chưa có khóa học nào dùng ${versionLabel(v)}.${classes.length ? '' : ''}</div>`}</section>`;
    return { title: `${s.name} ${versionLabel(v)}`, html: `${pageHead(s.name, { crumbsHtml: crumbs([['Chặng', '#/admin/stages'], [s.name]]), badges: `<span class="muted">${esc(s.code)}</span>`, lede: 'Phiên bản đã phát hành là bất biến; mọi thay đổi đi qua một bản nháp mới rồi được áp dụng cho khóa học bằng một thao tác.' })}
      <div class="stack">
        <div class="card card-pad row-between"><div class="row"><span class="section-label">Phiên bản</span>${lineage(vs, href, v.id)}</div><div class="row small muted">${v.status === 'published' ? `Phát hành ${fmtDateTime(v.publishedAt)}` : v.status === 'draft' ? (v.clonedFrom ? `Nhân bản từ ${versionLabel(sv(v.clonedFrom))}` : 'Bản nháp đầu tiên') : 'Đã lưu trữ'}</div></div>
        ${blockersHtml}
        <section class="card"><div class="card-head"><div class="row"><h2>Học liệu của ${versionLabel(v)}</h2>${badge(v.status)}</div><div class="row">${isDraft ? `<button class="un-btn un-btn-secondary un-btn-sm" data-action="new-lesson" data-sv="${v.id}">${icon('plus')} Thêm học liệu</button>` : `<span class="small muted">${icon('lock')} Không sửa được</span>`}</div></div><ul class="list">${lessons}</ul>
          <div class="complete-bar"><span class="small muted">${v.lessons.filter((l) => l.required).length} bắt buộc · ${v.lessons.filter((l) => !l.required).length} tùy chọn</span><div class="row">${actions}</div></div></section>
        ${outdatedHtml}
        ${usedHtml}
      </div>` };
  }

  function pageLessonEditor([stageId, lessonId]) {
    const v = svOfLesson(lessonId); const s = stage(stageId);
    if (!v || !s || v.stageId !== s.id) { location.hash = s ? `#/admin/stages/${s.id}` : '#/admin/stages'; return null; }
    const l = v.lessons.find((x) => x.id === lessonId);
    if (l.type !== 'quiz' && l.type !== 'homework') { location.hash = `#/admin/stages/${s.id}?v=${v.id}`; return null; }
    const isDraft = v.status === 'draft'; const blockers = isDraft ? lessonBlockers(l) : [];
    const data = `data-sv="${v.id}" data-lesson="${l.id}"`;
    const moveBtns = (kind, id, i, n) => `<button class="icon-btn" data-action="move-${kind}" ${data} data-id="${id}" data-dir="-1" aria-label="Chuyển lên" ${i === 0 ? 'disabled' : ''}>${icon('chevron-up')}</button><button class="icon-btn" data-action="move-${kind}" ${data} data-id="${id}" data-dir="1" aria-label="Chuyển xuống" ${i === n - 1 ? 'disabled' : ''}>${icon('chevron-down')}</button>`;
    const fact = (k, val) => `<div><dt>${k}</dt><dd>${val}</dd></div>`;
    let settings, body;
    if (l.type === 'quiz') {
      const z = l.quiz; const total = z.questions.reduce((a, x) => a + x.points, 0);
      settings = `<dl class="facts">${fact('Chế độ', z.mode === 'assessment' ? 'Kiểm tra (tính vào tiến độ)' : 'Luyện tập')}${fact('Ngưỡng đạt', `${z.threshold}%`)}${fact('Số lượt', z.attempts == null ? 'Không giới hạn' : `${z.attempts} lượt`)}${fact('Thời gian', z.timeLimit ? `${z.timeLimit} phút` : 'Không giới hạn')}${fact('Xáo trộn', z.shuffle ? 'Câu hỏi và phương án' : 'Giữ thứ tự soạn')}${fact('Cách chấm', 'Đúng toàn bộ phương án mới được điểm câu')}</dl>
        ${z.mode === 'assessment' ? `<p class="small muted">Sau khi nộp, học viên chỉ thấy điểm và kết quả Đạt hoặc Chưa đạt, không thấy đáp án <span class="badge badge-optional">đề xuất</span>. Lượt tốt nhất được tính.</p>` : '<p class="small muted">Sau khi nộp, học viên thấy đáp án đúng và giải thích từng câu.</p>'}`;
      body = `<section class="card"><div class="card-head"><h2>Câu hỏi <span class="muted small">${z.questions.length} câu · ${total} điểm</span></h2>${isDraft ? `<button class="un-btn un-btn-secondary un-btn-sm" data-action="new-question" ${data}>${icon('plus')} Thêm câu hỏi</button>` : ''}</div>
        ${z.questions.length ? `<ol class="list q-list">${z.questions.map((x, i) => `<li><span class="ord">${i + 1}</span><div class="grow stack-sm">
            <div class="title">${mdInline(x.text)}</div>
            <div class="meta"><span>${x.type === 'multi' ? 'Nhiều đáp án' : 'Một đáp án'}</span><span>${x.points} điểm</span>${x.options.some((o) => o.correct) ? '' : '<span class="badge badge-warn">Chưa có đáp án đúng</span>'}</div>
            <ul class="opt-list">${x.options.map((o) => `<li class="${o.correct ? 'is-correct' : ''}">${o.correct ? `${icon('check-circle')}<span class="sr-only">Đáp án đúng: </span>` : '<span class="opt-dot" aria-hidden="true"></span>'}<span>${mdInline(o.text)}</span></li>`).join('')}</ul>
            ${x.explanation ? `<p class="small muted">${icon('info')} ${mdInline(x.explanation)}</p>` : ''}</div>
            ${isDraft ? `<div class="inline-actions">${moveBtns('question', x.id, i, z.questions.length)}<button class="icon-btn" data-action="edit-question" ${data} data-id="${x.id}" aria-label="Sửa câu ${i + 1}">${icon('pencil')}</button><button class="icon-btn" data-action="delete-question" ${data} data-id="${x.id}" aria-label="Xóa câu ${i + 1}">${icon('trash')}</button></div>` : ''}</li>`).join('')}</ol>`
          : empty('Chưa có câu hỏi', 'Quiz cần ít nhất một câu hỏi có đáp án đúng trước khi phát hành chặng.', isDraft ? `<button class="un-btn un-btn-primary un-btn-sm" data-action="new-question" ${data}>${icon('plus')} Thêm câu hỏi đầu tiên</button>` : '')}</section>`;
    } else {
      const hw = l.homework; const max = hwMax(hw);
      settings = `<dl class="facts">${fact('Ngưỡng đạt', `${hw.threshold}% tổng điểm${max ? ` (từ ${Math.ceil(hw.threshold * max / 100)}/${max})` : ''}`)}${fact('Cách nộp', 'Đường dẫn repo và/hoặc file')}${fact('Giới hạn file', `${MAX_FILES} file, mỗi file ≤ ${MAX_FILE_MB} MB <span class="badge badge-optional">đề xuất</span>`)}${fact('Định dạng', FILE_EXTS.join(', '))}</dl>
        <p class="small muted">Hạn nộp đặt riêng cho từng lớp trong trang lớp, không đặt ở đây.</p>`;
      body = `<section class="card"><div class="card-head"><h2>Đề bài</h2></div><div class="card-body">${hw.brief.trim() ? `<div class="md">${renderMarkdown(hw.brief)}</div>` : '<p class="muted small">Chưa có đề bài. Bấm Sửa thiết lập để nhập.</p>'}
          ${hw.attachments.length ? `<div class="section-label" style="margin-top:var(--sp-4)">File đính kèm</div><div class="row" style="margin-top:var(--sp-2)">${hw.attachments.map((n) => `<button class="file-chip" data-action="download-file" data-name="${esc(n)}">${icon('paperclip')}<span>${esc(n)}</span></button>`).join('')}</div>` : ''}</div></section>
        <section class="card"><div class="card-head"><h2>Tiêu chí chấm <span class="muted small">${hw.criteria.length} tiêu chí · ${max} điểm</span></h2>${isDraft ? `<button class="un-btn un-btn-secondary un-btn-sm" data-action="new-criterion" ${data}>${icon('plus')} Thêm tiêu chí</button>` : ''}</div>
          ${hw.criteria.length ? `<div class="table-wrap"><table><thead><tr><th>#</th><th>Tiêu chí</th><th class="num">Điểm tối đa</th>${isDraft ? '<th class="cell-actions">Thao tác</th>' : ''}</tr></thead><tbody>${hw.criteria.map((c, i) => `<tr><td class="num">${i + 1}</td><td>${esc(c.text)}</td><td class="num">${c.max}</td>${isDraft ? `<td class="cell-actions"><div class="inline-actions">${moveBtns('criterion', c.id, i, hw.criteria.length)}<button class="icon-btn" data-action="edit-criterion" ${data} data-id="${c.id}" aria-label="Sửa tiêu chí ${i + 1}">${icon('pencil')}</button><button class="icon-btn" data-action="delete-criterion" ${data} data-id="${c.id}" aria-label="Xóa tiêu chí ${i + 1}">${icon('trash')}</button></div></td>` : ''}</tr>`).join('')}</tbody>
            <tfoot><tr><th></th><th>Tổng</th><th class="num">${max}</th>${isDraft ? '<th></th>' : ''}</tr></tfoot></table></div>`
          : empty('Chưa có tiêu chí', 'Bài tập cần ít nhất một tiêu chí chấm trước khi phát hành chặng.', isDraft ? `<button class="un-btn un-btn-primary un-btn-sm" data-action="new-criterion" ${data}>${icon('plus')} Thêm tiêu chí đầu tiên</button>` : '')}</section>`;
    }
    return { title: l.title, html: `${pageHead(l.title, { crumbsHtml: crumbs([['Chặng', '#/admin/stages'], [`${s.name} ${versionLabel(v)}`, `#/admin/stages/${s.id}?v=${v.id}`], [l.title]]),
        badges: `<span class="badge badge-optional">${TYPE_VI[l.type]}</span>${badge(v.status)}${l.required ? '' : '<span class="badge badge-optional">Không bắt buộc</span>'}`,
        lede: lessonMeta(l), actions: isDraft ? `<button class="un-btn un-btn-secondary" data-action="edit-lesson" data-sv="${v.id}" data-id="${l.id}">${icon('pencil')} Sửa thiết lập</button>` : '' })}
      <div class="stack">
        ${isDraft ? '' : `<div class="alert alert-info">${icon('lock')}<div class="alert-body">${versionLabel(v)} ${STATUS_VI[v.status].toLowerCase()} nên không sửa được. Nhân bản chặng thành bản nháp mới để sửa; câu hỏi và tiêu chí giữ nguyên key.</div></div>`}
        ${blockers.length ? `<div class="alert alert-warn">${icon('alert')}<div class="alert-body"><strong>Chưa đủ để phát hành:</strong><ul class="alert-list">${blockers.map((b) => `<li>${esc(b)}</li>`).join('')}</ul></div></div>` : ''}
        <section class="card"><div class="card-head"><h2>Thiết lập</h2></div><div class="card-body stack-sm">${settings}</div></section>
        ${body}
      </div>` };
  }
  function questionForm(x = {}) {
    const opts = x.options || []; const rows = Array.from({ length: 6 }, (_, i) => { const o = opts[i] || {};
      return `<div class="opt-edit-row"><label class="check"><input type="checkbox" name="correct${i}" ${o.correct ? 'checked' : ''} aria-label="Phương án ${i + 1} là đáp án đúng"><span class="small">Đúng</span></label><input class="input" name="opt${i}" value="${esc(o.text || '')}" aria-label="Phương án ${i + 1}" placeholder="Phương án ${i + 1}${i < 2 ? '' : ' (không bắt buộc)'}"><input type="hidden" name="oid${i}" value="${esc(o.id || '')}"></div>`; }).join('');
    return `<div class="stack-sm">
      ${field('Nội dung câu hỏi', `<textarea class="input input-prose" name="text" rows="3" required>${esc(x.text || '')}</textarea>`, { required: true, help: 'Hỗ trợ **đậm** và `mã`.' })}
      <div class="form-grid">${field('Loại câu', `<select class="input" name="type">${selectOpts([{ value: 'single', label: 'Một đáp án' }, { value: 'multi', label: 'Nhiều đáp án' }], x.type || 'single')}</select>`)}${field('Điểm', `<input class="input" type="number" name="points" min="1" step="1" value="${x.points || 1}">`, { required: true })}</div>
      <fieldset class="opt-edit"><legend>Phương án <span class="muted small">tối thiểu 2, đánh dấu đáp án đúng</span></legend>${rows}</fieldset>
      ${field('Giải thích', `<textarea class="input input-prose" name="explanation" rows="2">${esc(x.explanation || '')}</textarea>`, { help: 'Hiện cho học viên sau khi nộp quiz luyện tập.' })}
    </div>`;
  }
  const criterionForm = (c = {}) => `<div class="form-grid"><div class="span-2">${field('Tiêu chí', `<input class="input" name="text" value="${esc(c.text || '')}" placeholder="vd: Chạy đúng yêu cầu" required>`, { required: true })}</div>${field('Điểm tối đa', `<input class="input" type="number" name="max" min="1" max="100" step="1" value="${c.max || ''}" required>`, { required: true, help: 'Số nguyên từ 1 đến 100.' })}</div>`;

  function pageCourses() {
    const rows = S.courses.map((c) => { const vs = courseVersionsOf(c.id); const latest = latestPublished(vs); const classes = vs.flatMap((v) => classesUsingCV(v.id));
      return `<tr class="is-clickable" data-href="#/admin/courses/${c.id}"><td><div class="cell-2"><a class="primary" href="#/admin/courses/${c.id}">${esc(c.name)}</a><span class="secondary">${esc(c.code)}</span></div></td><td>${lineage(vs, (v) => `#/admin/courses/${c.id}?v=${v.id}`)}</td><td class="num">${latest ? latest.stages.length : '—'}</td><td>${classes.length ? classes.map((x) => `${x.code} (${versionLabel(cv(x.courseVersionId))})`).join(', ') : '<span class="muted">—</span>'}</td></tr>`; }).join('');
    return { title: 'Khóa học', html: `${pageHead('Khóa học', { lede: 'Khóa học là một chuỗi phiên bản chặng có thứ tự. Lớp gắn với đúng một phiên bản khóa học đã phát hành.', actions: `<button class="un-btn un-btn-primary" data-action="new-course">${icon('plus')} Tạo khóa học</button>` })}
      <div class="card"><div class="table-wrap"><table><thead><tr><th>Khóa học</th><th>Phiên bản</th><th class="num">Số chặng</th><th>Lớp đang dùng</th></tr></thead><tbody>${rows || `<tr><td colspan="4">${empty('Chưa có khóa học', 'Tạo khóa học rồi ghép các chặng đã phát hành.')}</td></tr>`}</tbody></table></div></div>` };
  }

  function pageCourseDetail([courseId], query) {
    const co = course(courseId); if (!co) { location.hash = '#/admin/courses'; return null; }
    const vs = courseVersionsOf(co.id); const v = cv(query.v) && cv(query.v).courseId === co.id ? cv(query.v) : (latestPublished(vs) || vs[vs.length - 1]);
    const href = (x) => `#/admin/courses/${co.id}?v=${x.id}`; const isDraft = v.status === 'draft'; const draft = draftOf(vs);
    const classes = classesUsingCV(v.id); const blocker = isDraft ? coursePublishBlocker(v) : null;
    const deleteBlock = v.status !== 'draft' && classes.length ? `Đang dùng bởi lớp ${classes.map((c) => c.code).join(', ')}` : '';
    const actions = isDraft
      ? `<button class="un-btn un-btn-secondary" data-action="delete-cv" data-id="${v.id}">${icon('trash')} Xóa bản nháp</button><button class="un-btn un-btn-primary" data-action="publish-cv" data-id="${v.id}" ${blocker ? `aria-disabled="true" title="${esc(blocker)}"` : ''}>${icon('check')} Phát hành ${versionLabel(v)}</button>`
      : `${v.status === 'published' ? `<button class="un-btn un-btn-secondary" data-action="archive-cv" data-id="${v.id}">${icon('archive')} Lưu trữ</button>` : ''}<button class="un-btn un-btn-secondary" data-action="delete-cv" data-id="${v.id}" ${deleteBlock ? `aria-disabled="true" title="${esc(deleteBlock)}"` : ''}>${icon('trash')} Xóa</button>${draft ? `<a class="un-btn un-btn-navy" href="${href(draft)}">${icon('pencil')} Mở bản nháp ${versionLabel(draft)}</a>` : `<button class="un-btn un-btn-navy" data-action="clone-cv" data-id="${v.id}">${icon('copy')} Nhân bản thành bản nháp</button>`}`;
    const stagesHtml = v.stages.length ? v.stages.map((svId, i) => { const s = sv(svId); const st = stage(s.stageId); const newer = latestPublished(stageVersionsOf(st.id)); const stale = newer && newer.no > s.no;
      return `<li><span class="ord">${i + 1}</span><div class="grow"><a class="title" href="#/admin/stages/${st.id}?v=${s.id}">${esc(st.name)}</a><div class="meta"><span class="vpill is-${s.status}" style="height:22px;font-size:11px">${versionLabel(s)}</span><span>${s.lessons.length} học liệu · ${s.lessons.filter((l) => l.required).length} bắt buộc</span>${stale ? `<span class="badge badge-warn">${icon('alert')} ${versionLabel(newer)} đã phát hành</span>` : ''}</div></div>
        ${isDraft ? `<div class="inline-actions">${stale ? `<button class="un-btn un-btn-ghost un-btn-sm" data-action="swap-stage" data-cv="${v.id}" data-old="${s.id}" data-new="${newer.id}">Dùng ${versionLabel(newer)}</button>` : ''}<button class="icon-btn" data-action="move-stage" data-cv="${v.id}" data-id="${s.id}" data-dir="-1" aria-label="Chuyển lên" ${i === 0 ? 'disabled' : ''}>${icon('chevron-up')}</button><button class="icon-btn" data-action="move-stage" data-cv="${v.id}" data-id="${s.id}" data-dir="1" aria-label="Chuyển xuống" ${i === v.stages.length - 1 ? 'disabled' : ''}>${icon('chevron-down')}</button><button class="icon-btn" data-action="remove-stage" data-cv="${v.id}" data-id="${s.id}" aria-label="Gỡ khỏi khóa học">${icon('x')}</button></div>` : ''}</li>`; }).join('')
      : `<li>${empty('Chưa có chặng', isDraft ? 'Thêm các chặng đã phát hành theo thứ tự học.' : 'Phiên bản này không có chặng.')}</li>`;
    return { title: `${co.name} ${versionLabel(v)}`, html: `${pageHead(co.name, { crumbsHtml: crumbs([['Khóa học', '#/admin/courses'], [co.name]]), badges: `<span class="muted">${esc(co.code)}</span>`, lede: 'Nhân bản khóa học là nhân bản nông: phiên bản mới trỏ lại đúng các phiên bản chặng cũ cho đến khi bạn đổi.' })}
      <div class="stack">
        <div class="card card-pad row-between"><div class="row"><span class="section-label">Phiên bản</span>${lineage(vs, href, v.id)}</div><div class="row small muted">${v.status === 'published' ? `Phát hành ${fmtDateTime(v.publishedAt)}` : v.status === 'draft' ? (v.clonedFrom ? `Nhân bản từ ${versionLabel(cv(v.clonedFrom))}` : 'Bản nháp đầu tiên') : 'Đã lưu trữ'}</div></div>
        ${blocker ? `<div class="alert alert-warn">${icon('alert')}<div class="alert-body">Chưa phát hành được: ${esc(blocker)}</div></div>` : ''}
        <section class="card"><div class="card-head"><div class="row"><h2>Chặng trong ${versionLabel(v)}</h2>${badge(v.status)}</div>${isDraft ? `<button class="un-btn un-btn-secondary un-btn-sm" data-action="add-stage" data-cv="${v.id}">${icon('plus')} Thêm chặng</button>` : `<span class="small muted">${icon('lock')} Không sửa được</span>`}</div><ul class="list">${stagesHtml}</ul>
          <div class="complete-bar"><span class="small muted">${v.stages.reduce((a, id) => a + sv(id).lessons.filter((l) => l.required).length, 0)} học liệu bắt buộc toàn khóa</span><div class="row">${actions}</div></div></section>
        <section class="card"><div class="card-head"><h2>Lớp gắn với ${versionLabel(v)}</h2></div>${classes.length ? `<ul class="list">${classes.map((c) => `<li><div class="grow"><a class="title" href="#/admin/classes/${c.id}">${esc(c.code)} · ${esc(c.name)}</a><div class="meta"><span>${activeMembersOf(c.id).length} học viên</span></div></div><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span></li>`).join('')}</ul>` : `<div class="card-body muted small">Chưa có lớp nào gắn với ${versionLabel(v)}.</div>`}</section>
      </div>` };
  }

  function pageClasses() {
    const rows = S.classes.map((c) => { const v = cv(c.courseVersionId); const n = activeMembersOf(c.id).length;
      return `<tr class="is-clickable" data-href="#/admin/classes/${c.id}"><td><div class="cell-2"><a class="primary" href="#/admin/classes/${c.id}">${esc(c.code)}</a><span class="secondary">${esc(c.name)}</span></div></td><td>${esc(course(v.courseId).name)} <span class="vpill is-${v.status}" style="height:22px;font-size:11px">${versionLabel(v)}</span></td><td><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span></td><td>${esc(user(c.teacherId)?.name || '—')}</td><td class="num">${n}</td><td>${c.status === 'draft' ? '<span class="muted">—</span>' : progressBar(classAvgProgress(c.id))}</td><td class="num">${fmtDate(c.startDate)}</td></tr>`; }).join('');
    return { title: 'Lớp học', html: `${pageHead('Lớp học', { lede: 'Mỗi lớp chạy trọn đời trên một phiên bản khóa học. Đổi phiên bản chỉ khi lớp còn ở trạng thái nháp.', actions: `<button class="un-btn un-btn-primary" data-action="new-class">${icon('plus')} Tạo lớp</button>` })}
      <div class="card"><div class="table-wrap"><table><thead><tr><th>Lớp</th><th>Khóa học</th><th>Trạng thái</th><th>Giảng viên</th><th class="num">Học viên</th><th>Tiến độ TB</th><th class="num">Bắt đầu</th></tr></thead><tbody>${rows}</tbody></table></div></div>` };
  }

  function pageClassDetail([classId], query) {
    const c = cls(classId); if (!c) { location.hash = '#/admin/classes'; return null; }
    const keys = ['students', 'report', 'summary', 'homework', 'quiz', 'settings'];
    const tab = keys.includes(query.tab) ? query.tab : 'students';
    const v = cv(c.courseVersionId); const base = `#/admin/classes/${c.id}`;
    const lifecycle = c.status === 'draft' ? `<button class="un-btn un-btn-primary" data-action="activate-class" data-id="${c.id}">${icon('play')} Kích hoạt lớp</button>` : c.status === 'active' ? `<button class="un-btn un-btn-secondary" data-action="end-class" data-id="${c.id}">Kết thúc lớp</button>` : '';
    const body = tab === 'students' ? renderStudentsTab(c) : tab === 'settings' ? renderSettingsTab(c) : classTabBody(c, tab, query, base);
    return { title: c.code, html: `${pageHead(`${c.code} · ${c.name}`, { crumbsHtml: crumbs([['Lớp học', '#/admin/classes'], [c.code]]), badges: `<span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>`, lede: `${esc(course(v.courseId).name)} ${versionLabel(v)} · ${fmtDate(c.startDate)} → ${fmtDate(c.endDate)} · Giảng viên ${esc(user(c.teacherId)?.name || '—')}`, actions: lifecycle })}${classTabs(c, base, keys, tab)}${body}` };
  }
  const CLASS_TABS = { students: 'Học viên', report: 'Tiến độ', summary: 'Tổng kết', homework: 'Bài tập', quiz: 'Quiz', settings: 'Cài đặt' };
  function classTabs(c, base, keys, tab) {
    const waiting = gradingQueue(me(), { classId: c.id }).length;
    const count = (k) => k === 'students' ? `<span class="count">${activeMembersOf(c.id).length}</span>` : k === 'homework' && waiting ? `<span class="count">${waiting}<span class="sr-only"> bài chờ chấm</span></span>` : '';
    return `<nav class="tabs" aria-label="Mục của lớp">${keys.map((k) => `<a href="${base}?tab=${k}" ${tab === k ? 'aria-current="page"' : ''}>${CLASS_TABS[k]}${count(k)}</a>`).join('')}</nav>`;
  }
  // Tabs shared by the admin and teacher views of a class; the teacher sees the same data for their own classes.
  function classTabBody(c, tab, query, base) {
    if (tab === 'summary') return renderSummaryTab(c, query, `${base}?tab=summary`);
    if (tab === 'homework') return renderHomeworkTab(c);
    if (tab === 'quiz') return renderQuizTab(c);
    return renderReport(c, query, `${base}?tab=report`);
  }

  /* ------------------------------------------- class summary, homework, quiz */
  function renderSummaryTab(c, query, basePath) {
    const v = cv(c.courseVersionId); const cols = v.stages.map((id) => ({ sv: sv(id), st: stage(sv(id).stageId) }));
    const f = { incomplete: query.incomplete || '', rework: query.rework === '1', overdue: query.overdue === '1', nocomment: query.nocomment || '' };
    let rows = activeMembersOf(c.id).map((m) => ({ m, u: user(m.userId), sums: courseSummary(m), p: memberProgress(m) })).sort((a, b) => a.u.name.localeCompare(b.u.name, 'vi'));
    const total = rows.length;
    if (f.incomplete) rows = rows.filter((r) => !r.sums.find((x) => x.sv.id === f.incomplete)?.complete);
    if (f.rework) rows = rows.filter((r) => r.sums.some((x) => x.rework));
    if (f.overdue) rows = rows.filter((r) => r.sums.some((x) => x.overdue));
    if (f.nocomment) rows = rows.filter((r) => !r.sums.find((x) => x.sv.id === f.nocomment)?.review);
    const anyFilter = f.incomplete || f.rework || f.overdue || f.nocomment;
    const stageOpts = [{ value: '', label: 'Tất cả chặng' }, ...cols.map((x) => ({ value: x.sv.id, label: x.st.name }))];
    const cell = (m, u, x) => { const req = x.rows.filter((r) => r.l.required); const done = req.filter((r) => r.st.done).length;
      const tone = x.complete ? 'is-complete' : x.overdue ? 'is-overdue' : x.rework ? 'is-rework' : done ? 'is-progress' : 'is-todo';
      const state = x.complete ? 'Hoàn thành' : `${done}/${req.length} bắt buộc`;
      const flags = [x.overdue ? 'có bài quá hạn' : '', x.rework ? 'có bài cần làm lại' : ''].filter(Boolean);
      return `<td><button type="button" class="matrix-cell ${tone}" data-action="stage-summary" data-member="${m.id}" data-sv="${x.sv.id}" aria-label="${esc(u.name)}, ${esc(x.stage.name)}: ${state}${flags.length ? ', ' + flags.join(', ') : ''}, ${x.review ? 'đã nhận xét' : 'chưa nhận xét'}. Mở tổng kết chặng">
        <span class="mc-state">${x.complete ? icon('check-circle') : ''}${state}</span>
        <span class="mc-flags">${x.overdue ? '<span class="badge badge-danger">Quá hạn</span>' : ''}${x.rework ? '<span class="badge badge-warn">Làm lại</span>' : ''}${x.review ? `<span class="mc-review" title="Đã nhận xét">${icon('message')}</span>` : ''}</span></button></td>`; };
    const body = rows.map((r) => `<tr><th scope="row"><div class="cell-2"><span class="primary">${esc(r.u.name)}</span><span class="secondary">${esc(r.u.email)}</span></div></th>${r.sums.map((x) => cell(r.m, r.u, x)).join('')}<td class="num">${progressBar(r.p.pct)}</td></tr>`).join('');
    return `<div class="card">
      <form class="filter-bar" data-form="summary-filter" data-base="${basePath}">
        ${field('Chưa hoàn thành chặng', `<select class="input" name="incomplete">${selectOpts(stageOpts, f.incomplete)}</select>`)}
        ${field('Chưa có nhận xét chặng', `<select class="input" name="nocomment">${selectOpts(stageOpts, f.nocomment)}</select>`)}
        <label class="check"><input type="checkbox" name="rework" value="1" ${f.rework ? 'checked' : ''}> Có bài cần làm lại</label>
        <label class="check"><input type="checkbox" name="overdue" value="1" ${f.overdue ? 'checked' : ''}> Có bài quá hạn nộp</label>
        <div class="row"><button class="un-btn un-btn-navy un-btn-sm" type="submit">Lọc</button>${anyFilter ? `<a class="un-btn un-btn-ghost un-btn-sm" href="${basePath}">Xóa bộ lọc</a>` : ''}</div>
      </form>
      <div class="card-head" style="border-bottom:0;padding-bottom:0"><span class="small muted">${rows.length}/${total} học viên · Bấm một ô để xem tổng kết chặng và viết nhận xét</span>
        <span class="matrix-legend small"><span class="lg is-complete">Hoàn thành</span><span class="lg is-progress">Đang học</span><span class="lg is-rework">Cần làm lại</span><span class="lg is-overdue">Quá hạn</span></span></div>
      <div class="table-wrap"><table class="matrix"><thead><tr><th>Học viên</th>${cols.map((x) => `<th title="${esc(x.st.name)} ${versionLabel(x.sv)}">${esc(x.st.name)}</th>`).join('')}<th class="num">Toàn khóa</th></tr></thead>
        <tbody>${body || `<tr><td colspan="${cols.length + 2}">${anyFilter ? empty('Không có học viên khớp bộ lọc', 'Nới lỏng điều kiện hoặc xóa bộ lọc.', `<a class="un-btn un-btn-secondary un-btn-sm" href="${basePath}">Xóa bộ lọc</a>`) : empty('Chưa có học viên', 'Mời học viên vào lớp để xem tổng kết.')}</td></tr>`}</tbody></table></div></div>`;
  }
  // Per-lesson detail line used by the stage summary drawer and the student's own summary page.
  function lessonDetail(m, l) {
    if (l.type === 'quiz') {
      const done = doneAttempts(m.id, l.id); const best = quizBest(m.id, l.id); const lim = attemptLimit(m.id, l);
      return done.length ? `Tốt nhất ${pctOf(best.score, best.max)}% (${best.score}/${best.max}) · ${done.length}${lim != null ? `/${lim}` : ''} lượt` : lim != null ? `Chưa làm · ${lim} lượt` : 'Chưa làm';
    }
    if (l.type === 'homework') {
      const s = latestSub(m.id, l.id); const due = deadlineOf(m.classId, l.id); const g = s?.grades[s.grades.length - 1];
      return s ? `Lần nộp #${s.no} · ${fmtDateTime(s.submittedAt)}${s.late ? ' · nộp muộn' : ''}${g ? ` · ${g.total}/${g.max} điểm` : ''}` : due ? `Hạn ${fmtDateTime(due)} · ${dueIn(due)}` : 'Chưa nộp · chưa đặt hạn';
    }
    const pr = progressOf(m.id, l.id);
    return pr?.completedAt ? `Tích hoàn thành ${fmtDateTime(pr.completedAt)}` : pr ? `Mở lần đầu ${fmtDateTime(pr.firstOpenedAt)}` : 'Chưa mở';
  }
  function stageSummaryDrawer(memberId, svId) {
    const m = byId(S.members, memberId); const u = user(m.userId); const c = cls(m.classId); const x = stageSummary(m, svId);
    const req = x.rows.filter((r) => r.l.required); const done = req.filter((r) => r.st.done).length; const pct = req.length ? Math.round(done * 100 / req.length) : 0;
    const editable = canGrade(me(), c); const rv = x.review;
    return `<div class="modal-head"><div><h2>${esc(u.name)} · ${esc(x.stage.name)}</h2><div class="small muted">${esc(c.code)} · ${versionLabel(x.sv)} · ${x.complete ? 'Đã hoàn thành chặng' : `Còn ${x.missing.length} học liệu bắt buộc`}</div></div><button class="icon-btn" data-action="close-drawer" aria-label="Đóng">${icon('x')}</button></div>
      <div class="card-pad" style="border-bottom:1px solid var(--line)">${progressBar(pct, true)}<div class="small muted" style="margin-top:var(--sp-2)">${done}/${req.length} học liệu bắt buộc · video và bài đọc do học viên tự tích; quiz và bài tập tính theo kết quả</div></div>
      <div class="panel-body">
        <ul class="list">${x.rows.map(({ l, st }) => `<li><span class="type-icon" title="${TYPE_VI[l.type]}">${typeIcon(l)}</span><div class="grow"><div class="title">${esc(l.title)}${l.required ? '' : ' <span class="badge badge-optional">Không bắt buộc</span>'}</div><div class="meta"><span>${lessonDetail(m, l)}</span></div></div>${stBadge(st)}</li>`).join('')}</ul>
        <form class="card-pad stack-sm" data-form="stage-review" data-member="${m.id}" data-sv="${svId}" novalidate style="border-top:1px solid var(--line)">
          <h3>Nhận xét tổng kết chặng</h3>
          ${rv ? `<p class="small muted">Cập nhật ${fmtDateTime(rv.at)} bởi ${esc(user(rv.by)?.name || '')}${rv.history?.length ? ` · đã sửa ${rv.history.length} lần` : ''}. Học viên thấy nhận xét này trong trang tổng kết.</p>` : '<p class="small muted">Học viên thấy nhận xét này trong trang tổng kết của mình.</p>'}
          ${editable ? `${field('Nhận xét', `<textarea class="input input-prose" name="text" rows="4" required>${esc(rv?.text || '')}</textarea>`, { required: true })}
          <div class="alert alert-danger" data-form-error hidden>${icon('alert')}<div class="alert-body"></div></div>
          <div class="form-actions"><button class="un-btn un-btn-primary un-btn-sm" type="submit">${rv ? 'Lưu thay đổi' : 'Lưu nhận xét'}</button></div>` : `<p>${rv ? esc(rv.text) : '<span class="muted">Chưa có nhận xét.</span>'}</p>`}
        </form>
      </div>`;
  }
  function renderHomeworkTab(c) {
    const lessons = classLessons(c).filter((l) => l.type === 'homework'); const ms = activeMembersOf(c.id); const u = me();
    if (!lessons.length) return `<div class="card">${empty('Khóa học không có bài tập', 'Phiên bản khóa học của lớp này không có học liệu loại bài tập.')}</div>`;
    return `<div class="stack">${lessons.map((l) => { const due = deadlineOf(c.id, l.id); const st = svOfLesson(l.id);
      const rows = ms.map((m) => ({ m, stu: user(m.userId), s: latestSub(m.id, l.id), st: lessonStatus(m, l) }));
      const n = (state) => rows.filter((r) => r.st.state === state).length; const submitted = rows.filter((r) => r.s).length;
      return `<section class="card"><div class="card-head"><div><h2>${esc(l.title)}</h2><div class="small muted">${esc(stage(st.stageId).name)} · ${hwMax(l.homework)} điểm · đạt từ ${l.homework.threshold}%</div></div>
          <div class="row"><span class="small ${due && new Date(due) < new Date() ? 'text-danger' : 'muted'}">${icon('calendar')} ${due ? `Hạn ${fmtDateTime(due)} · ${dueIn(due)}` : 'Chưa đặt hạn nộp'}</span><button class="un-btn un-btn-secondary un-btn-sm" data-action="set-deadline" data-class="${c.id}" data-lesson="${l.id}" ${c.status === 'ended' ? 'aria-disabled="true" title="Lớp đã kết thúc"' : ''}>${due ? 'Đổi hạn nộp' : 'Đặt hạn nộp'}</button></div></div>
        <div class="card-body hw-stats small"><span><b class="num">${submitted}</b>/${rows.length} đã nộp</span><span><b class="num">${n('pending')}</b> chờ chấm</span><span><b class="num">${n('passed')}</b> đạt</span><span><b class="num">${n('rework')}</b> cần làm lại</span><span><b class="num">${n('overdue')}</b> quá hạn chưa nộp</span></div>
        <div class="table-wrap"><table class="hw-table"><colgroup><col><col><col><col><col></colgroup><thead><tr><th>Học viên</th><th>Trạng thái</th><th>Lần nộp gần nhất</th><th class="num">Điểm</th><th class="cell-actions">Thao tác</th></tr></thead><tbody>
          ${rows.map(({ stu, s, st: status }) => { const g = s?.grades[s.grades.length - 1];
            return `<tr class="${s && isOverdueGrading(s) ? 'is-overdue' : ''}"><td><div class="cell-2"><span class="primary">${esc(stu.name)}</span><span class="secondary">${esc(stu.email)}</span></div></td><td>${stBadge(status)}${s && isOverdueGrading(s) ? ' <span class="badge badge-danger">Quá 3 ngày chưa chấm</span>' : ''}</td>
              <td>${s ? `<div class="cell-2"><span>#${s.no} · ${fmtDateTime(s.submittedAt)}</span><span class="secondary">${s.late ? '<span class="text-danger">Nộp muộn</span> · ' : ''}${s.repoUrl ? 'Repo' : ''}${s.repoUrl && s.files.length ? ' + ' : ''}${s.files.length ? `${s.files.length} file` : ''}</span></div>` : '<span class="muted">—</span>'}</td>
              <td class="num">${g ? `${g.total}/${g.max}` : '—'}</td>
              <td class="cell-actions">${s && canGrade(u, c) ? `<a class="un-btn ${s.status === 'pending' ? 'un-btn-primary' : 'un-btn-ghost'} un-btn-sm" href="${gradeHref(s.id)}">${s.status === 'pending' ? 'Chấm' : 'Xem'}</a>` : ''}</td></tr>`; }).join('') || `<tr><td colspan="5">${empty('Chưa có học viên', 'Mời học viên vào lớp trước.')}</td></tr>`}
        </tbody></table></div></section>`; }).join('')}</div>`;
  }
  function questionRates(l, ms) {
    const ids = new Set(ms.map((m) => m.id)); const atts = S.quizAttempts.filter((a) => a.lessonId === l.id && a.status === 'submitted' && ids.has(a.memberId));
    return { count: atts.length, rows: l.quiz.questions.map((x) => { const right = x.options.filter((o) => o.correct).map((o) => o.id).sort().join();
      const ok_ = atts.filter((a) => (a.answers[x.id] || []).slice().sort().join() === right).length; return { x, pct: atts.length ? Math.round(ok_ * 100 / atts.length) : null }; }) };
  }
  function renderQuizTab(c) {
    const lessons = classLessons(c).filter((l) => l.type === 'quiz'); const ms = activeMembersOf(c.id); const u = me();
    if (!lessons.length) return `<div class="card">${empty('Khóa học không có quiz', 'Phiên bản khóa học của lớp này không có học liệu loại quiz.')}</div>`;
    return `<div class="stack">${lessons.map((l) => { const z = l.quiz; const rates = questionRates(l, ms); const st = svOfLesson(l.id);
      const tried = ms.filter((m) => doneAttempts(m.id, l.id).length); const passed = ms.filter((m) => lessonStatus(m, l).done).length;
      const avg = tried.length ? Math.round(tried.reduce((a, m) => { const b = quizBest(m.id, l.id); return a + pctOf(b.score, b.max); }, 0) / tried.length) : null;
      const grants = S.attemptGrants.filter((g) => g.lessonId === l.id && ms.some((m) => m.id === g.memberId)).sort((a, b) => b.at.localeCompare(a.at));
      const memberTable = z.mode !== 'assessment' ? '' : `<div class="table-wrap"><table><thead><tr><th>Học viên</th><th class="num">Lượt đã dùng</th><th class="num">Tốt nhất</th><th>Trạng thái</th><th class="cell-actions">Thao tác</th></tr></thead><tbody>
        ${ms.map((m) => { const stu = user(m.userId); const used = attemptsOf(m.id, l.id).length; const lim = attemptLimit(m.id, l); const b = quizBest(m.id, l.id); const status = lessonStatus(m, l); const out = used >= lim && !status.done;
          return `<tr><td><div class="cell-2"><span class="primary">${esc(stu.name)}</span><span class="secondary">${esc(stu.email)}</span></div></td><td class="num">${used}/${lim}</td><td class="num">${b ? `${pctOf(b.score, b.max)}%` : '—'}</td><td>${stBadge(status)}</td>
            <td class="cell-actions">${canGrade(u, c) && !status.done ? `<button class="un-btn ${out ? 'un-btn-secondary' : 'un-btn-ghost'} un-btn-sm" data-action="grant-attempts" data-member="${m.id}" data-lesson="${l.id}" ${c.status === 'active' ? '' : 'aria-disabled="true" title="Lớp không ở trạng thái đang chạy"'}>${icon('plus')} Cấp thêm lượt</button>` : ''}</td></tr>`; }).join('')}</tbody></table></div>
        ${grants.length ? `<div class="card-body"><div class="section-label">Lịch sử cấp thêm lượt</div><ul class="list compact">${grants.map((g) => `<li><div class="grow"><div class="title">+${g.extra} lượt cho ${esc(user(byId(S.members, g.memberId).userId).name)}</div><div class="meta"><span>${esc(g.reason)}</span><span>${esc(user(g.by)?.name || '')}</span></div></div><span class="when">${rel(g.at)}</span></li>`).join('')}</ul></div>` : ''}`;
      return `<section class="card"><div class="card-head"><div><h2>${esc(l.title)}</h2><div class="small muted">${esc(stage(st.stageId).name)} · ${z.mode === 'assessment' ? `Kiểm tra · ${z.attempts} lượt · ${z.timeLimit} phút` : 'Luyện tập · không giới hạn lượt'} · đạt từ ${z.threshold}%</div></div><span class="badge ${z.mode === 'assessment' ? 'badge-info' : 'badge-optional'}">${z.mode === 'assessment' ? 'Tính vào tiến độ' : 'Luyện tập'}</span></div>
        <div class="card-body hw-stats small"><span><b class="num">${tried.length}</b>/${ms.length} đã làm</span>${z.mode === 'assessment' ? `<span><b class="num">${passed}</b> đạt</span>` : ''}<span>Điểm TB lượt tốt nhất <b class="num">${avg == null ? '—' : avg + '%'}</b></span><span><b class="num">${rates.count}</b> lượt đã nộp</span></div>
        <div class="card-body"><div class="section-label">Tỉ lệ trả lời đúng từng câu</div>
          ${rates.count ? `<div class="rate-list">${rates.rows.map(({ x, pct }, i) => `<div class="rate-row"><span class="rate-q"><b>Câu ${i + 1}.</b> ${mdInline(x.text)}${pct < 30 ? ' <span class="badge badge-danger">Dưới 30%: xem lại câu hỏi</span>' : ''}</span><span class="rate-bar${pct < 30 ? ' is-low' : ''}" role="img" aria-label="${pct}% trả lời đúng"><i style="width:${pct}%"></i></span><span class="rate-pct num">${pct}%</span></div>`).join('')}</div>` : '<p class="small muted" style="margin-top:var(--sp-2)">Chưa có lượt nộp nào.</p>'}</div>
        ${memberTable}</section>`; }).join('')}</div>`;
  }
  function renderStudentsTab(c) {
    const ms = membersOf(c.id).sort((a, b) => (a.status === 'dropped') - (b.status === 'dropped') || user(a.userId).name.localeCompare(user(b.userId).name, 'vi'));
    const rows = ms.map((m) => { const u = user(m.userId); const inv = latestInvitation(c.id, u.id);
      const acct = u.status === 'disabled' ? dot('disabled') : u.mustChangePassword ? dot('invited', (u.tempPasswordExpiresAt && new Date(u.tempPasswordExpiresAt) < new Date()) ? 'Mật khẩu tạm hết hạn' : 'Chưa đăng nhập') : dot('active', 'Đã kích hoạt');
      const invHtml = inv ? `<div class="cell-2"><span>${dot(inv.status, inv.kind === 'added' && inv.status === 'sent' ? 'Đã gửi thông báo' : STATUS_VI[inv.status])}</span><span class="secondary">${inv.status === 'failed' ? esc(inv.lastError) + ` · ${inv.attempts} lần thử` : rel(inv.createdAt)}</span></div>` : '—';
      const actions = m.status === 'dropped' ? `<span class="muted small">${STATUS_VI.dropped}</span>` : `${u.mustChangePassword && u.status !== 'disabled' ? `<button class="un-btn un-btn-ghost un-btn-sm" data-action="resend" data-class="${c.id}" data-user="${u.id}">${icon('mail')} Gửi lại</button>` : ''}${u.status === 'disabled' ? `<button class="un-btn un-btn-ghost un-btn-sm" data-action="enable-user" data-id="${u.id}">Kích hoạt lại</button>` : `<button class="un-btn un-btn-ghost un-btn-sm" data-action="disable-user" data-id="${u.id}">Vô hiệu hóa</button>`}<button class="un-btn un-btn-ghost un-btn-sm" data-action="remove-member" data-id="${m.id}">Gỡ khỏi lớp</button>`;
      return `<tr ${m.status === 'dropped' ? 'style="opacity:.6"' : ''}><td><div class="cell-2"><span class="primary">${esc(u.name)}</span><span class="secondary">${esc(u.email)}</span></div></td><td>${acct}</td><td>${invHtml}</td><td>${m.status === 'dropped' ? '<span class="muted">—</span>' : progressBar(memberProgress(m).pct)}</td><td class="cell-actions">${actions}</td></tr>`; }).join('');
    return `<div class="card"><div class="card-head"><h2>Học viên trong lớp</h2><button class="un-btn un-btn-primary un-btn-sm" data-action="invite" data-id="${c.id}" ${c.status === 'ended' ? 'aria-disabled="true" title="Không mời được vào lớp đã kết thúc"' : ''}>${icon('mail')} Mời học viên</button></div>
      <div class="table-wrap"><table><thead><tr><th>Học viên</th><th>Tài khoản</th><th>Lời mời</th><th>Tiến độ</th><th class="cell-actions">Thao tác</th></tr></thead><tbody>${rows || `<tr><td colspan="5">${empty('Chưa có học viên', 'Mời học viên bằng email; họ sẽ nhận mật khẩu tạm có hiệu lực 72 giờ.')}</td></tr>`}</tbody></table></div></div>`;
  }
  function renderSettingsTab(c) {
    const published = S.courseVersions.filter((x) => x.status === 'published' || x.id === c.courseVersionId);
    const locked = c.status !== 'draft';
    return `<form class="card" data-form="class-settings" data-id="${c.id}" novalidate><div class="card-head"><h2>Cài đặt lớp</h2></div><div class="card-body stack-sm">
      ${locked ? `<div class="alert alert-info">${icon('info')}<div class="alert-body">Lớp ${classStatusVi[c.status].toLowerCase()} không đổi được phiên bản khóa học. Lớp chạy trọn đời trên ${esc(course(cv(c.courseVersionId).courseId).name)} ${versionLabel(cv(c.courseVersionId))}.</div></div>` : ''}
      <div class="form-grid">
        ${field('Mã lớp', `<input class="input" value="${esc(c.code)}" disabled>`)}
        ${field('Tên lớp', `<input class="input" name="name" value="${esc(c.name)}" required>`, { required: true })}
        ${field('Phiên bản khóa học', `<select class="input" name="courseVersionId" ${locked ? 'disabled' : ''}>${selectOpts(published.map((x) => ({ value: x.id, label: `${course(x.courseId).name} ${versionLabel(x)}${x.status !== 'published' ? ' (' + STATUS_VI[x.status] + ')' : ''}` })), c.courseVersionId)}</select>`, { required: true, help: locked ? '' : 'Chỉ liệt kê phiên bản đã phát hành.' })}
        ${field('Giảng viên phụ trách', `<select class="input" name="teacherId">${selectOpts(S.users.filter((u) => u.role === 'teacher').map((u) => ({ value: u.id, label: u.name })), c.teacherId)}</select>`, { required: true })}
        ${field('Ngày bắt đầu dự kiến', `<input class="input" type="date" name="startDate" value="${c.startDate}">`, { required: true })}
        ${field('Ngày kết thúc dự kiến', `<input class="input" type="date" name="endDate" value="${c.endDate}">`, { required: true })}
      </div>
      <div class="alert alert-danger" data-form-error hidden>${icon('alert')}<div class="alert-body"></div></div>
      <div class="form-actions"><button class="un-btn un-btn-primary" type="submit">Lưu thay đổi</button></div></div></form>`;
  }

  /* ---------------------------------------------------- report (FR-40) */
  function renderReport(c, query, basePath) {
    const v = cv(c.courseVersionId); const stagesCols = v.stages.map((id) => ({ sv: sv(id), st: stage(sv(id).stageId) }));
    const f = { notlogged: query.notlogged === '1', inactive: query.inactive || '', below: query.below || '', sort: query.sort || 'name' };
    let rows = activeMembersOf(c.id).map((m) => ({ m, u: user(m.userId), p: memberProgress(m) }));
    if (f.notlogged) rows = rows.filter((r) => !r.u.lastLoginAt);
    if (f.inactive) rows = rows.filter((r) => daysSince(r.u.lastActiveAt) > Number(f.inactive));
    if (f.below) rows = rows.filter((r) => r.p.pct < Number(f.below));
    const sorters = { name: (a, b) => a.u.name.localeCompare(b.u.name, 'vi'), pct: (a, b) => a.p.pct - b.p.pct, 'pct-desc': (a, b) => b.p.pct - a.p.pct, activity: (a, b) => (new Date(b.u.lastActiveAt || 0)) - (new Date(a.u.lastActiveAt || 0)), 'activity-asc': (a, b) => (new Date(a.u.lastActiveAt || 0)) - (new Date(b.u.lastActiveAt || 0)) };
    rows.sort(sorters[f.sort] || sorters.name);
    const anyFilter = f.notlogged || f.inactive || f.below;
    const total = activeMembersOf(c.id).length;
    const sortLink = (key, label, cls = '') => { const active = f.sort.startsWith(key); const next = active && f.sort === key ? key + (key === 'pct' ? '-desc' : '-asc') : key; return `<th class="${cls}"><a class="sort" style="color:inherit;text-decoration:none;display:inline-flex;align-items:center;gap:4px" href="${basePath}${q({ ...f, notlogged: f.notlogged ? 1 : '', sort: next }).replace('?', basePath.includes('?') ? '&' : '?')}" ${active ? 'aria-sort="other"' : ''}>${label} ${icon('sort')}</a></th>`; };
    const thead = `<tr><th>Học viên</th><th>Lời mời</th>${stagesCols.map((s) => `<th class="num" title="${esc(s.st.name)} ${versionLabel(s.sv)}">${esc(s.st.code)}</th>`).join('')}${sortLink('pct', 'Toàn khóa', 'num')}${sortLink('activity', 'Hoạt động cuối')}</tr>`;
    const body = rows.map(({ m, u, p }) => { const inv = latestInvitation(c.id, u.id);
      return `<tr class="is-clickable" data-action="student-detail" data-member="${m.id}" tabindex="0"><td><div class="cell-2"><span class="primary">${esc(u.name)}</span><span class="secondary">${esc(u.email)}</span></div></td><td>${inv ? dot(inv.status) : '—'}</td>${p.stages.map((s) => `<td class="num">${s.total ? `<span style="color:${s.pct === 100 ? 'var(--ok)' : 'inherit'}">${s.pct}%</span>` : '—'}</td>`).join('')}<td class="num">${progressBar(p.pct)}</td><td><div class="cell-2"><span>${u.lastActiveAt ? rel(u.lastActiveAt) : '<span class="dot dot-invited">Chưa đăng nhập</span>'}</span>${u.lastActiveAt ? `<span class="secondary">${fmtDateTime(u.lastActiveAt)}</span>` : ''}</div></td></tr>`; }).join('');
    return `<div class="card">
      <form class="filter-bar" data-form="report-filter" data-base="${basePath}">
        <label class="check"><input type="checkbox" name="notlogged" value="1" ${f.notlogged ? 'checked' : ''}> Chưa đăng nhập</label>
        ${field('Không hoạt động quá (ngày)', `<input class="input" type="number" min="1" name="inactive" value="${esc(f.inactive)}" placeholder="7">`)}
        ${field('% toàn khóa dưới', `<input class="input" type="number" min="1" max="100" name="below" value="${esc(f.below)}" placeholder="50">`)}
        ${field('Sắp xếp', `<select class="input" name="sort">${selectOpts([{ value: 'name', label: 'Tên A → Z' }, { value: 'pct', label: '% toàn khóa thấp → cao' }, { value: 'pct-desc', label: '% toàn khóa cao → thấp' }, { value: 'activity', label: 'Hoạt động mới nhất' }, { value: 'activity-asc', label: 'Lâu không hoạt động' }], f.sort)}</select>`)}
        <div class="row"><button class="un-btn un-btn-navy un-btn-sm" type="submit">Lọc</button>${anyFilter ? `<a class="un-btn un-btn-ghost un-btn-sm" href="${basePath}">Xóa bộ lọc</a>` : ''}</div>
      </form>
      <div class="card-head" style="border-bottom:0;padding-bottom:0"><span class="small muted">${rows.length}/${total} học viên · Bấm vào một dòng để xem từng học liệu</span><span class="badge badge-warn badge-wrap">${icon('info')} Video và bài đọc do học viên tự xác nhận; quiz và bài tập tính theo kết quả</span></div>
      <div class="table-wrap"><table><thead>${thead}</thead><tbody>${body || `<tr><td colspan="${4 + stagesCols.length}">${anyFilter ? empty('Không có học viên khớp bộ lọc', 'Nới lỏng điều kiện hoặc xóa bộ lọc.', `<a class="un-btn un-btn-secondary un-btn-sm" href="${basePath}">Xóa bộ lọc</a>`) : empty('Chưa có học viên', 'Mời học viên vào lớp để theo dõi tiến độ.')}</td></tr>`}</tbody></table></div></div>`;
  }
  function studentDetailDrawer(memberId) {
    const m = byId(S.members, memberId); const u = user(m.userId); const c = cls(m.classId); const v = cv(c.courseVersionId); const p = memberProgress(m);
    const body = v.stages.map((id, i) => { const s = sv(id); const st = stage(s.stageId); const sp = p.stages[i];
      return `<section><div class="card-head" style="background:var(--surface-50)"><h2>${i + 1}. ${esc(st.name)} <span class="muted small">${versionLabel(s)}</span></h2><div style="width:140px">${progressBar(sp.pct)}</div></div><ul class="list">${s.lessons.map((l) => `<li><span class="type-icon" title="${TYPE_VI[l.type]}">${typeIcon(l)}</span><div class="grow"><div class="title">${esc(l.title)} ${l.required ? '' : '<span class="badge badge-optional">Không bắt buộc</span>'}</div><div class="meta"><span>${lessonDetail(m, l)}</span></div></div>${stBadge(lessonStatus(m, l))}</li>`).join('')}</ul>
        <div class="card-pad" style="padding-top:var(--sp-3);padding-bottom:var(--sp-3)"><button class="un-btn un-btn-ghost un-btn-sm" data-action="stage-summary" data-member="${m.id}" data-sv="${s.id}">${icon('message')} Tổng kết chặng${S.stageReviews[m.id + ':' + s.id] ? ' · đã nhận xét' : ''}</button></div></section>`; }).join('');
    return `<div class="modal-head"><div><h2>${esc(u.name)}</h2><div class="small muted">${esc(u.email)} · ${c.code} · hoạt động cuối ${rel(u.lastActiveAt)}</div></div><button class="icon-btn" data-action="close-drawer" aria-label="Đóng">${icon('x')}</button></div>
      <div class="card-pad" style="border-bottom:1px solid var(--line)">${progressBar(p.pct, true)}<div class="small muted" style="margin-top:8px">${p.done}/${p.total} học liệu bắt buộc · video và bài đọc do học viên tự tích; quiz và bài tập tính theo kết quả</div></div>
      <div class="panel-body">${body}</div>`;
  }

  /* -------------------------------------------------- teacher pages */
  function teacherClassCards(classes, hrefFor) {
    return classes.length ? `<div class="grid-3">${classes.map((c) => { const v = cv(c.courseVersionId); const ms = activeMembersOf(c.id); const stale = c.status === 'active' ? ms.filter((m) => daysSince(user(m.userId).lastActiveAt) > 7).length : 0;
      return `<a class="card card-pad" href="${hrefFor(c)}" style="text-decoration:none;display:block"><div class="row-between"><h2>${esc(c.code)}</h2><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span></div><p class="muted small" style="margin:4px 0 16px">${esc(c.name)} · ${esc(course(v.courseId).name)} ${versionLabel(v)}</p>${c.status === 'draft' ? `<p class="small muted">Bắt đầu ${fmtDate(c.startDate)}</p>` : progressBar(classAvgProgress(c.id))}<div class="row small muted" style="margin-top:12px"><span>${ms.length} học viên</span>${stale ? `<span class="dot dot-invited">${stale} lâu không hoạt động</span>` : ''}</div></a>`; }).join('')}</div>` : `<div class="card">${empty('Chưa có lớp', 'Bạn chưa được phân công lớp nào.')}</div>`;
  }
  function pageTeachClasses() {
    const u = me(); const classes = S.classes.filter((c) => c.teacherId === u.id); const queue = gradingQueue(u); const late = queue.filter((r) => r.overdue).length;
    const alert = queue.length ? `<div class="alert ${late ? 'alert-danger' : 'alert-info'}">${icon(late ? 'alert' : 'inbox')}<div class="alert-body"><strong>${queue.length} bài đang chờ bạn chấm${late ? `, ${late} bài đã quá ${GRADING_SLA_DAYS} ngày` : ''}.</strong> Bài cũ nhất nộp ${rel(queue[0].s.submittedAt)}. <a href="#/teach/grading${late ? '?overdue=1' : ''}">Mở hàng chờ chấm</a></div></div>` : '';
    return { title: 'Lớp của tôi', html: `${pageHead('Lớp của tôi', { lede: 'Các lớp bạn phụ trách. Vào lớp để xem tiến độ, tổng kết chặng, bài tập và quiz.' })}<div class="stack">${alert}${teacherClassCards(classes, (c) => `#/teach/classes/${c.id}`)}</div>` };
  }
  function pageTeachClass([classId], query) {
    const c = cls(classId); const u = me();
    if (!c || c.teacherId !== u.id) { toast('Bạn chỉ xem được lớp mình phụ trách.', 'error'); location.hash = '#/teach'; return null; }
    const v = cv(c.courseVersionId); const keys = ['report', 'summary', 'homework', 'quiz']; const tab = keys.includes(query.tab) ? query.tab : 'report'; const base = `#/teach/classes/${c.id}`;
    return { title: c.code, html: `${pageHead(`${c.code} · ${c.name}`, { crumbsHtml: crumbs([['Lớp của tôi', '#/teach'], [c.code]]), badges: `<span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>`, lede: `${esc(course(v.courseId).name)} ${versionLabel(v)} · ${fmtDate(c.startDate)} → ${fmtDate(c.endDate)}` })}${classTabs(c, base, keys, tab)}${classTabBody(c, tab, query, base)}` };
  }

  /* ------------------------------------------------------- grading */
  function pageGrading(_, query) {
    const u = me(); const base = u.role === 'admin' ? '#/admin/grading' : '#/teach/grading';
    const myClasses = S.classes.filter((c) => canGrade(u, c) && c.status !== 'draft');
    const f = { classId: query.class || '', lessonId: query.lesson || '', overdue: query.overdue === '1' };
    const all = gradingQueue(u); let rows = gradingQueue(u, { classId: f.classId || undefined, lessonId: f.lessonId || undefined });
    if (f.overdue) rows = rows.filter((r) => r.overdue);
    const anyFilter = f.classId || f.lessonId || f.overdue; const late = all.filter((r) => r.overdue).length;
    const hwLessons = [...new Map(myClasses.filter((c) => !f.classId || c.id === f.classId).flatMap(classLessons).filter((l) => l.type === 'homework').map((l) => [l.id, l])).values()];
    const recent = S.submissions.filter((s) => s.grades.length && canGrade(u, cls(byId(S.members, s.memberId).classId))).sort((a, b) => b.grades[b.grades.length - 1].at.localeCompare(a.grades[a.grades.length - 1].at)).slice(0, 6);
    const table = `<div class="table-wrap"><table class="queue-table"><thead><tr><th>Học viên</th><th>Bài tập</th><th>Lớp</th><th>Nộp lúc</th><th class="num">Đã chờ</th><th class="cell-actions"><span class="sr-only">Thao tác</span></th></tr></thead><tbody>
      ${rows.map((r) => `<tr class="is-clickable${r.overdue ? ' is-overdue' : ''}" data-href="${gradeHref(r.s.id)}" tabindex="0"><td><div class="cell-2"><span class="primary">${esc(r.student.name)}</span><span class="secondary">Lần nộp #${r.s.no}${r.s.late ? ' · <span class="text-danger">nộp muộn</span>' : ''}</span></div></td>
        <td><div class="cell-2"><span>${esc(shortTitle(r.lesson))}</span><span class="secondary">${esc(stage(r.svx.stageId).name)}</span></div></td><td>${esc(r.c.code)}</td><td>${fmtDateTime(r.s.submittedAt)}</td>
        <td class="num">${r.overdue ? `<span class="badge badge-danger">Quá ${GRADING_SLA_DAYS} ngày</span> ` : ''}${fmtWait(r.waited)}</td><td class="cell-actions"><a class="un-btn un-btn-primary un-btn-sm" href="${gradeHref(r.s.id)}">Chấm</a></td></tr>`).join('')
      || `<tr><td colspan="6">${anyFilter ? empty('Không có bài khớp bộ lọc', 'Nới lỏng điều kiện hoặc xóa bộ lọc.', `<a class="un-btn un-btn-secondary un-btn-sm" href="${base}">Xóa bộ lọc</a>`) : empty('Không còn bài chờ chấm', 'Khi học viên nộp bài, bài sẽ hiện ở đây theo thứ tự nộp sớm nhất trước.')}</td></tr>`}</tbody></table></div>`;
    return { title: 'Chấm bài', html: `${pageHead('Chấm bài', { lede: `Bài nộp chờ chấm, cũ nhất ở trên. Mục tiêu chấm trong ${GRADING_SLA_DAYS} ngày kể từ lúc nộp.`, badges: late ? `<span class="badge badge-danger">${late} bài quá ${GRADING_SLA_DAYS} ngày</span>` : '' })}
      <div class="stack"><section class="card" aria-label="Hàng chờ chấm">
        <form class="filter-bar" data-form="grading-filter" data-base="${base}">
          ${field('Lớp', `<select class="input" name="class">${selectOpts([{ value: '', label: 'Tất cả lớp' }, ...myClasses.map((c) => ({ value: c.id, label: c.code }))], f.classId)}</select>`)}
          ${field('Bài tập', `<select class="input" name="lesson">${selectOpts([{ value: '', label: 'Tất cả bài tập' }, ...hwLessons.map((l) => ({ value: l.id, label: shortTitle(l) }))], f.lessonId)}</select>`)}
          <label class="check"><input type="checkbox" name="overdue" value="1" ${f.overdue ? 'checked' : ''}> Chỉ bài quá ${GRADING_SLA_DAYS} ngày</label>
          <div class="row"><button class="un-btn un-btn-navy un-btn-sm" type="submit">Lọc</button>${anyFilter ? `<a class="un-btn un-btn-ghost un-btn-sm" href="${base}">Xóa bộ lọc</a>` : ''}</div>
        </form>
        <div class="card-head" style="border-bottom:0;padding-bottom:0"><span class="small muted">${rows.length}/${all.length} bài chờ chấm</span></div>${table}</section>
        ${u.role === 'admin' ? backlogCard() : ''}
        <section class="card"><div class="card-head"><h2>Đã chấm gần đây</h2></div>${recent.length ? `<ul class="list">${recent.map((s) => { const g = s.grades[s.grades.length - 1]; const m = byId(S.members, s.memberId); const l = lessonById(s.lessonId);
          return `<li><div class="grow"><a class="title" href="${gradeHref(s.id)}">${esc(user(m.userId).name)} · ${esc(shortTitle(l))}</a><div class="meta"><span>${esc(cls(m.classId).code)}</span><span>Lần nộp #${s.no}</span><span>${g.total}/${g.max} điểm</span><span>${esc(user(g.by)?.name || '')} chấm ${rel(g.at)}</span></div></div>${stBadge(g.result === 'passed' ? { tone: 'ok', label: 'Đạt' } : { tone: 'warn', label: 'Cần làm lại' })}</li>`; }).join('')}</ul>` : empty('Chưa chấm bài nào', 'Bài đã chấm sẽ hiện ở đây.')}</section></div>` };
  }
  function pageGradeSubmission([subId]) {
    const u = me(); const s = byId(S.submissions, subId); const m = s && byId(S.members, s.memberId); const c = m && cls(m.classId); const base = u.role === 'admin' ? '#/admin/grading' : '#/teach/grading';
    if (!s || !canGrade(u, c)) { toast('Bạn chỉ chấm được bài của lớp mình phụ trách.', 'error'); location.hash = base; return null; }
    const l = lessonById(s.lessonId); const hw = l.homework; const stu = user(m.userId); const latest = latestSub(m.id, l.id); const isLatest = latest.id === s.id;
    const last = s.grades[s.grades.length - 1]; const due = deadlineOf(c.id, l.id); const max = hwMax(hw);
    const scoreOf = (cid) => last?.scores.find((x) => x.criterionId === cid);
    const note = !isLatest ? `<div class="alert alert-warn">${icon('alert')}<div class="alert-body">Đây là lần nộp #${s.no}; học viên đã nộp lần #${latest.no}. Chỉ chấm lần nộp mới nhất. <a href="${gradeHref(latest.id)}">Mở lần nộp #${latest.no}</a></div></div>` : '';
    const history = subsOf(m.id, l.id).slice().reverse();
    const left = `<div class="stack">${note}
      <section class="card"><div class="card-head"><h2>Bài nộp #${s.no}</h2>${s.status === 'pending' ? `<span class="badge badge-info">Chờ chấm · ${fmtWait(daysSince(s.submittedAt))}</span>` : stBadge(s.status === 'passed' ? { tone: 'ok', label: 'Đạt' } : s.status === 'rework' ? { tone: 'warn', label: 'Cần làm lại' } : { tone: 'optional', label: 'Đã thay bằng lần nộp sau' })}</div>
        <div class="card-body"><dl class="facts">
          <dt>Học viên</dt><dd>${esc(stu.name)} <span class="muted small">${esc(stu.email)}</span></dd>
          <dt>Lớp</dt><dd>${esc(c.code)} · ${esc(c.name)}</dd>
          <dt>Nộp lúc</dt><dd>${fmtDateTime(s.submittedAt)}${s.late ? ' <span class="badge badge-danger">Nộp muộn</span>' : ''}</dd>
          <dt>Hạn nộp</dt><dd>${due ? fmtDateTime(due) : 'Chưa đặt'}</dd>
          ${s.repoUrl ? `<dt>Repo</dt><dd><a href="${esc(s.repoUrl)}" target="_blank" rel="noopener noreferrer">${icon('link')} ${esc(s.repoUrl.replace(/^https:\/\//, ''))}<span class="sr-only"> (mở tab mới)</span></a></dd>` : ''}
          ${s.commitRef ? `<dt>Commit/tag</dt><dd><code>${esc(s.commitRef)}</code></dd>` : ''}
          ${s.files.length ? `<dt>File</dt><dd class="row">${s.files.map((x) => `<button type="button" class="file-chip" data-action="download-file" data-name="${esc(x.name)}">${icon('download')} ${esc(x.name)} <span class="muted">${fmtSize(x.size)}</span></button>`).join('')}</dd>` : ''}
        </dl></div></section>
      <details class="card"><summary class="card-head"><h2>Đề bài: ${esc(shortTitle(l))}</h2><span class="small muted">Mở để xem</span></summary><div class="card-body"><div class="md">${renderMarkdown(hw.brief)}</div></div></details>
      <section class="card"><div class="card-head"><h2>Lịch sử nộp và chấm</h2></div><ul class="list">${history.map((x) => `<li><div class="grow"><div class="title">${x.id === s.id ? `Lần nộp #${x.no} (đang xem)` : `<a class="title" href="${gradeHref(x.id)}">Lần nộp #${x.no}</a>`}</div><div class="meta"><span>${fmtDateTime(x.submittedAt)}</span>${x.late ? '<span class="text-danger">nộp muộn</span>' : ''}${x.grades.map((g) => `<span>${esc(user(g.by)?.name || '')}: ${g.total}/${g.max} · ${g.result === 'passed' ? 'Đạt' : 'Cần làm lại'}${g.comment ? ` · "${esc(g.comment)}"` : ''}</span>`).join('')}</div></div></li>`).join('')}</ul></section></div>`;
    const lock = !isLatest || s.status === 'superseded';
    const form = `<form class="card grade-form" data-form="grade" data-id="${s.id}" novalidate>
      <div class="card-head"><h2>${last ? 'Sửa kết quả chấm' : 'Chấm theo tiêu chí'}</h2><span class="small muted">Đạt từ ${hw.threshold}% (${Math.ceil(max * hw.threshold / 100)}/${max} điểm)</span></div>
      <fieldset class="card-body stack-sm" ${lock ? 'disabled' : ''}><legend class="sr-only">Điểm từng tiêu chí</legend>
        ${hw.criteria.map((cr, i) => { const sc = scoreOf(cr.id); return `<div class="rubric-row"><div class="rubric-head"><label for="sc-${cr.id}"><span class="ord">${i + 1}</span> ${esc(cr.text)}</label><span class="rubric-score"><input class="input num" id="sc-${cr.id}" type="number" inputmode="numeric" name="score-${cr.id}" data-grade-score min="0" max="${cr.max}" step="1" value="${sc ? sc.score : ''}" required aria-describedby="mx-${cr.id}"><span id="mx-${cr.id}" class="muted small">/ ${cr.max}</span></span></div>
          <input class="input" name="note-${cr.id}" value="${esc(sc?.comment || '')}" placeholder="Ghi chú cho tiêu chí này (không bắt buộc)" aria-label="Ghi chú: ${esc(cr.text)}"></div>`; }).join('')}
        <div class="grade-total" data-grade-total aria-live="polite" data-max="${max}" data-threshold="${hw.threshold}"></div>
        ${field('Nhận xét chung', `<textarea class="input input-prose" name="comment" rows="4" required>${esc(last?.comment || '')}</textarea>`, { required: true, help: 'Học viên nhận email kết quả kèm nhận xét này.' })}
        <div class="alert alert-danger" data-form-error hidden>${icon('alert')}<div class="alert-body"></div></div>
        <div class="form-actions"><a class="un-btn un-btn-ghost" href="${base}">Về hàng chờ</a><button class="un-btn un-btn-primary" type="submit">${last ? 'Lưu kết quả mới' : 'Lưu kết quả'}</button></div>
      </fieldset></form>`;
    return { title: `Chấm bài · ${stu.name}`, html: `${pageHead(`${stu.name} · ${shortTitle(l)}`, { crumbsHtml: crumbs([['Chấm bài', base], [`${stu.name} #${s.no}`]]), lede: `${esc(c.code)} · ${esc(stage(svOfLesson(l.id).stageId).name)} · ${hw.criteria.length} tiêu chí, tổng ${max} điểm` })}<div class="grade-layout">${left}${form}</div>`,
      after: () => updateGradeTotal($('form[data-form=grade]')) };
  }
  function updateGradeTotal(form) {
    if (!form) return; const out = form.querySelector('[data-grade-total]'); const max = Number(out.dataset.max); const th = Number(out.dataset.threshold);
    const vals = [...form.querySelectorAll('[data-grade-score]')]; const filled = vals.filter((x) => x.value !== '');
    const total = filled.reduce((a, x) => a + Math.max(0, Math.min(Number(x.max), Number(x.value) || 0)), 0); const pass = pctOf(total, max) >= th;
    out.innerHTML = `<span>Tổng <b class="num">${total}/${max}</b> (${pctOf(total, max)}%)</span>${filled.length < vals.length ? `<span class="muted small">Còn ${vals.length - filled.length} tiêu chí chưa nhập</span>` : `<span class="badge ${pass ? 'badge-ok' : 'badge-warn'}">${pass ? 'Đạt' : 'Cần làm lại'}</span>`}`;
  }

  /* -------------------------------------------------- student pages */
  // Work the student still owes: homework not yet Đạt and assessments not yet passed, soonest deadline first.
  function todoItems(m) {
    const c = cls(m.classId); if (c.status !== 'active') return [];
    return classLessons(c).filter((l) => l.required && (l.type === 'homework' || (l.type === 'quiz' && l.quiz.mode === 'assessment'))).map((l) => ({ l, st: lessonStatus(m, l), due: l.type === 'homework' ? deadlineOf(c.id, l.id) : null }))
      .filter((x) => !x.st.done && !(x.l.type === 'quiz' && x.st.tone === 'danger'))
      .sort((a, b) => (a.due ? new Date(a.due) : Infinity) - (b.due ? new Date(b.due) : Infinity));
  }
  function pageLearn() {
    const u = me(); const ms = S.members.filter((m) => m.userId === u.id && m.status === 'active');
    const cards = ms.map((m) => { const c = cls(m.classId); const v = cv(c.courseVersionId); const p = memberProgress(m);
      const next = c.status === 'active' ? classLessons(c).find((l) => !lessonStatus(m, l).done) : null;
      return `<div class="card card-pad"><div class="row-between"><h2>${esc(c.name)}</h2><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span></div><p class="muted small" style="margin:4px 0 16px">${esc(c.code)} · ${esc(course(v.courseId).name)} · ${fmtDate(c.startDate)} → ${fmtDate(c.endDate)}</p>
        ${progressBar(p.pct, true)}<p class="small muted" style="margin-top:6px">${p.done}/${p.total} học liệu bắt buộc</p>
        <div class="row" style="margin-top:16px">${c.status === 'draft' ? `<span class="small muted">Lớp bắt đầu ${fmtDate(c.startDate)}. Bạn sẽ vào học được khi lớp kích hoạt.</span>` : `<a class="un-btn ${next ? 'un-btn-primary' : 'un-btn-navy'}" href="${next ? `#/learn/classes/${c.id}/lessons/${next.id}` : `#/learn/classes/${c.id}`}">${next ? 'Học tiếp' : 'Xem lộ trình'}</a><a class="un-btn un-btn-ghost" href="#/learn/classes/${c.id}">Lộ trình</a><a class="un-btn un-btn-ghost" href="#/learn/classes/${c.id}/summary">Tổng kết</a>`}</div></div>`; }).join('');
    const todos = ms.flatMap((m) => todoItems(m).map((x) => ({ ...x, c: cls(m.classId) })));
    const todoCard = todos.length ? `<section class="card"><div class="card-head"><h2>Việc cần làm</h2><span class="small muted">${todos.length} bài tập và bài kiểm tra chưa đạt</span></div><ul class="list">${todos.map(({ l, st, due, c }) => { const late = due && new Date(due) < new Date();
      return `<li><span class="type-icon" title="${TYPE_VI[l.type]}">${typeIcon(l)}</span><div class="grow"><a class="title" href="#/learn/classes/${c.id}/lessons/${l.id}">${esc(l.title)}</a><div class="meta"><span>${esc(c.code)}</span>${due ? `<span class="${late ? 'text-danger' : ''}">${icon('calendar')} Hạn ${fmtDateTime(due)} · ${dueIn(due)}</span>` : l.type === 'quiz' ? `<span>Còn ${attemptLimit(memberOf(c.id, u.id).id, l) - attemptsOf(memberOf(c.id, u.id).id, l.id).length} lượt</span>` : '<span>Chưa đặt hạn</span>'}</div></div>${stBadge(st)}</li>`; }).join('')}</ul></section>` : '';
    return { title: 'Lớp của tôi', html: `${pageHead(`Xin chào, ${u.name.split(' ').pop()}`, { lede: 'Các lớp bạn đang tham gia.' })}<div class="stack">${todoCard}${cards ? `<div class="grid-2">${cards}</div>` : `<div class="card">${empty('Chưa có lớp', 'Khi được mời vào lớp, lớp sẽ hiện ở đây.')}</div>`}</div>` };
  }
  function learnerLesson(classId, lessonId) {
    const u = me(); const c = cls(classId); const m = c && memberOf(c.id, u.id);
    if (!m || m.status !== 'active') { location.hash = '#/learn'; return null; }
    return { u, c, m, v: cv(c.courseVersionId) };
  }
  function pageLearnClass([classId]) {
    const ctx = learnerLesson(classId); if (!ctx) return null; const { c, m, v } = ctx;
    const p = memberProgress(m); const canTick = c.status === 'active';
    const stagesHtml = v.stages.map((id, i) => { const s = sv(id); const st = stage(s.stageId); const sp = p.stages[i];
      return `<section class="card stage"><div class="stage-head"><span class="ord">${i + 1}</span><div class="grow"><h2>${esc(st.name)}</h2><div class="small muted">${s.lessons.length} học liệu · ${sp.done}/${sp.total} bắt buộc đã xong</div></div>${progressBar(sp.pct)}</div>
        ${s.lessons.map((l) => { const status = lessonStatus(m, l); const done = status.done; const pr = progressOf(m.id, l.id); const opened = !!pr; const graded = l.type === 'quiz' || l.type === 'homework';
          const due = l.type === 'homework' ? deadlineOf(c.id, l.id) : null;
          const sub = l.type === 'quiz' ? lessonMeta(l).replace(/^Quiz /, 'Quiz ') : l.type === 'homework' ? `Bài tập${due ? ` · hạn ${fmtDateTime(due)}${done ? '' : ` (${dueIn(due)})`}` : ' · chưa đặt hạn'}` : l.type === 'video' ? `Video · ${esc(l.duration || '')}` : 'Bài đọc';
          const right = graded ? `<div class="lesson-state">${stBadge(status)}</div>` : (() => { const disabled = !canTick || !opened; const why = !canTick ? (c.status === 'ended' ? 'Lớp đã kết thúc' : 'Lớp chưa bắt đầu') : !opened ? 'Mở học liệu trước khi tích' : '';
            return `<label class="check${disabled ? ' is-disabled' : ''}" title="${esc(why)}"><input type="checkbox" data-change="toggle-complete" data-class="${c.id}" data-lesson="${l.id}" ${done ? 'checked' : ''} ${disabled ? 'disabled' : ''} aria-label="Đã học xong: ${esc(l.title)}"><span class="small">Đã học xong</span></label>`; })();
          return `<div class="lesson-row${done ? ' is-done' : ''}" data-kind="${l.type}"><a class="lesson-link" href="#/learn/classes/${c.id}/lessons/${l.id}"><span class="type-icon" title="${TYPE_VI[l.type]}">${typeIcon(l)}</span><span><span class="t">${esc(l.title)}</span><br><span class="sub">${sub}${l.required ? '' : ' · không bắt buộc'}${!graded && opened && !done ? ' · đã mở' : ''}</span></span></a>${right}</div>`; }).join('')}</section>`; }).join('');
    return { title: c.code, html: `${pageHead(c.name, { crumbsHtml: crumbs([['Lớp của tôi', '#/learn'], [c.code]]), badges: `<span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>`, lede: `${esc(course(v.courseId).name)} ${versionLabel(v)} · Giảng viên ${esc(user(c.teacherId)?.name || '')}`, actions: `<a class="un-btn un-btn-secondary" href="#/learn/classes/${c.id}/summary">${icon('bar-chart')} Xem tổng kết</a>` })}
      <div class="stack">
        <div class="card card-pad"><div class="row-between"><div><div class="section-label">Tiến độ toàn khóa</div><div class="small muted">${p.done}/${p.total} học liệu bắt buộc. Video và bài đọc tính khi bạn tích; quiz kiểm tra tính khi đạt ngưỡng; bài tập tính khi được chấm Đạt.</div></div><div style="min-width:260px">${progressBar(p.pct, true)}</div></div>
          ${c.status !== 'active' ? `<div class="alert alert-info" style="margin-top:16px">${icon('info')}<div class="alert-body">${c.status === 'ended' ? 'Lớp đã kết thúc: bạn vẫn xem được học liệu và kết quả nhưng không tích, làm quiz hay nộp bài được nữa.' : 'Lớp chưa bắt đầu.'}</div></div>` : ''}</div>
        <div class="roadmap">${stagesHtml}</div>
      </div>` };
  }
  function pageLearnSummary([classId]) {
    const ctx = learnerLesson(classId); if (!ctx) return null; const { c, m, v } = ctx; const p = memberProgress(m);
    const cards = courseSummary(m).map((x, i) => { const req = x.rows.filter((r) => r.l.required); const done = req.filter((r) => r.st.done).length; const rv = x.review;
      return `<section class="card"><div class="card-head"><div><h2>${i + 1}. ${esc(x.stage.name)}</h2><div class="small muted">${done}/${req.length} học liệu bắt buộc</div></div>${x.complete ? '<span class="badge badge-ok">Hoàn thành chặng</span>' : x.overdue ? '<span class="badge badge-danger">Có bài quá hạn</span>' : x.rework ? '<span class="badge badge-warn">Có bài cần làm lại</span>' : '<span class="badge badge-info">Đang học</span>'}</div>
        <div class="card-pad" style="padding-bottom:0">${progressBar(req.length ? Math.round(done * 100 / req.length) : 0)}</div>
        <ul class="list">${x.rows.map(({ l, st }) => `<li><span class="type-icon" title="${TYPE_VI[l.type]}">${typeIcon(l)}</span><div class="grow"><a class="title" href="#/learn/classes/${c.id}/lessons/${l.id}">${esc(l.title)}</a><div class="meta"><span>${lessonDetail(m, l)}</span>${l.required ? '' : '<span>không bắt buộc</span>'}</div></div>${stBadge(st)}</li>`).join('')}</ul>
        ${rv ? `<div class="card-body review-box"><div class="section-label">Nhận xét của giảng viên</div><p>${esc(rv.text)}</p><div class="small muted">${esc(user(rv.by)?.name || '')} · ${fmtDateTime(rv.at)}</div></div>` : '<div class="card-body review-empty small muted">Chưa có nhận xét của giảng viên cho chặng này.</div>'}</section>`; }).join('');
    return { title: `Tổng kết · ${c.code}`, html: `${pageHead('Tổng kết khóa học', { crumbsHtml: crumbs([['Lớp của tôi', '#/learn'], [c.code, `#/learn/classes/${c.id}`], ['Tổng kết']]), lede: `${esc(c.name)} · ${esc(course(v.courseId).name)} ${versionLabel(v)}` })}
      <div class="stack"><div class="card card-pad"><div class="row-between"><div><div class="section-label">Toàn khóa</div><div class="small muted">${p.done}/${p.total} học liệu bắt buộc</div></div><div style="min-width:260px">${progressBar(p.pct, true)}</div></div></div>${cards}</div>` };
  }
  function pageLesson([classId, lessonId], query) {
    const ctx = learnerLesson(classId); if (!ctx) return null; const { c, m, v } = ctx;
    let found = null;
    v.stages.forEach((id, i) => { const s = sv(id); const l = s.lessons.find((x) => x.id === lessonId); if (l) found = { s, st: stage(s.stageId), l, i }; });
    if (!found) { location.hash = `#/learn/classes/${c.id}`; return null; }
    if (c.status === 'active') openLesson(c.id, lessonId); // first open is recorded once; later opens do not overwrite it
    const { s, st, l } = found; const status = lessonStatus(m, l);
    const all = classLessons(c); const idx = all.findIndex((x) => x.id === l.id); const prev = all[idx - 1]; const next = all[idx + 1];
    const navBtns = `<div class="row">${prev ? `<a class="un-btn un-btn-ghost un-btn-sm" href="#/learn/classes/${c.id}/lessons/${prev.id}">Bài trước</a>` : ''}${next ? `<a class="un-btn un-btn-ghost un-btn-sm" href="#/learn/classes/${c.id}/lessons/${next.id}">Bài tiếp ${icon('arrow-right')}</a>` : ''}</div>`;
    let content; let after;
    if (l.type === 'quiz') ({ content, after } = quizView(c, m, l, query, navBtns));
    else if (l.type === 'homework') content = homeworkView(c, m, l, navBtns);
    else {
      const pr = progressOf(m.id, l.id); const done = !!pr?.completedAt; const canTick = c.status === 'active';
      content = `<div class="card">${l.type === 'video'
        ? `<div class="video-frame" role="img" aria-label="Khung phát video ${esc(l.video)}"><div><div class="play">${icon('play')}</div><strong>${esc(l.title)}</strong><small>${esc(l.video)} · ${esc(l.duration || '')} · phát qua đường dẫn có chữ ký (giả lập trong bản mẫu)</small></div></div>`
        : `<div class="card-body"><div class="md">${renderMarkdown(l.content)}</div></div>`}
        <div class="complete-bar">${navBtns}
          <div class="row">${done ? `<span class="dot dot-ok small">Đã tích ${fmtDateTime(pr.completedAt)}</span>` : ''}<button class="un-btn ${done ? 'un-btn-secondary' : 'un-btn-primary'}" data-action="complete" data-class="${c.id}" data-lesson="${l.id}" data-done="${done ? 0 : 1}" ${canTick ? '' : `aria-disabled="true" title="${c.status === 'ended' ? 'Lớp đã kết thúc' : 'Lớp chưa bắt đầu'}"`}>${done ? 'Bỏ tích' : `${icon('check')} Đã học xong`}</button></div></div></div>`;
    }
    const side = `<aside class="card"><div class="card-head"><h2>${esc(st.name)} <span class="muted small">${versionLabel(s)}</span></h2></div><ul class="side-list">${s.lessons.map((x) => { const d = lessonStatus(m, x).done; return `<li><a href="#/learn/classes/${c.id}/lessons/${x.id}" ${x.id === l.id ? 'aria-current="page"' : ''}>${d ? icon('check-circle', 'done-mark') : '<span class="todo-mark"></span>'}<span>${esc(x.title)}</span>${d ? '<span class="sr-only"> (đã xong)</span>' : ''}</a></li>`; }).join('')}</ul>
      <div class="card-pad"><a class="un-btn un-btn-ghost un-btn-sm" href="#/learn/classes/${c.id}">${icon('layers')} Toàn bộ lộ trình</a></div></aside>`;
    const badges = `${l.type === 'quiz' || l.type === 'homework' ? stBadge(status) : ''}${l.required ? '' : ' <span class="badge badge-optional">Không bắt buộc</span>'}`;
    return { title: l.title, html: `${pageHead(l.title, { crumbsHtml: crumbs([['Lớp của tôi', '#/learn'], [c.code, `#/learn/classes/${c.id}`], [st.name, `#/learn/classes/${c.id}`], [l.title]]), badges })}
      <div class="viewer">${content}${side}</div>`, after };
  }
  function quizView(c, m, l, query, navBtns) {
    const z = l.quiz; const base = `#/learn/classes/${c.id}/lessons/${l.id}`;
    const done = doneAttempts(m.id, l.id); const open = openAttempt(m.id, l.id); const lim = attemptLimit(m.id, l); const left = lim == null ? null : lim - attemptsOf(m.id, l.id).length;
    const assess = z.mode === 'assessment'; const best = quizBest(m.id, l.id); const passed = lessonStatus(m, l).done;
    const blocked = c.status !== 'active' ? (c.status === 'ended' ? 'Lớp đã kết thúc' : 'Lớp chưa bắt đầu') : left != null && left <= 0 && !open ? `Bạn đã dùng hết ${lim} lượt` : '';
    const startBtn = (label) => `<button class="un-btn un-btn-primary" data-action="start-quiz" data-class="${c.id}" data-lesson="${l.id}" ${blocked ? `aria-disabled="true" title="${esc(blocked)}"` : ''}>${icon(open ? 'arrow-right' : 'play')} ${label}</button>`;
    const result = query.result && done.find((a) => a.id === query.result);
    if (open && !result) {
      const qs = (open.order || z.questions.map((x) => x.id)).map((id) => z.questions.find((x) => x.id === id)).filter(Boolean);
      const answered = qs.filter((x) => (open.answers[x.id] || []).length).length;
      const body = qs.map((x, i) => { const ids = open.optOrder?.[x.id] || x.options.map((o) => o.id); const multi = x.type === 'multi'; const chosen = open.answers[x.id] || [];
        return `<fieldset class="quiz-q" data-q="${x.id}"><legend><span class="ord">${i + 1}</span><span>${mdInline(x.text)}</span></legend><p class="small muted">${multi ? 'Chọn tất cả đáp án đúng' : 'Chọn một đáp án'} · ${x.points} điểm</p>
          ${ids.map((oid) => x.options.find((o) => o.id === oid)).filter(Boolean).map((o) => `<label class="quiz-opt"><input type="${multi ? 'checkbox' : 'radio'}" name="q-${x.id}" value="${o.id}" ${chosen.includes(o.id) ? 'checked' : ''} data-change="quiz-answer" data-attempt="${open.id}" data-q="${x.id}"><span>${mdInline(o.text)}</span></label>`).join('')}</fieldset>`; }).join('');
      return { content: `<div class="card quiz-take">
        <div class="quiz-bar">${open.deadlineAt ? `<span class="timer-pill" role="timer" data-deadline="${open.deadlineAt}" aria-label="Thời gian còn lại">${icon('timer')}<span data-timer-text>--:--</span></span>` : '<span class="small muted">Không giới hạn thời gian</span>'}
          <span class="small"><span data-answered data-total="${qs.length}">${answered}/${qs.length}</span> câu đã trả lời</span><span class="small muted" data-saved aria-live="polite">${open.savedAt ? `Đã lưu ${fmtDateTime(open.savedAt).slice(-5)}` : 'Câu trả lời được lưu tự động'}</span>
          <button class="un-btn un-btn-primary un-btn-sm" data-action="submit-quiz" data-id="${open.id}" data-class="${c.id}" data-lesson="${l.id}">Nộp bài</button></div>
        <div class="card-body stack">${assess ? `<div class="alert alert-info">${icon('info')}<div class="alert-body">Lượt ${open.no}${lim != null ? `/${lim}` : ''}. Hết giờ, hệ thống tự nộp với các câu đã lưu. Rời trang không dừng đồng hồ.</div></div>` : ''}${body}</div>
        <div class="complete-bar">${navBtns}<button class="un-btn un-btn-primary" data-action="submit-quiz" data-id="${open.id}" data-class="${c.id}" data-lesson="${l.id}">Nộp bài</button></div></div>`,
        after: () => startQuizTimer(open.id, `${base}?result=${open.id}`) };
    }
    if (result) {
      const pct = pctOf(result.score, result.max); const pass = pct >= z.threshold;
      const review = assess ? `<p class="small muted">${icon('lock')} Bài kiểm tra chỉ hiện điểm và kết quả Đạt, không hiện đáp án <span class="badge badge-optional">đề xuất</span></p>`
        : `<div class="stack">${z.questions.map((x, i) => { const right = x.options.filter((o) => o.correct).map((o) => o.id); const given = result.answers[x.id] || []; const okQ = given.length && given.slice().sort().join() === right.slice().sort().join();
          return `<fieldset class="quiz-q is-review"><legend><span class="ord">${i + 1}</span><span>${mdInline(x.text)}</span></legend><p class="small ${okQ ? 'text-ok' : 'text-danger'}">${icon(okQ ? 'check-circle' : 'x-circle')} ${okQ ? `Đúng · ${x.points} điểm` : given.length ? 'Sai · 0 điểm' : 'Chưa trả lời · 0 điểm'}</p>
            ${x.options.map((o) => { const picked = given.includes(o.id); const cls_ = o.correct ? ' is-correct' : picked ? ' is-wrong' : ''; return `<div class="quiz-opt${cls_}">${icon(o.correct ? 'check-circle' : picked ? 'x-circle' : 'circle')}<span>${mdInline(o.text)}</span><span class="sr-only">${o.correct ? ' (đáp án đúng)' : ''}${picked ? ' (bạn chọn)' : ''}</span>${picked ? '<span class="badge badge-optional">Bạn chọn</span>' : ''}</div>`; }).join('')}
            ${x.explanation ? `<div class="quiz-explain"><strong>Giải thích.</strong> ${mdInline(x.explanation)}</div>` : ''}</fieldset>`; }).join('')}</div>`;
      return { content: `<div class="card"><div class="card-body stack">
        <div class="quiz-score ${pass ? 'is-pass' : 'is-fail'}"><div><div class="section-label">Lượt ${result.no} · nộp ${fmtDateTime(result.submittedAt)}</div><div class="quiz-score-num num">${result.score}/${result.max} <span>(${pct}%)</span></div></div>
          ${assess ? `<span class="badge ${pass ? 'badge-ok' : 'badge-warn'}">${pass ? 'Đạt' : `Chưa đạt · cần ${z.threshold}%`}</span>` : `<span class="badge badge-info">Luyện tập</span>`}</div>
        ${result.autoSubmitted ? `<div class="alert alert-warn">${icon('clock')}<div class="alert-body">Lượt này được tự nộp khi hết giờ. Chỉ các câu đã lưu trước hạn được chấm.</div></div>` : ''}
        ${assess && best ? `<p class="small muted">Kết quả tính vào tiến độ là lượt tốt nhất: ${pctOf(best.score, best.max)}%.${left != null ? ` Còn ${Math.max(0, left)} lượt.` : ''}</p>` : ''}
        ${review}</div>
        <div class="complete-bar"><a class="un-btn un-btn-ghost un-btn-sm" href="${base}">Về trang quiz</a><div class="row">${!passed || !assess ? startBtn('Làm lại') : ''}</div></div></div>` };
    }
    const facts = `<dl class="facts">
      <dt>Hình thức</dt><dd>${assess ? 'Kiểm tra, tính vào tiến độ' : 'Luyện tập, làm bao nhiêu lần cũng được'}</dd>
      <dt>Số câu</dt><dd>${z.questions.length} câu · ${z.questions.reduce((a, x) => a + x.points, 0)} điểm</dd>
      <dt>Ngưỡng đạt</dt><dd>${z.threshold}%</dd>
      ${assess ? `<dt>Thời gian</dt><dd>${z.timeLimit} phút mỗi lượt</dd><dt>Số lượt</dt><dd>${attemptsOf(m.id, l.id).length}/${lim} đã dùng${grantsOf(m.id, l.id).length ? ' · đã được cấp thêm' : ''} · tính lượt tốt nhất <span class="badge badge-optional">đề xuất</span></dd>` : ''}
    </dl>`;
    const hist = done.length ? `<section><div class="section-label" style="margin-bottom:var(--sp-2)">Các lượt đã nộp</div><ul class="list bordered attempt-list">${done.slice().reverse().map((a) => `<li><div class="grow"><a class="title" href="${base}?result=${a.id}">Lượt ${a.no}</a><div class="meta"><span>${fmtDateTime(a.submittedAt)}</span>${a.autoSubmitted ? '<span>tự nộp khi hết giờ</span>' : ''}</div></div><span class="num">${a.score}/${a.max} · ${pctOf(a.score, a.max)}%</span>${stBadge(pctOf(a.score, a.max) >= z.threshold ? { tone: 'ok', label: 'Đạt' } : { tone: 'warn', label: 'Chưa đạt' })}</li>`).join('')}</ul></section>` : '';
    return { content: `<div class="card"><div class="card-body stack">${facts}
      ${passed && assess ? `<div class="alert alert-ok">${icon('check-circle')}<div class="alert-body">Bạn đã đạt bài kiểm tra này với ${pctOf(best.score, best.max)}%.${left > 0 ? ' Bạn vẫn có thể làm thêm; kết quả tính theo lượt tốt nhất.' : ''}</div></div>` : ''}
      ${blocked && c.status === 'active' ? `<div class="alert alert-warn">${icon('alert')}<div class="alert-body">${esc(blocked)} mà chưa đạt. Liên hệ giảng viên nếu cần cấp thêm lượt.</div></div>` : ''}
      ${!blocked && c.status === 'active' ? `<p class="hint-line">${icon(assess ? 'timer' : 'info')}<span>${assess ? `Đồng hồ ${z.timeLimit} phút bắt đầu khi bạn bấm làm bài và vẫn chạy nếu bạn rời trang. Câu trả lời được lưu ngay khi chọn; hết giờ, bài tự nộp.` : 'Câu trả lời được lưu ngay khi chọn. Nộp xong, bạn xem được đáp án và giải thích từng câu.'}</span></p>` : ''}
      ${hist}</div>
      <div class="complete-bar">${navBtns}${startBtn(done.length ? 'Làm lượt mới' : 'Bắt đầu làm bài')}</div></div>` };
  }
  function startQuizTimer(attemptId, resultHash) {
    const pill = $('.timer-pill'); if (!pill) return; const end = new Date(pill.dataset.deadline).getTime(); const warned = { 5: false, 1: false };
    const tick = () => {
      const left = Math.max(0, end - Date.now()); const mm = Math.floor(left / 60e3); const ss = Math.floor((left % 60e3) / 1e3);
      pill.querySelector('[data-timer-text]').textContent = `${pad(mm)}:${pad(ss)}`;
      pill.classList.toggle('is-warn', left <= 5 * 60e3 && left > 60e3); pill.classList.toggle('is-danger', left <= 60e3);
      [5, 1].forEach((n) => { if (!warned[n] && left <= n * 60e3 && left > 0) { warned[n] = true; toast(`Còn ${n} phút. Câu trả lời đã chọn được lưu tự động.`, 'warn'); } });
      if (left === 0) { clearInterval(quizTimer); quizTimer = null; const r = submitAttempt(attemptId, { auto: true }); if (r.ok) { toast('Hết giờ: hệ thống đã tự nộp bài với các câu đã lưu.', 'warn'); go(resultHash); } }
    };
    tick(); quizTimer = setInterval(tick, 1000);
  }
  function homeworkView(c, m, l, navBtns) {
    const hw = l.homework; const subs = subsOf(m.id, l.id); const latest = subs[subs.length - 1]; const due = deadlineOf(c.id, l.id); const max = hwMax(hw);
    const g = latest?.grades[latest.grades.length - 1]; const passed = latest?.status === 'passed'; const late = due && new Date(due) < new Date();
    const feedback = g ? `<section class="card ${latest.status === 'passed' ? 'is-pass' : 'is-fail'} feedback"><div class="card-head"><h2>Kết quả lần nộp #${latest.no}</h2>${stBadge(latest.status === 'passed' ? { tone: 'ok', label: 'Đạt' } : latest.status === 'rework' ? { tone: 'warn', label: 'Cần làm lại' } : { tone: 'info', label: 'Chờ chấm lần nộp mới' })}</div>
      <div class="card-body stack-sm"><div class="quiz-score-num num">${g.total}/${g.max} <span>(${pctOf(g.total, g.max)}%)</span></div>
        <div class="table-wrap"><table><thead><tr><th>Tiêu chí</th><th class="num">Điểm</th><th>Ghi chú</th></tr></thead><tbody>${hw.criteria.map((cr) => { const sc = g.scores.find((x) => x.criterionId === cr.id); return `<tr><td>${esc(cr.text)}</td><td class="num">${sc ? sc.score : '—'}/${cr.max}</td><td>${sc?.comment ? esc(sc.comment) : '<span class="muted">—</span>'}</td></tr>`; }).join('')}</tbody></table></div>
        <div class="review-box"><div class="section-label">Nhận xét của ${esc(user(g.by)?.name || 'giảng viên')}</div><p>${esc(g.comment)}</p><div class="small muted">${fmtDateTime(g.at)}</div></div></div></section>` : '';
    const lock = c.status !== 'active' ? (c.status === 'ended' ? 'Lớp đã kết thúc, không nộp bài được nữa.' : 'Lớp chưa bắt đầu.') : passed ? 'Bài đã được chấm Đạt nên không cần nộp lại.' : '';
    const form = lock ? `<div class="alert ${passed ? 'alert-ok' : 'alert-info'}">${icon(passed ? 'check-circle' : 'info')}<div class="alert-body">${lock}</div></div>` : `<form class="card" data-form="submit-homework" data-class="${c.id}" data-lesson="${l.id}" novalidate>
      <div class="card-head"><h2>${latest ? `Nộp lại (lần #${latest.no + 1})` : 'Nộp bài'}</h2>${due ? `<span class="small ${late ? 'text-danger' : 'muted'}">${icon('calendar')} Hạn ${fmtDateTime(due)} · ${dueIn(due)}</span>` : ''}</div>
      <div class="card-body stack-sm">
        ${latest?.status === 'pending' ? `<div class="alert alert-info">${icon('info')}<div class="alert-body">Lần nộp #${latest.no} đang chờ chấm. Nộp lại sẽ thay lần nộp đó; giảng viên chỉ chấm lần mới nhất.</div></div>` : ''}
        ${late ? `<div class="alert alert-warn">${icon('clock')}<div class="alert-body">Đã quá hạn nộp. Bạn vẫn nộp được, bài sẽ được đánh dấu nộp muộn.</div></div>` : ''}
        ${field('Đường dẫn repo', `<input class="input" type="url" name="repoUrl" inputmode="url" autocomplete="url" placeholder="https://github.com/ten-ban/ten-repo" value="${esc(latest?.repoUrl || '')}">`, { help: 'GitHub, GitLab hoặc Bitbucket. Repo riêng tư: thêm giảng viên làm người xem <span class="badge badge-optional">đề xuất</span>' })}
        ${field('Commit hoặc tag', `<input class="input" name="commitRef" placeholder="vd: a1b2c3d hoặc v1.0" value="${esc(latest?.commitRef || '')}">`, { help: 'Không bắt buộc. Giúp giảng viên chấm đúng phiên bản bạn nộp.' })}
        ${field('File đính kèm', `<input class="input input-file" type="file" name="file" multiple accept="${FILE_EXTS.map((x) => '.' + x).join(',')}">`, { help: `Tối đa ${MAX_FILES} file, mỗi file ${MAX_FILE_MB} MB: ${FILE_EXTS.join(', ')} <span class="badge badge-optional">đề xuất</span>. Cần repo hoặc ít nhất một file.` })}
        <div class="alert alert-danger" data-form-error hidden>${icon('alert')}<div class="alert-body"></div></div>
        <div class="form-actions"><button class="un-btn un-btn-primary" type="submit">${icon('upload')} ${latest ? 'Nộp lại' : 'Nộp bài'}</button></div>
      </div></form>`;
    const history = subs.length ? `<section class="card"><div class="card-head"><h2>Lịch sử nộp</h2></div><ul class="list">${subs.slice().reverse().map((x) => { const gx = x.grades[x.grades.length - 1];
      return `<li><div class="grow"><div class="title">Lần nộp #${x.no}</div><div class="meta"><span>${fmtDateTime(x.submittedAt)}</span>${x.late ? '<span class="text-danger">nộp muộn</span>' : ''}${x.repoUrl ? `<span>${esc(x.repoUrl.replace(/^https:\/\//, ''))}</span>` : ''}${x.files.length ? `<span>${x.files.map((f) => esc(f.name)).join(', ')}</span>` : ''}${gx ? `<span>${gx.total}/${gx.max} điểm</span>` : ''}</div></div>${stBadge(x.status === 'passed' ? { tone: 'ok', label: 'Đạt' } : x.status === 'rework' ? { tone: 'warn', label: 'Cần làm lại' } : x.status === 'pending' ? { tone: 'info', label: 'Chờ chấm' } : { tone: 'optional', label: 'Đã thay bằng lần sau' })}</li>`; }).join('')}</ul></section>` : '';
    return `<div class="stack">
      <section class="card"><div class="card-body"><div class="md">${renderMarkdown(hw.brief)}</div>
        ${hw.attachments?.length ? `<div class="section-label" style="margin:var(--sp-5) 0 var(--sp-2)">Tài liệu kèm theo</div><div class="row">${hw.attachments.map((n) => `<button type="button" class="file-chip" data-action="download-file" data-name="${esc(n)}">${icon('download')} ${esc(n)}</button>`).join('')}</div>` : ''}
        <div class="section-label" style="margin:var(--sp-5) 0 var(--sp-2)">Tiêu chí chấm · đạt từ ${hw.threshold}% (${Math.ceil(max * hw.threshold / 100)}/${max} điểm)</div>
        <ul class="list bordered">${hw.criteria.map((cr, i) => `<li><span class="ord">${i + 1}</span><div class="grow">${esc(cr.text)}</div><span class="num muted">${cr.max} điểm</span></li>`).join('')}</ul></div>
        <div class="complete-bar">${navBtns}</div></section>
      ${feedback}${form}${history}</div>`;
  }

  /* -------------------------------------------------------- dialogs */
  function lessonForm(l = {}) {
    const type = l.type || 'video'; const z = l.quiz || { mode: 'practice', threshold: 70, attempts: 2, timeLimit: 20, shuffle: true }; const hw = l.homework || { brief: '', threshold: 70, attachments: [] };
    const sec = (k, html) => `<div class="stack-sm" data-lesson-section="${k}" ${type === k ? '' : 'hidden'}>${html}</div>`;
    return `<div class="stack-sm">
      ${field('Tiêu đề', `<input class="input" name="title" value="${esc(l.title || '')}" required>`, { required: true })}
      <div class="form-grid">${field('Loại', `<select class="input" name="type" data-change="lesson-type">${selectOpts([{ value: 'video', label: 'Video' }, { value: 'markdown', label: 'Bài đọc (Markdown)' }, { value: 'quiz', label: 'Quiz' }, { value: 'homework', label: 'Bài tập' }], type)}</select>`, { help: l.id ? 'Đổi loại sẽ bỏ nội dung của loại cũ.' : '' })}
        <label class="check" style="align-self:end"><input type="checkbox" name="required" ${l.required !== false ? 'checked' : ''}> Bắt buộc (tính vào %)</label></div>
      ${sec('video', `${field('File video', `<input class="input" name="video" value="${esc(l.video || '')}" placeholder="vd: db-02-join.mp4">`, { help: 'Bản mẫu chỉ ghi tên file; hệ thống thật upload lên kho lưu trữ và phát qua đường dẫn có chữ ký.' })}${field('Thời lượng', `<input class="input" name="duration" value="${esc(l.duration || '')}" placeholder="mm:ss">`)}`)}
      ${sec('markdown', field('Nội dung Markdown', `<textarea class="input input-prose" name="content" rows="10">${esc(l.content || '')}</textarea>`, { help: 'Hỗ trợ tiêu đề, danh sách, khối mã, trích dẫn, **đậm** và `mã`. HTML được lọc khi hiển thị.' }))}
      ${sec('quiz', `<div class="form-grid">${field('Hình thức', `<select class="input" name="mode" data-change="quiz-mode">${selectOpts([{ value: 'practice', label: 'Luyện tập: không giới hạn, không tính tiến độ' }, { value: 'assessment', label: 'Kiểm tra: giới hạn lượt và giờ, tính tiến độ' }], z.mode)}</select>`)}
          ${field('Ngưỡng đạt (%)', `<input class="input" type="number" inputmode="numeric" name="quizThreshold" min="1" max="100" value="${z.threshold}">`)}</div>
        <div class="form-grid" data-quiz-limits ${z.mode === 'assessment' ? '' : 'hidden'}>${field('Số lượt làm bài', `<input class="input" type="number" inputmode="numeric" name="attempts" min="1" max="10" value="${z.attempts ?? 2}">`, { help: 'Tính lượt tốt nhất <span class="badge badge-optional">đề xuất</span>' })}
          ${field('Thời gian mỗi lượt (phút)', `<input class="input" type="number" inputmode="numeric" name="timeLimit" min="1" max="180" value="${z.timeLimit ?? 20}">`)}</div>
        <label class="check"><input type="checkbox" name="shuffle" ${z.shuffle ? 'checked' : ''}> Đảo thứ tự câu hỏi và đáp án mỗi lượt</label>
        <p class="small muted">${l.id ? 'Soạn câu hỏi ở trang chi tiết học liệu.' : 'Sau khi tạo, bạn được chuyển đến trang soạn câu hỏi.'}</p>`)}
      ${sec('homework', `${field('Đề bài (Markdown)', `<textarea class="input input-prose" name="brief" rows="8">${esc(hw.brief)}</textarea>`, { help: 'Mô tả yêu cầu, đầu ra cần nộp và cách chấm.' })}
        <div class="form-grid">${field('Ngưỡng đạt (%)', `<input class="input" type="number" inputmode="numeric" name="hwThreshold" min="1" max="100" value="${hw.threshold}">`)}
          ${field('Tài liệu kèm theo', `<input class="input" name="attachments" value="${esc(hw.attachments.join(', '))}" placeholder="vd: schema.sql, data.zip">`, { help: 'Tên file, cách nhau bằng dấu phẩy.' })}</div>
        <p class="small muted">${l.id ? 'Soạn tiêu chí chấm ở trang chi tiết học liệu.' : 'Sau khi tạo, bạn được chuyển đến trang soạn tiêu chí chấm.'}</p>`)}
    </div>`;
  }
  const inputs = (form) => Object.fromEntries(new FormData(form).entries());

  /* ------------------------------------------------------- actions */
  function handle(result, successMsg, after) {
    if (!result.ok) { if ($('#modal').open) modalError(result.error); else toast(result.error, 'error'); return false; }
    closeModal(); if (successMsg) toast(successMsg); if (after) after(); else render(); return true;
  }
  const ACTIONS = {
    'close-toast': (el) => el.closest('.toast')?.remove(),
    'toggle-nav': (btn) => { const nav = $('#nav'); const open = nav.classList.toggle('is-open'); btn.setAttribute('aria-expanded', String(open)); },
    'close-modal': () => closeModal(),
    'close-drawer': () => closeDrawer(),
    'logout': () => { S.session = null; save(); location.hash = '#/login'; render(); },
    'reset-data': () => confirmAction({ title: 'Đặt lại dữ liệu mẫu?', text: 'Mọi thay đổi bạn đã thực hiện trong bản mẫu này sẽ bị xóa và dữ liệu quay về kịch bản ban đầu.', confirm: 'Đặt lại', danger: true, onConfirm: () => { const id = S.session?.userId; resetData(); if (id && user(id)) S.session = { userId: id }; save(); return ok(); } }),
    'demo-login': (btn) => { const f = $('form[data-form=login]'); f.email.value = btn.dataset.email; f.password.value = 'demo-password'; f.requestSubmit(); },
    'new-stage': () => openModal({ title: 'Tạo chặng', confirm: 'Tạo chặng', body: `<div class="form-grid">${field('Mã chặng', '<input class="input" name="code" placeholder="vd: DOCKER" required>', { required: true, help: 'Duy nhất, dùng trong báo cáo.' })}${field('Tên chặng', '<input class="input" name="name" placeholder="vd: Docker cơ bản" required>', { required: true })}</div><p class="small muted">Hệ thống tạo kèm phiên bản nháp v1 để bạn thêm học liệu.</p>`, onSubmit: (d) => { const r = createStage(d.code, d.name); if (r.ok) { closeModal(); toast('Đã tạo chặng với bản nháp v1.'); location.hash = `#/admin/stages/${r.id}`; } return r; } }),
    'clone-sv': (btn) => handle(cloneStage(btn.dataset.id), 'Đã tạo bản nháp mới. Học liệu được sao chép, giữ nguyên lesson_key.', () => { const d = draftOf(stageVersionsOf(sv(btn.dataset.id).stageId)); location.hash = `#/admin/stages/${d.stageId}?v=${d.id}`; }),
    'publish-sv': (btn) => { const v = sv(btn.dataset.id); const b = stagePublishBlockers(v); if (b.length) { toast(`Chưa phát hành được: ${b[0]}${b.length > 1 ? ` (và ${b.length - 1} vấn đề khác)` : ''}`, 'error'); return; } confirmAction({ title: `Phát hành ${stage(v.stageId).name} ${versionLabel(v)}?`, text: `Sau khi phát hành, phiên bản này và ${v.lessons.length} học liệu của nó không sửa được nữa. Khóa học đang dùng phiên bản cũ sẽ không tự cập nhật; bạn áp dụng ở bước tiếp theo.`, confirm: 'Phát hành', onConfirm: () => { const r = publishStage(v.id); if (r.ok) { closeModal(); toast(`Đã phát hành ${versionLabel(v)}.`); render(); } return r; } }); },
    'archive-sv': (btn) => { const v = sv(btn.dataset.id); confirmAction({ title: `Lưu trữ ${versionLabel(v)}?`, text: 'Phiên bản lưu trữ không gắn mới vào khóa học được, nhưng các khóa học và lớp đang dùng vẫn hoạt động bình thường.', confirm: 'Lưu trữ', onConfirm: () => handle(archiveStageVersion(v.id), 'Đã lưu trữ.') && ok() }); },
    'delete-sv': (btn) => { const v = sv(btn.dataset.id); const st = stage(v.stageId); const refs = cvsUsingSV(v.id); if (v.status !== 'draft' && refs.length) { toast(`Không xóa được: đang dùng trong ${refs.map((c) => `${course(c.courseId).name} ${versionLabel(c)}`).join(', ')}.`, 'error'); return; }
      confirmAction({ title: `Xóa ${st.name} ${versionLabel(v)}?`, text: v.status === 'draft' ? `Bản nháp và ${v.lessons.length} học liệu trong đó sẽ bị xóa. Không hoàn tác được.` : 'Phiên bản này không được khóa học nào tham chiếu. Xóa sẽ không hoàn tác được.', confirm: 'Xóa phiên bản', danger: true, onConfirm: () => { const r = deleteStageVersion(v.id); if (r.ok) { closeModal(); toast('Đã xóa.'); location.hash = r.stageGone ? '#/admin/stages' : `#/admin/stages/${st.id}`; render(); } return r; } }); },
    'new-lesson': (btn) => openModal({ title: 'Thêm học liệu', confirm: 'Thêm', body: lessonForm(), onSubmit: (d) => { const r = saveLesson(btn.dataset.sv, null, { ...d, required: d.required === 'on', shuffle: d.shuffle === 'on' });
      const authored = d.type === 'quiz' || d.type === 'homework';
      return handle(r, authored ? `Đã thêm học liệu. Tiếp theo: soạn ${d.type === 'quiz' ? 'câu hỏi' : 'tiêu chí chấm'}.` : 'Đã thêm học liệu.', authored ? () => go(`#/admin/stages/${sv(btn.dataset.sv).stageId}/lessons/${r.id}`) : null) && ok(); } }),
    'edit-lesson': (btn) => { const l = sv(btn.dataset.sv).lessons.find((x) => x.id === btn.dataset.id); openModal({ title: 'Sửa học liệu', body: lessonForm(l), onSubmit: (d) => handle(saveLesson(btn.dataset.sv, l.id, { ...d, required: d.required === 'on', shuffle: d.shuffle === 'on' }), 'Đã lưu học liệu.') && ok() }); },
    'delete-lesson': (btn) => { const l = sv(btn.dataset.sv).lessons.find((x) => x.id === btn.dataset.id); confirmAction({ title: `Xóa "${l.title}"?`, text: 'Học liệu bị xóa khỏi bản nháp này. Các phiên bản đã phát hành không bị ảnh hưởng.', confirm: 'Xóa học liệu', danger: true, onConfirm: () => handle(deleteLesson(btn.dataset.sv, l.id), 'Đã xóa học liệu.') && ok() }); },
    'move-lesson': (btn) => { moveLesson(btn.dataset.sv, btn.dataset.id, Number(btn.dataset.dir)); render(); },
    'apply-sv': (btn) => { const v = sv(btn.dataset.id || btn.dataset.sv); const co = course(btn.dataset.course); const latest = latestPublished(courseVersionsOf(co.id)); const used = latest.stages.map(sv).find((x) => x.stageId === v.stageId);
      openModal({ title: `Áp dụng ${stage(v.stageId).name} ${versionLabel(v)} cho ${co.name}`, confirm: `Tạo và phát hành ${co.name} v${latest.no + 1}`, body: `<p>Trong một giao dịch, hệ thống sẽ:</p><ol style="margin:0;padding-left:20px;display:grid;gap:6px"><li>Nhân bản <strong>${esc(co.name)} ${versionLabel(latest)}</strong> thành <strong>v${latest.no + 1}</strong>.</li><li>Thay <strong>${esc(stage(v.stageId).name)} ${versionLabel(used)}</strong> bằng <strong>${versionLabel(v)}</strong>, giữ nguyên thứ tự và ${latest.stages.length - 1} chặng còn lại.</li><li>Phát hành v${latest.no + 1}.</li></ol><div class="alert alert-info">${icon('info')}<div class="alert-body">Các lớp đang chạy trên ${versionLabel(latest)} (${classesUsingCV(latest.id).map((c) => c.code).join(', ') || 'chưa có'}) không bị ảnh hưởng. Lớp mới hoặc lớp nháp mới gắn được v${latest.no + 1}.</div></div>`,
        onSubmit: () => { const r = applyStageVersion(v.id, co.id); if (r.ok) { closeModal(); toast(`Đã phát hành ${co.name} v${r.no} dùng ${versionLabel(v)}.`); render(); } return r; } }); },
    'new-course': () => openModal({ title: 'Tạo khóa học', confirm: 'Tạo khóa học', body: `<div class="form-grid">${field('Mã khóa học', '<input class="input" name="code" placeholder="vd: ADV" required>', { required: true })}${field('Tên khóa học', '<input class="input" name="name" placeholder="vd: Lập trình nâng cao" required>', { required: true })}</div><p class="small muted">Hệ thống tạo kèm phiên bản nháp v1 để bạn ghép chặng.</p>`, onSubmit: (d) => { const r = createCourse(d.code, d.name); if (r.ok) { closeModal(); toast('Đã tạo khóa học với bản nháp v1.'); location.hash = `#/admin/courses/${r.id}`; } return r; } }),
    'clone-cv': (btn) => handle(cloneCourse(btn.dataset.id), 'Đã tạo bản nháp mới (nhân bản nông).', () => { const d = draftOf(courseVersionsOf(cv(btn.dataset.id).courseId)); location.hash = `#/admin/courses/${d.courseId}?v=${d.id}`; }),
    'publish-cv': (btn) => { const v = cv(btn.dataset.id); const b = coursePublishBlocker(v); if (b) { toast(b, 'error'); return; } confirmAction({ title: `Phát hành ${course(v.courseId).name} ${versionLabel(v)}?`, text: `Sau khi phát hành, danh sách và thứ tự ${v.stages.length} chặng không sửa được. Lớp mới có thể gắn với phiên bản này.`, confirm: 'Phát hành', onConfirm: () => handle(publishCourse(v.id), `Đã phát hành ${versionLabel(v)}.`) && ok() }); },
    'archive-cv': (btn) => { const v = cv(btn.dataset.id); confirmAction({ title: `Lưu trữ ${versionLabel(v)}?`, text: 'Lớp mới không gắn được phiên bản lưu trữ; các lớp đang dùng vẫn hoạt động bình thường.', confirm: 'Lưu trữ', onConfirm: () => handle(archiveCourseVersion(v.id), 'Đã lưu trữ.') && ok() }); },
    'delete-cv': (btn) => { const v = cv(btn.dataset.id); const co = course(v.courseId); const refs = classesUsingCV(v.id); if (v.status !== 'draft' && refs.length) { toast(`Không xóa được: đang dùng bởi lớp ${refs.map((c) => c.code).join(', ')}.`, 'error'); return; }
      confirmAction({ title: `Xóa ${co.name} ${versionLabel(v)}?`, text: 'Không hoàn tác được. Các phiên bản chặng không bị ảnh hưởng.', confirm: 'Xóa phiên bản', danger: true, onConfirm: () => { const r = deleteCourseVersion(v.id); if (r.ok) { closeModal(); toast('Đã xóa.'); location.hash = r.courseGone ? '#/admin/courses' : `#/admin/courses/${co.id}`; render(); } return r; } }); },
    'add-stage': (btn) => { const v = cv(btn.dataset.cv); const usedStages = new Set(v.stages.map((id) => sv(id).stageId));
      const options = S.stages.filter((s) => !usedStages.has(s.id)).flatMap((s) => stageVersionsOf(s.id).filter((x) => x.status === 'published').map((x) => ({ value: x.id, label: `${s.name} ${versionLabel(x)}${x.id === latestPublished(stageVersionsOf(s.id)).id ? ' (mới nhất)' : ''}` })));
      if (!options.length) { toast('Không còn chặng đã phát hành nào để thêm.', 'error'); return; }
      openModal({ title: 'Thêm chặng vào khóa học', confirm: 'Thêm', body: field('Phiên bản chặng', `<select class="input" name="svId">${selectOpts(options, options[0].value)}</select>`, { help: 'Chỉ liệt kê phiên bản đã phát hành của các chặng chưa có trong khóa học.' }), onSubmit: (d) => handle(addStageToCourse(v.id, d.svId), 'Đã thêm chặng.') && ok() }); },
    'swap-stage': (btn) => handle(replaceStageInCourse(btn.dataset.cv, btn.dataset.old, btn.dataset.new), 'Đã đổi sang phiên bản mới.'),
    'remove-stage': (btn) => handle(removeStageFromCourse(btn.dataset.cv, btn.dataset.id), 'Đã gỡ chặng khỏi bản nháp.'),
    'move-stage': (btn) => { moveStageInCourse(btn.dataset.cv, btn.dataset.id, Number(btn.dataset.dir)); render(); },
    'new-class': () => { const published = S.courseVersions.filter((x) => x.status === 'published'); if (!published.length) { toast('Chưa có phiên bản khóa học nào được phát hành.', 'error'); return; }
      openModal({ title: 'Tạo lớp', confirm: 'Tạo lớp', body: `<div class="form-grid">${field('Mã lớp', '<input class="input" name="code" placeholder="vd: basic04" required>', { required: true })}${field('Tên lớp', '<input class="input" name="name" placeholder="vd: Lập trình cơ bản – khóa 4" required>', { required: true })}
        <div class="span-2">${field('Phiên bản khóa học', `<select class="input" name="courseVersionId">${selectOpts(published.map((x) => ({ value: x.id, label: `${course(x.courseId).name} ${versionLabel(x)}` })), published[published.length - 1].id)}</select>`, { required: true, help: 'Chỉ phiên bản đã phát hành. Đổi được khi lớp còn ở trạng thái nháp.' })}</div>
        ${field('Ngày bắt đầu dự kiến', `<input class="input" type="date" name="startDate" value="${new Date(Date.now() + 14 * 864e5).toISOString().slice(0, 10)}">`, { required: true })}${field('Ngày kết thúc dự kiến', `<input class="input" type="date" name="endDate" value="${new Date(Date.now() + 120 * 864e5).toISOString().slice(0, 10)}">`, { required: true })}
        <div class="span-2">${field('Giảng viên phụ trách', `<select class="input" name="teacherId">${selectOpts(S.users.filter((u) => u.role === 'teacher').map((u) => ({ value: u.id, label: u.name })))}</select>`, { required: true })}</div></div>`,
        onSubmit: (d) => { const r = createClass(d); if (r.ok) { closeModal(); toast('Đã tạo lớp ở trạng thái nháp.'); location.hash = `#/admin/classes/${r.id}`; } return r; } }); },
    'activate-class': (btn) => { const c = cls(btn.dataset.id); confirmAction({ title: `Kích hoạt lớp ${c.code}?`, text: `${activeMembersOf(c.id).length} học viên sẽ bắt đầu học và tích hoàn thành được. Sau khi kích hoạt, lớp không đổi được phiên bản khóa học.`, confirm: 'Kích hoạt', onConfirm: () => handle(setClassStatus(c.id, 'active'), `Lớp ${c.code} đang chạy.`) && ok() }); },
    'end-class': (btn) => { const c = cls(btn.dataset.id); confirmAction({ title: `Kết thúc lớp ${c.code}?`, text: 'Học viên chỉ còn xem học liệu, không tích hoàn thành được nữa. Không mời thêm học viên được. Không hoàn tác được.', confirm: 'Kết thúc lớp', danger: true, onConfirm: () => handle(setClassStatus(c.id, 'ended'), `Lớp ${c.code} đã kết thúc.`) && ok() }); },
    'invite': (btn) => { const c = cls(btn.dataset.id); openModal({ title: `Mời học viên vào ${c.code}`, confirm: 'Gửi lời mời', body: `<div class="stack-sm">${field('Email', '<input class="input" type="email" name="email" placeholder="hocvien@example.com" required>', { required: true, help: 'Email được chuẩn hóa (cắt khoảng trắng, chữ thường) trước khi so khớp.' })}${field('Họ tên', '<input class="input" name="name" placeholder="Bỏ trống nếu email đã có tài khoản">')}
      <div class="alert alert-info">${icon('info')}<div class="alert-body">Email mới: tạo tài khoản, gửi mật khẩu tạm hiệu lực 72 giờ. Email đã có tài khoản: chỉ thêm vào lớp và gửi thông báo. Mật khẩu tạm không bao giờ hiển thị trên giao diện này.</div></div></div>`,
      onSubmit: (d) => { const r = invite(c.id, d.email, d.name); if (r.ok) { closeModal(); toast(r.kind === 'added' ? `${r.user.name} đã có tài khoản: đã thêm vào lớp và gửi thông báo.` : `Đã tạo tài khoản cho ${r.user.name}, lời mời đang được gửi.`); render(); } return r; } }); },
    'resend': (btn) => { const u = user(btn.dataset.user); confirmAction({ title: `Gửi lại lời mời cho ${u.name}?`, text: 'Mật khẩu tạm cũ mất hiệu lực ngay; mật khẩu tạm mới có hiệu lực 72 giờ kể từ bây giờ.', confirm: 'Gửi lại', onConfirm: () => handle(resendInvite(btn.dataset.class, u.id), 'Lời mời mới đang được gửi.') && ok() }); },
    'remove-member': (btn) => { const m = byId(S.members, btn.dataset.id); const u = user(m.userId); confirmAction({ title: `Gỡ ${u.name} khỏi lớp?`, text: 'Thành viên chuyển sang trạng thái đã rời lớp; dữ liệu tiến độ được giữ nguyên.', confirm: 'Gỡ khỏi lớp', danger: true, onConfirm: () => handle(removeMember(m.id), `Đã gỡ ${u.name} khỏi lớp.`) && ok() }); },
    'disable-user': (btn) => { const u = user(btn.dataset.id); confirmAction({ title: `Vô hiệu hóa tài khoản ${u.email}?`, text: 'Tài khoản không đăng nhập được, phiên hiện tại bị hủy. Tiến độ học giữ nguyên.', confirm: 'Vô hiệu hóa', danger: true, onConfirm: () => handle(disableUser(u.id), 'Đã vô hiệu hóa tài khoản.') && ok() }); },
    'enable-user': (btn) => handle(enableUser(btn.dataset.id), 'Đã kích hoạt lại tài khoản.'),
    'student-detail': (btn) => openDrawer(studentDetailDrawer(btn.dataset.member)),
    'stage-summary': (btn) => openDrawer(stageSummaryDrawer(btn.dataset.member, btn.dataset.sv)),
    'run-daily-jobs': () => { const r = runDailyJobs(); toast(r.digests || r.reminders ? `Đã gửi ${r.digests} email tổng hợp chấm bài và ${r.reminders} email nhắc hạn nộp.` : 'Không có email mới: email hôm nay đã gửi rồi hoặc không có gì cần nhắc.'); render(); },
    'download-file': (btn) => toast(`Tải ${btn.dataset.name}: bản mẫu chỉ giả lập tải xuống.`),
    'start-quiz': (btn) => { if (btn.getAttribute('aria-disabled') === 'true') { toast(btn.title, 'error'); return; }
      const r = startAttempt(btn.dataset.class, btn.dataset.lesson); if (!r.ok) { toast(r.error, 'error'); return; }
      if (r.resumed) toast('Bạn đang có một lượt chưa nộp: tiếp tục lượt đó.'); go(`#/learn/classes/${btn.dataset.class}/lessons/${btn.dataset.lesson}`); },
    'submit-quiz': (btn) => { const left = [...document.querySelectorAll('.quiz-take .quiz-q')].filter((f) => !f.querySelector('input:checked')).length;
      confirmAction({ title: 'Nộp bài?', text: left ? `Bạn còn <strong>${left} câu chưa trả lời</strong>; các câu này tính 0 điểm. Sau khi nộp không sửa được nữa.` : 'Bạn đã trả lời tất cả câu hỏi. Sau khi nộp không sửa được nữa.', confirm: 'Nộp bài',
        onConfirm: () => { const r = submitAttempt(btn.dataset.id); if (r.ok) { toast(r.late ? 'Đã quá giờ: lượt được chấm với các câu đã lưu trước hạn.' : 'Đã nộp bài.', r.late ? 'warn' : 'ok'); go(`#/learn/classes/${btn.dataset.class}/lessons/${btn.dataset.lesson}?result=${btn.dataset.id}`); } return r; } }); },
    'set-deadline': (btn) => { if (btn.getAttribute('aria-disabled') === 'true') { toast(btn.title, 'error'); return; }
      const c = cls(btn.dataset.class); const l = lessonById(btn.dataset.lesson); const cur = deadlineOf(c.id, l.id);
      openModal({ title: `Hạn nộp: ${shortTitle(l)}`, confirm: 'Lưu hạn nộp', body: `<div class="stack-sm"><p class="small muted">Lớp ${esc(c.code)}. Hạn nộp chỉ áp dụng cho lớp này. Học viên chưa nộp nhận email nhắc 24 giờ trước hạn.</p>
        ${field('Hạn nộp', `<input class="input" type="datetime-local" name="due" value="${toLocalInput(cur)}">`, { required: true, help: 'Theo giờ máy của bạn. Nộp sau hạn vẫn được nhận nhưng đánh dấu nộp muộn.' })}
        ${cur ? '<label class="check"><input type="checkbox" name="clear"> Bỏ hạn nộp</label>' : ''}</div>`,
        onSubmit: (d) => { const clear = d.clear === 'on'; if (!clear && !d.due) return err('Chọn ngày giờ hạn nộp.'); return handle(setDeadline(c.id, l.id, clear ? null : d.due), clear ? 'Đã bỏ hạn nộp.' : 'Đã lưu hạn nộp.') && ok(); } }); },
    'grant-attempts': (btn) => { if (btn.getAttribute('aria-disabled') === 'true') { toast(btn.title, 'error'); return; }
      const m = byId(S.members, btn.dataset.member); const l = lessonById(btn.dataset.lesson); const stu = user(m.userId);
      openModal({ title: `Cấp thêm lượt cho ${stu.name}`, confirm: 'Cấp thêm lượt', body: `<div class="stack-sm"><p class="small muted">${esc(shortTitle(l))} · đã dùng ${attemptsOf(m.id, l.id).length}/${attemptLimit(m.id, l)} lượt.</p>
        ${field('Số lượt cấp thêm', `<select class="input" name="extra">${selectOpts([1, 2, 3].map((n) => ({ value: String(n), label: `${n} lượt` })), '1')}</select>`)}
        ${field('Lý do', '<textarea class="input input-prose" name="reason" rows="3" required placeholder="vd: Mất kết nối khi đang làm bài"></textarea>', { required: true, help: 'Lưu vào lịch sử cấp lượt của lớp.' })}</div>`,
        onSubmit: (d) => handle(grantAttempts(m.id, l.id, d.extra, d.reason), `Đã cấp thêm ${d.extra} lượt cho ${stu.name}.`) && ok() }); },
    'new-question': (btn) => openModal({ title: 'Thêm câu hỏi', confirm: 'Thêm câu hỏi', body: questionForm(), onSubmit: (d) => handle(saveQuestion(btn.dataset.sv, btn.dataset.lesson, null, d), 'Đã thêm câu hỏi.') && ok() }),
    'edit-question': (btn) => { const x = lessonById(btn.dataset.lesson).quiz.questions.find((y) => y.id === btn.dataset.id); openModal({ title: 'Sửa câu hỏi', body: questionForm(x), onSubmit: (d) => handle(saveQuestion(btn.dataset.sv, btn.dataset.lesson, x.id, d), 'Đã lưu câu hỏi.') && ok() }); },
    'delete-question': (btn) => confirmAction({ title: 'Xóa câu hỏi?', text: 'Câu hỏi bị xóa khỏi bản nháp này. Phiên bản đã phát hành không bị ảnh hưởng.', confirm: 'Xóa câu hỏi', danger: true, onConfirm: () => handle(deleteQuestion(btn.dataset.sv, btn.dataset.lesson, btn.dataset.id), 'Đã xóa câu hỏi.') && ok() }),
    'move-question': (btn) => handle(moveQuestion(btn.dataset.sv, btn.dataset.lesson, btn.dataset.id, Number(btn.dataset.dir))),
    'new-criterion': (btn) => openModal({ title: 'Thêm tiêu chí chấm', confirm: 'Thêm tiêu chí', body: criterionForm(), onSubmit: (d) => handle(saveCriterion(btn.dataset.sv, btn.dataset.lesson, null, d), 'Đã thêm tiêu chí.') && ok() }),
    'edit-criterion': (btn) => { const c = lessonById(btn.dataset.lesson).homework.criteria.find((y) => y.id === btn.dataset.id); openModal({ title: 'Sửa tiêu chí chấm', body: criterionForm(c), onSubmit: (d) => handle(saveCriterion(btn.dataset.sv, btn.dataset.lesson, c.id, d), 'Đã lưu tiêu chí.') && ok() }); },
    'delete-criterion': (btn) => confirmAction({ title: 'Xóa tiêu chí?', text: 'Tiêu chí bị xóa khỏi bản nháp này; tổng điểm bài tập giảm theo.', confirm: 'Xóa tiêu chí', danger: true, onConfirm: () => handle(deleteCriterion(btn.dataset.sv, btn.dataset.lesson, btn.dataset.id), 'Đã xóa tiêu chí.') && ok() }),
    'move-criterion': (btn) => handle(moveCriterion(btn.dataset.sv, btn.dataset.lesson, btn.dataset.id, Number(btn.dataset.dir))),
    'complete': (btn) => { if (btn.getAttribute('aria-disabled') === 'true') { toast(btn.title, 'error'); return; } handle(setComplete(btn.dataset.class, btn.dataset.lesson, btn.dataset.done === '1'), btn.dataset.done === '1' ? 'Đã ghi nhận hoàn thành.' : 'Đã bỏ tích.'); }
  };
  const FORMS = {
    login: (form) => { const r = login(form.email.value, form.password.value); if (!r.ok) { formError(form, r.error); return; } location.hash = r.mustChange ? '#/first-login' : homeFor(me()); render(); },
    forgot: (form) => { form.querySelector('[data-form-ok]').hidden = false; form.email.value = ''; },
    'first-login': (form) => { const r = changePassword(form.password.value, form.confirm.value); if (!r.ok) { formError(form, r.error); return; } toast('Đã lưu mật khẩu. Chào mừng bạn vào lớp.'); location.hash = homeFor(me()); render(); },
    'class-settings': (form) => { const d = inputs(form); const c = cls(form.dataset.id); if (!('courseVersionId' in d)) d.courseVersionId = c.courseVersionId; const r = updateClass(c.id, d); if (!r.ok) { formError(form, r.error); return; } toast('Đã lưu cài đặt lớp.'); render(); },
    'summary-filter': (form) => { const d = inputs(form); location.hash = withQuery(form.dataset.base, { incomplete: d.incomplete, nocomment: d.nocomment, rework: d.rework ? 1 : '', overdue: d.overdue ? 1 : '' }); },
    'grading-filter': (form) => { const d = inputs(form); location.hash = withQuery(form.dataset.base, { class: d.class, lesson: d.lesson, overdue: d.overdue ? 1 : '' }); },
    grade: (form) => { const id = form.dataset.id; const r = gradeSubmission(id, inputs(form)); if (!r.ok) { formError(form, r.error); return; }
      const msg = `Đã lưu ${r.total}/${r.max} · ${r.result === 'passed' ? 'Đạt' : 'Cần làm lại'}. Email kết quả đã gửi cho học viên.`;
      const next = r.edit ? null : gradingQueue(me())[0];
      if (r.edit) { toast(msg); render(); } else if (next) { toast(`${msg} Chuyển sang bài tiếp theo.`); go(gradeHref(next.s.id)); } else { toast(`${msg} Đã chấm hết hàng chờ.`); go(me().role === 'admin' ? '#/admin/grading' : '#/teach/grading'); } },
    'submit-homework': (form) => { const d = inputs(form); const files = [...form.querySelector('input[type=file]').files].map((f) => ({ name: f.name, size: f.size }));
      const r = submitHomework(form.dataset.class, form.dataset.lesson, { repoUrl: d.repoUrl, commitRef: d.commitRef, files }); if (!r.ok) { formError(form, r.error); return; }
      toast(`Đã nộp lần #${r.no}${r.late ? ' (nộp muộn)' : ''}${r.replaced ? `, thay lần #${r.replaced} đang chờ chấm` : ''}. Bạn sẽ nhận email khi có kết quả.`); render(); },
    'stage-review': (form) => { const r = saveStageReview(form.dataset.member, form.dataset.sv, form.text.value); if (!r.ok) { formError(form, r.error); return; }
      toast('Đã lưu nhận xét. Học viên thấy nhận xét trong trang tổng kết.'); render(); openDrawer(stageSummaryDrawer(form.dataset.member, form.dataset.sv)); },
    'report-filter': (form) => { const d = inputs(form); location.hash = form.dataset.base + q({ notlogged: d.notlogged ? 1 : '', inactive: d.inactive, below: d.below, sort: d.sort }).replace('?', form.dataset.base.includes('?') ? '&' : '?'); },
    modal: (form) => { if (!modalHandler) return; const r = modalHandler(inputs(form)); if (r && r.ok === false) modalError(r.error); }
  };
  const withQuery = (base, obj) => base + q(obj).replace('?', base.includes('?') ? '&' : '?');
  function formError(form, msg) { const box = form.querySelector('[data-form-error]'); if (box) { box.hidden = false; box.querySelector('.alert-body').textContent = msg; } else toast(msg, 'error'); }

  document.addEventListener('click', (e) => {
    const a = e.target.closest('[data-action]');
    if (a) { if (a.tagName === 'A' && !a.dataset.action) return; e.preventDefault(); const fn = ACTIONS[a.dataset.action]; if (fn) fn(a); return; }
    const row = e.target.closest('tr[data-href]'); if (row && !e.target.closest('a, button')) { location.hash = row.dataset.href; return; }
    const d = $('#drawer'); if (d.hasAttribute('open') && e.target.classList.contains('scrim')) closeDrawer();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && $('#drawer').hasAttribute('open')) closeDrawer();
    if (e.key === 'Enter' && e.target.matches('tr[data-action]')) { e.preventDefault(); ACTIONS[e.target.dataset.action](e.target); }
    if (e.key === 'Enter' && e.target.matches('tr[data-href]')) { e.preventDefault(); location.hash = e.target.dataset.href; }
  });
  document.addEventListener('submit', (e) => { const form = e.target.closest('form[data-form]'); if (!form) return; e.preventDefault(); const fn = FORMS[form.dataset.form]; if (fn) fn(form); });
  document.addEventListener('change', (e) => {
    const el = e.target.closest('[data-change]'); if (!el) return;
    if (el.dataset.change === 'switch-role') { switchRole(el.value); location.hash = homeFor(me()); render(); }
    if (el.dataset.change === 'lesson-type') el.closest('.modal-body').querySelectorAll('[data-lesson-section]').forEach((x) => { x.hidden = x.dataset.lessonSection !== el.value; });
    if (el.dataset.change === 'quiz-mode') el.closest('.modal-body').querySelector('[data-quiz-limits]').hidden = el.value !== 'assessment';
    if (el.dataset.change === 'quiz-answer') {
      const fs = el.closest('fieldset'); const oids = [...fs.querySelectorAll('input:checked')].map((x) => x.value);
      const r = saveAnswer(el.dataset.attempt, el.dataset.q, oids); if (!r.ok) { toast(r.error, 'error'); render(); return; }
      const saved = $('[data-saved]'); if (saved) saved.textContent = `Đã lưu ${new Date(r.savedAt).toTimeString().slice(0, 8)}`;
      const ans = $('[data-answered]'); if (ans) ans.textContent = `${[...document.querySelectorAll('.quiz-take .quiz-q')].filter((f) => f.querySelector('input:checked')).length}/${ans.dataset.total}`;
    }
    if (el.dataset.change === 'toggle-complete') { const r = setComplete(el.dataset.class, el.dataset.lesson, el.checked); if (!r.ok) { el.checked = !el.checked; toast(r.error, 'error'); } else render(); }
  });
  document.addEventListener('input', (e) => { if (e.target.matches('[data-grade-score]')) updateGradeTotal(e.target.closest('form')); });
  $('#modal').addEventListener('close', () => { modalHandler = null; });
  window.addEventListener('hashchange', render);

  load(); render();
})();
