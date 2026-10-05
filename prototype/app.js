/* GoUp LMS prototype — single-page app.
   State lives in memory and is mirrored to localStorage so reloads keep your walk-through.
   Nothing here talks to a server; every rule from the spec is enforced in the functions
   under "Domain rules" so the prototype behaves like the real system would. */
(function () {
  'use strict';

  const STORE_KEY = 'goup-lms-prototype-v1';
  const TEMP_PASSWORD_HOURS = 72;
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
    upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/>'
  };
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
      const done = req.filter((l) => progressOf(member.id, l.id)?.completedAt).length;
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
      lessons: from.lessons.map((l) => ({ ...l, id: uid('l-') })) }; // same lesson_key, video reference copied not the file
    S.stageVersions.push(v); log('Nhân bản chặng', `${stage(from.stageId).name} ${versionLabel(from)} → ${versionLabel(v)}`); save(); return ok({ id: v.id });
  }
  function publishStage(svId) {
    const v = sv(svId); if (v.status !== 'draft') return err('Chỉ phát hành được bản nháp.');
    if (!v.lessons.length) return err('Chặng cần ít nhất một học liệu trước khi phát hành.');
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
    const title = data.title.trim(); if (!title) return err('Nhập tiêu đề học liệu.');
    if (data.type === 'video' && !data.video.trim()) return err('Học liệu video bắt buộc có file video.');
    if (data.type === 'markdown' && !data.content.trim()) return err('Học liệu markdown bắt buộc có nội dung.');
    const base = { title, type: data.type, required: !!data.required, video: data.type === 'video' ? data.video.trim() : undefined, duration: data.type === 'video' ? (data.duration || '—') : undefined, content: data.type === 'markdown' ? data.content : undefined };
    if (lessonId) { const l = v.lessons.find((x) => x.id === lessonId); Object.assign(l, base); }
    else v.lessons.push({ id: uid('l-'), key: uid('k-'), ...base });
    save(); return ok();
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
    const p = S.progress[m.id + ':' + lessonId]; if (!p) return err('Mở học liệu trước khi tích hoàn thành.');
    p.completedAt = done ? (p.completedAt || nowISO()) : null; u.lastActiveAt = nowISO(); save(); return ok();
  }

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
    { re: /^\/admin\/courses$/, page: pageCourses, roles: ['admin'] },
    { re: /^\/admin\/courses\/([^/]+)$/, page: pageCourseDetail, roles: ['admin'] },
    { re: /^\/admin\/classes$/, page: pageClasses, roles: ['admin'] },
    { re: /^\/admin\/classes\/([^/]+)$/, page: pageClassDetail, roles: ['admin'] },
    { re: /^\/teach$/, page: pageTeachClasses, roles: ['teacher'] },
    { re: /^\/teach\/classes\/([^/]+)$/, page: pageTeachClass, roles: ['teacher'] },
    { re: /^\/learn$/, page: pageLearn, roles: ['student'] },
    { re: /^\/learn\/classes\/([^/]+)$/, page: pageLearnClass, roles: ['student'] },
    { re: /^\/learn\/classes\/([^/]+)\/lessons\/([^/]+)$/, page: pageLesson, roles: ['student'] }
  ];
  const NAV = {
    admin: [['#/admin', 'Tổng quan'], ['#/admin/stages', 'Chặng'], ['#/admin/courses', 'Khóa học'], ['#/admin/classes', 'Lớp học']],
    teacher: [['#/teach', 'Lớp của tôi']],
    student: [['#/learn', 'Lớp của tôi']]
  };

  function render() {
    closeModal(); closeDrawer();
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
    const navItems = NAV[u.role].map(([href, label]) => { const cur = path === href.slice(1) || (href !== homeFor(u) && path.startsWith(href.slice(1) + '/')); return `<a href="${href}" ${cur ? 'aria-current="page"' : ''}>${label}</a>`; }).join('');
    const roleOpts = [['u-admin', 'Admin · Trần Minh Quân'], ['u-gv', 'Giảng viên · Lê Thu Hương'], ['u-gv2', 'Giảng viên · Phạm Quốc Bảo'], ['u-an', 'Học viên · Nguyễn Hoàng An'], ['u-phong', 'Học viên · Đặng Hải Phong']]
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

  function pageAdminDashboard() {
    const running = S.classes.filter((c) => c.status === 'active');
    const learners = new Set(running.flatMap((c) => activeMembersOf(c.id).map((m) => m.userId))).size;
    const failed = S.invitations.filter((i) => i.status === 'failed').length;
    const outdated = S.stages.flatMap((s) => outdatedCourses(s.id).map((o) => ({ ...o, stage: s })));
    const notLogged = S.members.filter((m) => m.status === 'active' && user(m.userId).status === 'invited').length;
    const alerts = [
      ...outdated.map((o) => `<div class="alert alert-warn">${icon('alert')}<div class="alert-body"><strong>${esc(o.course.name)} ${versionLabel(o.cv)}</strong> vẫn dùng ${esc(o.stage.name)} ${versionLabel(o.used)} trong khi ${versionLabel(o.latest)} đã phát hành.<div class="alert-actions"><a class="un-btn un-btn-secondary un-btn-sm" href="#/admin/stages/${o.stage.id}?v=${o.latest.id}">Xem và áp dụng</a></div></div></div>`),
      ...(failed ? [`<div class="alert alert-danger">${icon('mail')}<div class="alert-body"><strong>${failed} lời mời gửi thất bại.</strong> Kiểm tra email học viên rồi gửi lại trong trang lớp.</div></div>`] : [])
    ].join('');
    return { title: 'Tổng quan', html: `${pageHead('Tổng quan', { lede: 'Tình trạng các lớp đang chạy và việc cần xử lý.' })}
      <div class="stack">
        ${alerts ? `<div class="stack-sm">${alerts}</div>` : ''}
        ${scenarioGuide()}
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
            <ul class="list audit">${S.audit.slice(0, 8).map((a) => `<li><div class="grow"><div class="title">${esc(a.action)}</div><div class="meta">${esc(a.target)} · ${esc(user(a.by)?.name || '')}</div></div><span class="when">${rel(a.at)}</span></li>`).join('')}</ul></section>
        </div>
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
    const actions = isDraft
      ? `<button class="un-btn un-btn-secondary" data-action="delete-sv" data-id="${v.id}">${icon('trash')} Xóa bản nháp</button><button class="un-btn un-btn-primary" data-action="publish-sv" data-id="${v.id}" ${v.lessons.length ? '' : 'aria-disabled="true" title="Cần ít nhất một học liệu"'}>${icon('check')} Phát hành ${versionLabel(v)}</button>`
      : `${v.status === 'published' ? `<button class="un-btn un-btn-secondary" data-action="archive-sv" data-id="${v.id}">${icon('archive')} Lưu trữ</button>` : ''}<button class="un-btn un-btn-secondary" data-action="delete-sv" data-id="${v.id}" ${deleteBlock ? `aria-disabled="true" title="${esc(deleteBlock)}"` : ''}>${icon('trash')} Xóa</button>${draft ? `<a class="un-btn un-btn-navy" href="${href(draft)}">${icon('pencil')} Mở bản nháp ${versionLabel(draft)}</a>` : `<button class="un-btn un-btn-navy" data-action="clone-sv" data-id="${v.id}">${icon('copy')} Nhân bản thành bản nháp</button>`}`;
    const lessons = v.lessons.length ? v.lessons.map((l, i) => `<li>
        <span class="ord">${i + 1}</span><span class="type-icon" title="${l.type === 'video' ? 'Video' : 'Markdown'}">${icon(l.type === 'video' ? 'play' : 'file-text')}</span>
        <div class="grow"><div class="title">${esc(l.title)}</div><div class="meta"><span>${l.type === 'video' ? `Video · ${esc(l.video)} · ${esc(l.duration || '')}` : `Markdown · ${(l.content || '').length} ký tự`}</span>${l.required ? '' : '<span class="badge badge-optional">Không bắt buộc</span>'}<span class="muted">key ${esc(l.key)}</span></div></div>
        ${isDraft ? `<div class="inline-actions"><button class="icon-btn" data-action="move-lesson" data-sv="${v.id}" data-id="${l.id}" data-dir="-1" aria-label="Chuyển lên" ${i === 0 ? 'disabled' : ''}>${icon('chevron-up')}</button><button class="icon-btn" data-action="move-lesson" data-sv="${v.id}" data-id="${l.id}" data-dir="1" aria-label="Chuyển xuống" ${i === v.lessons.length - 1 ? 'disabled' : ''}>${icon('chevron-down')}</button><button class="icon-btn" data-action="edit-lesson" data-sv="${v.id}" data-id="${l.id}" aria-label="Sửa">${icon('pencil')}</button><button class="icon-btn" data-action="delete-lesson" data-sv="${v.id}" data-id="${l.id}" aria-label="Xóa">${icon('trash')}</button></div>` : ''}
      </li>`).join('') : `<li>${empty('Chưa có học liệu', isDraft ? 'Thêm video hoặc bài markdown để phát hành chặng này.' : 'Phiên bản này không có học liệu.')}</li>`;
    const outdatedHtml = outdated.length ? `<section class="card"><div class="card-head"><h2>Khóa học đang dùng phiên bản cũ của chặng này</h2></div><ul class="list">${outdated.map((o) => `<li><div class="grow"><a class="title" href="#/admin/courses/${o.course.id}?v=${o.cv.id}">${esc(o.course.name)} ${versionLabel(o.cv)}</a><div class="meta"><span>Đang dùng ${esc(s.name)} ${versionLabel(o.used)}</span>${o.blocked ? `<span class="badge badge-warn">${esc(o.blocked)}</span>` : ''}</div></div><button class="un-btn un-btn-primary un-btn-sm" data-action="apply-sv" data-sv="${v.id}" data-course="${o.course.id}" ${o.blocked ? 'aria-disabled="true"' : ''}>Áp dụng ${versionLabel(v)}</button></li>`).join('')}</ul></section>` : (latest && v.id === latest.id && S.courses.length ? `<div class="alert alert-ok">${icon('check-circle')}<div class="alert-body">Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này.</div></div>` : '');
    const usedHtml = `<section class="card"><div class="card-head"><h2>Đang được dùng ở</h2></div>${refs.length ? `<ul class="list">${refs.map((c) => `<li><div class="grow"><a class="title" href="#/admin/courses/${c.courseId}?v=${c.id}">${esc(course(c.courseId).name)} ${versionLabel(c)}</a><div class="meta">${badge(c.status)}<span>${classesUsingCV(c.id).map((x) => x.code).join(', ') || 'chưa có lớp'}</span></div></div></li>`).join('')}</ul>` : `<div class="card-body muted small">Chưa có khóa học nào dùng ${versionLabel(v)}.${classes.length ? '' : ''}</div>`}</section>`;
    return { title: `${s.name} ${versionLabel(v)}`, html: `${pageHead(s.name, { crumbsHtml: crumbs([['Chặng', '#/admin/stages'], [s.name]]), badges: `<span class="muted">${esc(s.code)}</span>`, lede: 'Phiên bản đã phát hành là bất biến; mọi thay đổi đi qua một bản nháp mới rồi được áp dụng cho khóa học bằng một thao tác.' })}
      <div class="stack">
        <div class="card card-pad row-between"><div class="row"><span class="section-label">Phiên bản</span>${lineage(vs, href, v.id)}</div><div class="row small muted">${v.status === 'published' ? `Phát hành ${fmtDateTime(v.publishedAt)}` : v.status === 'draft' ? (v.clonedFrom ? `Nhân bản từ ${versionLabel(sv(v.clonedFrom))}` : 'Bản nháp đầu tiên') : 'Đã lưu trữ'}</div></div>
        <section class="card"><div class="card-head"><div class="row"><h2>Học liệu của ${versionLabel(v)}</h2>${badge(v.status)}</div><div class="row">${isDraft ? `<button class="un-btn un-btn-secondary un-btn-sm" data-action="new-lesson" data-sv="${v.id}">${icon('plus')} Thêm học liệu</button>` : `<span class="small muted">${icon('lock')} Không sửa được</span>`}</div></div><ul class="list">${lessons}</ul>
          <div class="complete-bar"><span class="small muted">${v.lessons.filter((l) => l.required).length} bắt buộc · ${v.lessons.filter((l) => !l.required).length} tùy chọn</span><div class="row">${actions}</div></div></section>
        ${outdatedHtml}
        ${usedHtml}
      </div>` };
  }

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
    const tab = ['students', 'report', 'settings'].includes(query.tab) ? query.tab : 'students';
    const v = cv(c.courseVersionId); const base = `#/admin/classes/${c.id}`;
    const lifecycle = c.status === 'draft' ? `<button class="un-btn un-btn-primary" data-action="activate-class" data-id="${c.id}">${icon('play')} Kích hoạt lớp</button>` : c.status === 'active' ? `<button class="un-btn un-btn-secondary" data-action="end-class" data-id="${c.id}">Kết thúc lớp</button>` : '';
    const tabs = `<nav class="tabs" aria-label="Mục của lớp">${[['students', 'Học viên', activeMembersOf(c.id).length], ['report', 'Tiến độ'], ['settings', 'Cài đặt']].map(([k, l, n]) => `<a href="${base}?tab=${k}" ${tab === k ? 'aria-current="page"' : ''}>${l}${n != null ? `<span class="count">${n}</span>` : ''}</a>`).join('')}</nav>`;
    let body;
    if (tab === 'students') body = renderStudentsTab(c);
    else if (tab === 'report') body = renderReport(c, query, `${base}?tab=report`);
    else body = renderSettingsTab(c);
    return { title: c.code, html: `${pageHead(`${c.code} · ${c.name}`, { crumbsHtml: crumbs([['Lớp học', '#/admin/classes'], [c.code]]), badges: `<span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>`, lede: `${esc(course(v.courseId).name)} ${versionLabel(v)} · ${fmtDate(c.startDate)} → ${fmtDate(c.endDate)} · Giảng viên ${esc(user(c.teacherId)?.name || '—')}`, actions: lifecycle })}${tabs}${body}` };
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
    const sortLink = (key, label, cls = '') => { const active = f.sort.startsWith(key); const next = active && f.sort === key ? key + (key === 'pct' ? '-desc' : '-asc') : key; return `<th class="${cls}"><a class="sort" style="color:inherit;text-decoration:none;display:inline-flex;align-items:center;gap:4px" href="${basePath}${q({ ...f, notlogged: f.notlogged ? 1 : '', sort: next }).replace('?', '&')}" ${active ? 'aria-sort="other"' : ''}>${label} ${icon('sort')}</a></th>`; };
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
      <div class="card-head" style="border-bottom:0;padding-bottom:0"><span class="small muted">${rows.length}/${total} học viên · Bấm vào một dòng để xem từng học liệu</span><span class="badge badge-warn">${icon('info')} Tiến độ do học viên tự xác nhận</span></div>
      <div class="table-wrap"><table><thead>${thead}</thead><tbody>${body || `<tr><td colspan="${4 + stagesCols.length}">${anyFilter ? empty('Không có học viên khớp bộ lọc', 'Nới lỏng điều kiện hoặc xóa bộ lọc.', `<a class="un-btn un-btn-secondary un-btn-sm" href="${basePath}">Xóa bộ lọc</a>`) : empty('Chưa có học viên', 'Mời học viên vào lớp để theo dõi tiến độ.')}</td></tr>`}</tbody></table></div></div>`;
  }
  function studentDetailDrawer(memberId) {
    const m = byId(S.members, memberId); const u = user(m.userId); const c = cls(m.classId); const v = cv(c.courseVersionId); const p = memberProgress(m);
    const body = v.stages.map((id, i) => { const s = sv(id); const st = stage(s.stageId); const sp = p.stages[i];
      return `<section><div class="card-head" style="background:var(--surface-50)"><h2>${i + 1}. ${esc(st.name)} <span class="muted small">${versionLabel(s)}</span></h2><div style="width:140px">${progressBar(sp.pct)}</div></div><ul class="list">${s.lessons.map((l) => { const pr = progressOf(m.id, l.id);
        return `<li><span class="type-icon">${icon(l.type === 'video' ? 'play' : 'file-text')}</span><div class="grow"><div class="title">${esc(l.title)} ${l.required ? '' : '<span class="badge badge-optional">Không bắt buộc</span>'}</div><div class="meta"><span>${pr?.firstOpenedAt ? `Mở lần đầu ${fmtDateTime(pr.firstOpenedAt)}` : 'Chưa mở'}</span>${pr?.completedAt ? `<span class="dot dot-ok">Tích ${fmtDateTime(pr.completedAt)}</span>` : ''}</div></div>${pr?.completedAt ? icon('check-circle', 'done-mark') : ''}</li>`; }).join('')}</ul></section>`; }).join('');
    return `<div class="modal-head"><div><h2>${esc(u.name)}</h2><div class="small muted">${esc(u.email)} · ${c.code} · hoạt động cuối ${rel(u.lastActiveAt)}</div></div><button class="icon-btn" data-action="close-drawer" aria-label="Đóng">${icon('x')}</button></div>
      <div class="card-pad" style="border-bottom:1px solid var(--line)">${progressBar(p.pct, true)}<div class="small muted" style="margin-top:8px">${p.done}/${p.total} học liệu bắt buộc · tiến độ do học viên tự xác nhận</div></div>
      <div class="panel-body">${body}</div>`;
  }

  /* -------------------------------------------------- teacher pages */
  function teacherClassCards(classes, hrefFor) {
    return classes.length ? `<div class="grid-3">${classes.map((c) => { const v = cv(c.courseVersionId); const ms = activeMembersOf(c.id); const stale = c.status === 'active' ? ms.filter((m) => daysSince(user(m.userId).lastActiveAt) > 7).length : 0;
      return `<a class="card card-pad" href="${hrefFor(c)}" style="text-decoration:none;display:block"><div class="row-between"><h2>${esc(c.code)}</h2><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span></div><p class="muted small" style="margin:4px 0 16px">${esc(c.name)} · ${esc(course(v.courseId).name)} ${versionLabel(v)}</p>${c.status === 'draft' ? `<p class="small muted">Bắt đầu ${fmtDate(c.startDate)}</p>` : progressBar(classAvgProgress(c.id))}<div class="row small muted" style="margin-top:12px"><span>${ms.length} học viên</span>${stale ? `<span class="dot dot-invited">${stale} lâu không hoạt động</span>` : ''}</div></a>`; }).join('')}</div>` : `<div class="card">${empty('Chưa có lớp', 'Bạn chưa được phân công lớp nào.')}</div>`;
  }
  function pageTeachClasses() {
    const u = me(); const classes = S.classes.filter((c) => c.teacherId === u.id);
    return { title: 'Lớp của tôi', html: `${pageHead('Lớp của tôi', { lede: 'Các lớp bạn phụ trách. Vào lớp để xem tiến độ từng học viên theo chặng.' })}${teacherClassCards(classes, (c) => `#/teach/classes/${c.id}`)}` };
  }
  function pageTeachClass([classId], query) {
    const c = cls(classId); const u = me();
    if (!c || c.teacherId !== u.id) { toast('Bạn chỉ xem được lớp mình phụ trách.', 'error'); location.hash = '#/teach'; return null; }
    const v = cv(c.courseVersionId);
    return { title: c.code, html: `${pageHead(`${c.code} · ${c.name}`, { crumbsHtml: crumbs([['Lớp của tôi', '#/teach'], [c.code]]), badges: `<span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>`, lede: `${esc(course(v.courseId).name)} ${versionLabel(v)} · ${fmtDate(c.startDate)} → ${fmtDate(c.endDate)}` })}${renderReport(c, query, `#/teach/classes/${c.id}`)}` };
  }

  /* -------------------------------------------------- student pages */
  function pageLearn() {
    const u = me(); const ms = S.members.filter((m) => m.userId === u.id && m.status === 'active');
    const cards = ms.map((m) => { const c = cls(m.classId); const v = cv(c.courseVersionId); const p = memberProgress(m);
      const next = c.status === 'active' ? v.stages.map(sv).flatMap((s) => s.lessons).find((l) => !progressOf(m.id, l.id)?.completedAt) : null;
      return `<div class="card card-pad"><div class="row-between"><h2>${esc(c.name)}</h2><span class="badge badge-${c.status}">${classStatusVi[c.status]}</span></div><p class="muted small" style="margin:4px 0 16px">${esc(c.code)} · ${esc(course(v.courseId).name)} · ${fmtDate(c.startDate)} → ${fmtDate(c.endDate)}</p>
        ${progressBar(p.pct, true)}<p class="small muted" style="margin-top:6px">${p.done}/${p.total} học liệu bắt buộc</p>
        <div class="row" style="margin-top:16px">${c.status === 'draft' ? `<span class="small muted">Lớp bắt đầu ${fmtDate(c.startDate)}. Bạn sẽ vào học được khi lớp kích hoạt.</span>` : `<a class="un-btn ${next ? 'un-btn-primary' : 'un-btn-navy'}" href="${next ? `#/learn/classes/${c.id}/lessons/${next.id}` : `#/learn/classes/${c.id}`}">${next ? 'Học tiếp' : 'Xem lộ trình'}</a><a class="un-btn un-btn-ghost" href="#/learn/classes/${c.id}">Lộ trình</a>`}</div></div>`; }).join('');
    return { title: 'Lớp của tôi', html: `${pageHead(`Xin chào, ${u.name.split(' ').pop()}`, { lede: 'Các lớp bạn đang tham gia.' })}${cards ? `<div class="grid-2">${cards}</div>` : `<div class="card">${empty('Chưa có lớp', 'Khi được mời vào lớp, lớp sẽ hiện ở đây.')}</div>`}` };
  }
  function pageLearnClass([classId]) {
    const u = me(); const c = cls(classId); const m = c && memberOf(c.id, u.id);
    if (!m || m.status !== 'active') { location.hash = '#/learn'; return null; }
    const v = cv(c.courseVersionId); const p = memberProgress(m); const canTick = c.status === 'active';
    const stagesHtml = v.stages.map((id, i) => { const s = sv(id); const st = stage(s.stageId); const sp = p.stages[i];
      return `<section class="card stage"><div class="stage-head"><span class="ord">${i + 1}</span><div class="grow"><h2>${esc(st.name)}</h2><div class="small muted">${s.lessons.length} học liệu · ${sp.done}/${sp.total} bắt buộc đã xong</div></div>${progressBar(sp.pct)}</div>
        ${s.lessons.map((l) => { const pr = progressOf(m.id, l.id); const done = !!pr?.completedAt; const opened = !!pr; const disabled = !canTick || !opened;
          const why = !canTick ? (c.status === 'ended' ? 'Lớp đã kết thúc' : 'Lớp chưa bắt đầu') : !opened ? 'Mở học liệu trước khi tích' : '';
          return `<div class="lesson-row${done ? ' is-done' : ''}"><a class="lesson-link" href="#/learn/classes/${c.id}/lessons/${l.id}"><span class="type-icon">${icon(l.type === 'video' ? 'play' : 'file-text')}</span><span><span class="t">${esc(l.title)}</span><br><span class="sub">${l.type === 'video' ? `Video · ${esc(l.duration || '')}` : 'Bài đọc'}${l.required ? '' : ' · không bắt buộc'}${opened && !done ? ' · đã mở' : ''}</span></span></a>
            <label class="check${disabled ? ' is-disabled' : ''}" title="${esc(why)}"><input type="checkbox" data-change="toggle-complete" data-class="${c.id}" data-lesson="${l.id}" ${done ? 'checked' : ''} ${disabled ? 'disabled' : ''} aria-label="Đã học xong: ${esc(l.title)}"><span class="small">Đã học xong</span></label></div>`; }).join('')}</section>`; }).join('');
    return { title: c.code, html: `${pageHead(c.name, { crumbsHtml: crumbs([['Lớp của tôi', '#/learn'], [c.code]]), badges: `<span class="badge badge-${c.status}">${classStatusVi[c.status]}</span>`, lede: `${esc(course(v.courseId).name)} ${versionLabel(v)} · Giảng viên ${esc(user(c.teacherId)?.name || '')}` })}
      <div class="stack">
        <div class="card card-pad"><div class="row-between"><div><div class="section-label">Tiến độ toàn khóa</div><div class="small muted">${p.done}/${p.total} học liệu bắt buộc. Học liệu không bắt buộc không tính vào %.</div></div><div style="min-width:260px">${progressBar(p.pct, true)}</div></div>
          ${c.status !== 'active' ? `<div class="alert alert-info" style="margin-top:16px">${icon('info')}<div class="alert-body">${c.status === 'ended' ? 'Lớp đã kết thúc: bạn vẫn xem được học liệu nhưng không tích hoàn thành được nữa.' : 'Lớp chưa bắt đầu.'}</div></div>` : ''}</div>
        <div class="roadmap">${stagesHtml}</div>
      </div>` };
  }
  function pageLesson([classId, lessonId]) {
    const u = me(); const c = cls(classId); const m = c && memberOf(c.id, u.id);
    if (!m || m.status !== 'active') { location.hash = '#/learn'; return null; }
    const v = cv(c.courseVersionId); let found = null;
    v.stages.forEach((id, i) => { const s = sv(id); const l = s.lessons.find((x) => x.id === lessonId); if (l) found = { s, st: stage(s.stageId), l, i }; });
    if (!found) { location.hash = `#/learn/classes/${c.id}`; return null; }
    openLesson(c.id, lessonId); // FR-31: first open recorded, later opens do not overwrite
    const { s, st, l } = found; const pr = progressOf(m.id, l.id); const done = !!pr?.completedAt; const canTick = c.status === 'active';
    const all = v.stages.map(sv).flatMap((x) => x.lessons); const idx = all.findIndex((x) => x.id === l.id); const prev = all[idx - 1]; const next = all[idx + 1];
    const content = l.type === 'video'
      ? `<div class="video-frame" role="img" aria-label="Khung phát video ${esc(l.video)}"><div><div class="play">${icon('play')}</div><strong>${esc(l.title)}</strong><small>${esc(l.video)} · ${esc(l.duration || '')} · phát qua đường dẫn có chữ ký (giả lập trong bản mẫu)</small></div></div>`
      : `<div class="card-body"><div class="md">${renderMarkdown(l.content)}</div></div>`;
    const side = `<aside class="card"><div class="card-head"><h2>${esc(st.name)} <span class="muted small">${versionLabel(s)}</span></h2></div><ul class="side-list">${s.lessons.map((x) => { const d = !!progressOf(m.id, x.id)?.completedAt; return `<li><a href="#/learn/classes/${c.id}/lessons/${x.id}" ${x.id === l.id ? 'aria-current="page"' : ''}>${d ? icon('check-circle', 'done-mark') : '<span class="todo-mark"></span>'}<span>${esc(x.title)}</span></a></li>`; }).join('')}</ul>
      <div class="card-pad"><a class="un-btn un-btn-ghost un-btn-sm" href="#/learn/classes/${c.id}">${icon('layers')} Toàn bộ lộ trình</a></div></aside>`;
    return { title: l.title, html: `${pageHead(l.title, { crumbsHtml: crumbs([['Lớp của tôi', '#/learn'], [c.code, `#/learn/classes/${c.id}`], [st.name, `#/learn/classes/${c.id}`], [l.title]]), badges: l.required ? '' : '<span class="badge badge-optional">Không bắt buộc</span>' })}
      <div class="viewer"><div class="card">${content}
        <div class="complete-bar"><div class="row">${prev ? `<a class="un-btn un-btn-ghost un-btn-sm" href="#/learn/classes/${c.id}/lessons/${prev.id}">Bài trước</a>` : ''}${next ? `<a class="un-btn un-btn-ghost un-btn-sm" href="#/learn/classes/${c.id}/lessons/${next.id}">Bài tiếp ${icon('arrow-right')}</a>` : ''}</div>
          <div class="row">${done ? `<span class="dot dot-ok small">Đã tích ${fmtDateTime(pr.completedAt)}</span>` : ''}<button class="un-btn ${done ? 'un-btn-secondary' : 'un-btn-primary'}" data-action="complete" data-class="${c.id}" data-lesson="${l.id}" data-done="${done ? 0 : 1}" ${canTick ? '' : `aria-disabled="true" title="${c.status === 'ended' ? 'Lớp đã kết thúc' : 'Lớp chưa bắt đầu'}"`}>${done ? 'Bỏ tích' : `${icon('check')} Đã học xong`}</button></div></div></div>${side}</div>` };
  }

  /* -------------------------------------------------------- dialogs */
  function lessonForm(l = {}) {
    return `<div class="stack-sm">
      ${field('Tiêu đề', `<input class="input" name="title" value="${esc(l.title || '')}" required>`, { required: true })}
      <div class="form-grid">${field('Loại', `<select class="input" name="type" data-change="lesson-type">${selectOpts([{ value: 'video', label: 'Video' }, { value: 'markdown', label: 'Markdown' }], l.type || 'video')}</select>`)}
        <label class="check" style="align-self:end"><input type="checkbox" name="required" ${l.required !== false ? 'checked' : ''}> Bắt buộc (tính vào %)</label></div>
      <div data-lesson-video ${(l.type || 'video') === 'video' ? '' : 'hidden'}>${field('File video', `<input class="input" name="video" value="${esc(l.video || '')}" placeholder="vd: db-02-join.mp4">`, { help: 'Bản mẫu chỉ ghi tên file; hệ thống thật upload lên kho lưu trữ và phát qua đường dẫn có chữ ký.' })}${field('Thời lượng', `<input class="input" name="duration" value="${esc(l.duration || '')}" placeholder="mm:ss">`)}</div>
      <div data-lesson-md ${l.type === 'markdown' ? '' : 'hidden'}>${field('Nội dung Markdown', `<textarea class="input" name="content" rows="10">${esc(l.content || '')}</textarea>`, { help: 'Hỗ trợ tiêu đề, danh sách, khối mã, trích dẫn, **đậm** và `mã`. HTML được lọc khi hiển thị.' })}</div>
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
    'publish-sv': (btn) => { const v = sv(btn.dataset.id); confirmAction({ title: `Phát hành ${stage(v.stageId).name} ${versionLabel(v)}?`, text: `Sau khi phát hành, phiên bản này và ${v.lessons.length} học liệu của nó không sửa được nữa. Khóa học đang dùng phiên bản cũ sẽ không tự cập nhật; bạn áp dụng ở bước tiếp theo.`, confirm: 'Phát hành', onConfirm: () => { const r = publishStage(v.id); if (r.ok) { closeModal(); toast(`Đã phát hành ${versionLabel(v)}.`); render(); } return r; } }); },
    'archive-sv': (btn) => { const v = sv(btn.dataset.id); confirmAction({ title: `Lưu trữ ${versionLabel(v)}?`, text: 'Phiên bản lưu trữ không gắn mới vào khóa học được, nhưng các khóa học và lớp đang dùng vẫn hoạt động bình thường.', confirm: 'Lưu trữ', onConfirm: () => handle(archiveStageVersion(v.id), 'Đã lưu trữ.') && ok() }); },
    'delete-sv': (btn) => { const v = sv(btn.dataset.id); const st = stage(v.stageId); const refs = cvsUsingSV(v.id); if (v.status !== 'draft' && refs.length) { toast(`Không xóa được: đang dùng trong ${refs.map((c) => `${course(c.courseId).name} ${versionLabel(c)}`).join(', ')}.`, 'error'); return; }
      confirmAction({ title: `Xóa ${st.name} ${versionLabel(v)}?`, text: v.status === 'draft' ? `Bản nháp và ${v.lessons.length} học liệu trong đó sẽ bị xóa. Không hoàn tác được.` : 'Phiên bản này không được khóa học nào tham chiếu. Xóa sẽ không hoàn tác được.', confirm: 'Xóa phiên bản', danger: true, onConfirm: () => { const r = deleteStageVersion(v.id); if (r.ok) { closeModal(); toast('Đã xóa.'); location.hash = r.stageGone ? '#/admin/stages' : `#/admin/stages/${st.id}`; render(); } return r; } }); },
    'new-lesson': (btn) => openModal({ title: 'Thêm học liệu', confirm: 'Thêm', body: lessonForm(), onSubmit: (d) => handle(saveLesson(btn.dataset.sv, null, { ...d, required: d.required === 'on' }), 'Đã thêm học liệu.') && ok() }),
    'edit-lesson': (btn) => { const l = sv(btn.dataset.sv).lessons.find((x) => x.id === btn.dataset.id); openModal({ title: 'Sửa học liệu', body: lessonForm(l), onSubmit: (d) => handle(saveLesson(btn.dataset.sv, l.id, { ...d, required: d.required === 'on' }), 'Đã lưu học liệu.') && ok() }); },
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
    'complete': (btn) => { if (btn.getAttribute('aria-disabled') === 'true') { toast(btn.title, 'error'); return; } handle(setComplete(btn.dataset.class, btn.dataset.lesson, btn.dataset.done === '1'), btn.dataset.done === '1' ? 'Đã ghi nhận hoàn thành.' : 'Đã bỏ tích.'); }
  };
  const FORMS = {
    login: (form) => { const r = login(form.email.value, form.password.value); if (!r.ok) { formError(form, r.error); return; } location.hash = r.mustChange ? '#/first-login' : homeFor(me()); render(); },
    forgot: (form) => { form.querySelector('[data-form-ok]').hidden = false; form.email.value = ''; },
    'first-login': (form) => { const r = changePassword(form.password.value, form.confirm.value); if (!r.ok) { formError(form, r.error); return; } toast('Đã lưu mật khẩu. Chào mừng bạn vào lớp.'); location.hash = homeFor(me()); render(); },
    'class-settings': (form) => { const d = inputs(form); const c = cls(form.dataset.id); if (!('courseVersionId' in d)) d.courseVersionId = c.courseVersionId; const r = updateClass(c.id, d); if (!r.ok) { formError(form, r.error); return; } toast('Đã lưu cài đặt lớp.'); render(); },
    'report-filter': (form) => { const d = inputs(form); location.hash = form.dataset.base + q({ notlogged: d.notlogged ? 1 : '', inactive: d.inactive, below: d.below, sort: d.sort }).replace('?', form.dataset.base.includes('?') ? '&' : '?'); },
    modal: (form) => { if (!modalHandler) return; const r = modalHandler(inputs(form)); if (r && r.ok === false) modalError(r.error); }
  };
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
  });
  document.addEventListener('submit', (e) => { const form = e.target.closest('form[data-form]'); if (!form) return; e.preventDefault(); const fn = FORMS[form.dataset.form]; if (fn) fn(form); });
  document.addEventListener('change', (e) => {
    const el = e.target.closest('[data-change]'); if (!el) return;
    if (el.dataset.change === 'switch-role') { switchRole(el.value); location.hash = homeFor(me()); render(); }
    if (el.dataset.change === 'lesson-type') { const m = el.closest('.modal-body'); m.querySelector('[data-lesson-video]').hidden = el.value !== 'video'; m.querySelector('[data-lesson-md]').hidden = el.value !== 'markdown'; }
    if (el.dataset.change === 'toggle-complete') { const r = setComplete(el.dataset.class, el.dataset.lesson, el.checked); if (!r.ok) { el.checked = !el.checked; toast(r.error, 'error'); } else render(); }
  });
  $('#modal').addEventListener('close', () => { modalHandler = null; });
  window.addEventListener('hashchange', render);

  load(); render();
})();
