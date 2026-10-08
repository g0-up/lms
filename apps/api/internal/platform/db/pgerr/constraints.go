// Package pgerr dịch lỗi Postgres của ứng dụng. File này là sổ tên constraint DUY NHẤT:
// mọi tên PK, UNIQUE (kể cả partial unique index uq_*), CHECK và FK trong migrations/ có đúng một hằng ở đây,
// feature chỉ dùng hằng, không gõ chuỗi. Test tích hợp constraints_test.go so tập hằng với pg_constraint
// và index unique của pg_class theo cả hai chiều. Tên hằng là CamelCase của tên constraint.
//
// Thêm migration có constraint mới: thêm hằng cùng nhóm bảng rồi chạy test tích hợp của gói này.
package pgerr

// Bảng users.
const (
	PkUsers                = "pk_users"
	UqUsersEmailNormalized = "uq_users_email_normalized"
	CkUsersDisabledAt      = "ck_users_disabled_at"
	CkUsersEmailNormalized = "ck_users_email_normalized"
	CkUsersFullName        = "ck_users_full_name"
	CkUsersRole            = "ck_users_role"
	CkUsersStatus          = "ck_users_status"
)

// Bảng sessions.
const (
	PkSessions          = "pk_sessions"
	UqSessionsTokenHash = "uq_sessions_token_hash" //nolint:gosec // tên constraint, không phải thông tin đăng nhập
	FkSessionsUser      = "fk_sessions_user"
)

// Bảng media_files.
const (
	PkMediaFiles           = "pk_media_files"
	UqMediaFilesStorageKey = "uq_media_files_storage_key"
	FkMediaFilesUploadedBy = "fk_media_files_uploaded_by"
	CkMediaFilesKind       = "ck_media_files_kind"
	CkMediaFilesSizeBytes  = "ck_media_files_size_bytes"
	CkMediaFilesStatus     = "ck_media_files_status"
)

// Bảng stages.
const (
	PkStages          = "pk_stages"
	UqStagesCode      = "uq_stages_code"
	FkStagesCreatedBy = "fk_stages_created_by"
	CkStagesCode      = "ck_stages_code"
)

// Bảng stage_versions.
const (
	PkStageVersions               = "pk_stage_versions"
	UqStageVersionsIdStage        = "uq_stage_versions_id_stage"
	UqStageVersionsOneDraft       = "uq_stage_versions_one_draft"
	UqStageVersionsStageVersionNo = "uq_stage_versions_stage_version_no"
	FkStageVersionsClonedFrom     = "fk_stage_versions_cloned_from"
	FkStageVersionsCreatedBy      = "fk_stage_versions_created_by"
	FkStageVersionsStage          = "fk_stage_versions_stage"
	CkStageVersionsArchivedAt     = "ck_stage_versions_archived_at"
	CkStageVersionsPublishedAt    = "ck_stage_versions_published_at"
	CkStageVersionsStatus         = "ck_stage_versions_status"
	CkStageVersionsVersionNo      = "ck_stage_versions_version_no"
)

// Bảng lessons.
const (
	PkLessons                = "pk_lessons"
	UqLessonsVersionKey      = "uq_lessons_version_key"
	UqLessonsVersionPosition = "uq_lessons_version_position"
	FkLessonsStageVersion    = "fk_lessons_stage_version"
	FkLessonsVideoMedia      = "fk_lessons_video_media"
	CkLessonsDuration        = "ck_lessons_duration"
	CkLessonsKey             = "ck_lessons_key"
	CkLessonsPosition        = "ck_lessons_position"
	CkLessonsType            = "ck_lessons_type"
	CkLessonsTypeContent     = "ck_lessons_type_content"
)

// Bảng lesson_media.
const (
	PkLessonMedia       = "pk_lesson_media"
	FkLessonMediaLesson = "fk_lesson_media_lesson"
	FkLessonMediaMedia  = "fk_lesson_media_media"
)

// Bảng courses.
const (
	PkCourses          = "pk_courses"
	UqCoursesCode      = "uq_courses_code"
	FkCoursesCreatedBy = "fk_courses_created_by"
	CkCoursesCode      = "ck_courses_code"
)

// Bảng course_versions.
const (
	PkCourseVersions                = "pk_course_versions"
	UqCourseVersionsCourseVersionNo = "uq_course_versions_course_version_no"
	UqCourseVersionsOneDraft        = "uq_course_versions_one_draft"
	FkCourseVersionsClonedFrom      = "fk_course_versions_cloned_from"
	FkCourseVersionsCourse          = "fk_course_versions_course"
	FkCourseVersionsCreatedBy       = "fk_course_versions_created_by"
	CkCourseVersionsArchivedAt      = "ck_course_versions_archived_at"
	CkCourseVersionsPublishedAt     = "ck_course_versions_published_at"
	CkCourseVersionsStatus          = "ck_course_versions_status"
	CkCourseVersionsVersionNo       = "ck_course_versions_version_no"
)

// Bảng course_version_stages.
const (
	PkCourseVersionStages = "pk_course_version_stages"
	UqCvsVersionPosition  = "uq_cvs_version_position"
	UqCvsVersionStage     = "uq_cvs_version_stage"
	FkCvsCourseVersion    = "fk_cvs_course_version"
	FkCvsStage            = "fk_cvs_stage"
	FkCvsStageVersion     = "fk_cvs_stage_version"
	CkCvsPosition         = "ck_cvs_position"
)

// Bảng classes.
const (
	PkClasses              = "pk_classes"
	UqClassesCode          = "uq_classes_code"
	FkClassesCourseVersion = "fk_classes_course_version"
	FkClassesCreatedBy     = "fk_classes_created_by"
	FkClassesTeacher       = "fk_classes_teacher"
	CkClassesCode          = "ck_classes_code"
	CkClassesDates         = "ck_classes_dates"
	CkClassesStatus        = "ck_classes_status"
)

// Bảng class_members.
const (
	PkClassMembers          = "pk_class_members"
	UqClassMembersClassUser = "uq_class_members_class_user"
	FkClassMembersClass     = "fk_class_members_class"
	FkClassMembersUser      = "fk_class_members_user"
	CkClassMembersDropped   = "ck_class_members_dropped"
	CkClassMembersStatus    = "ck_class_members_status"
)

// Bảng invitations.
const (
	PkInvitations            = "pk_invitations"
	FkInvitationsClass       = "fk_invitations_class"
	FkInvitationsEmailOutbox = "fk_invitations_email_outbox"
	FkInvitationsInvitedBy   = "fk_invitations_invited_by"
	FkInvitationsUser        = "fk_invitations_user"
	CkInvitationsKind        = "ck_invitations_kind"
)

// Bảng lesson_progress.
const (
	PkLessonProgress       = "pk_lesson_progress"
	FkLessonProgressLesson = "fk_lesson_progress_lesson"
	FkLessonProgressMember = "fk_lesson_progress_member"
	CkLessonProgressOrder  = "ck_lesson_progress_order"
)

// Bảng email_outbox.
const (
	PkEmailOutbox         = "pk_email_outbox"
	CkEmailOutboxAttempts = "ck_email_outbox_attempts"
	CkEmailOutboxStatus   = "ck_email_outbox_status"
	CkEmailOutboxTemplate = "ck_email_outbox_template"
)

// Bảng password_reset_tokens.
const (
	PkPasswordResetTokens = "pk_password_reset_tokens"
	UqPrtTokenHash        = "uq_prt_token_hash" //nolint:gosec // tên constraint, không phải thông tin đăng nhập
	FkPrtUser             = "fk_prt_user"
)

// Bảng login_attempts.
const (
	PkLoginAttempts = "pk_login_attempts"
)

// Bảng audit_logs.
const (
	PkAuditLogs      = "pk_audit_logs"
	FkAuditLogsActor = "fk_audit_logs_actor"
)
