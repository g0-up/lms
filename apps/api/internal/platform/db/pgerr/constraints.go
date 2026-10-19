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

// Bảng stage_units.
const (
	PkStageUnits                = "pk_stage_units"
	UqStageUnitsIdVersion       = "uq_stage_units_id_version"
	UqStageUnitsVersionKey      = "uq_stage_units_version_key"
	UqStageUnitsVersionPosition = "uq_stage_units_version_position"
	UqStageUnitsVersionTitle    = "uq_stage_units_version_title"
	FkStageUnitsStageVersion    = "fk_stage_units_stage_version"
	CkStageUnitsKey             = "ck_stage_units_key"
	CkStageUnitsPosition        = "ck_stage_units_position"
	CkStageUnitsTitle           = "ck_stage_units_title"
)

// Bảng lessons.
const (
	PkLessons                = "pk_lessons"
	UqLessonsVersionKey      = "uq_lessons_version_key"
	UqLessonsVersionPosition = "uq_lessons_version_position"
	FkLessonsStageVersion    = "fk_lessons_stage_version"
	FkLessonsUnit            = "fk_lessons_unit"
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

// Bảng quiz_settings.
const (
	PkQuizSettings       = "pk_quiz_settings"
	FkQuizSettingsLesson = "fk_quiz_settings_lesson"
	CkQuizSettingsMode   = "ck_quiz_settings_mode"
	CkQuizSettingsRules  = "ck_quiz_settings_rules"
)

// Bảng questions.
const (
	PkQuestions               = "pk_questions"
	UqQuestionsLessonKey      = "uq_questions_lesson_key"
	UqQuestionsLessonPosition = "uq_questions_lesson_position"
	FkQuestionsLesson         = "fk_questions_lesson"
	CkQuestionsKey            = "ck_questions_key"
	CkQuestionsPoints         = "ck_questions_points"
	CkQuestionsType           = "ck_questions_type"
)

// Bảng question_options.
const (
	PkQuestionOptions                 = "pk_question_options"
	UqQuestionOptionsQuestionPosition = "uq_question_options_question_position"
	FkQuestionOptionsQuestion         = "fk_question_options_question"
	CkQuestionOptionsBody             = "ck_question_options_body"
)

// Bảng homework_settings.
const (
	PkHomeworkSettings            = "pk_homework_settings"
	FkHomeworkSettingsLesson      = "fk_homework_settings_lesson"
	CkHomeworkSettingsPassPercent = "ck_homework_settings_pass_percent" //nolint:gosec // tên constraint, không phải thông tin đăng nhập
)

// Bảng rubric_criteria.
const (
	PkRubricCriteria               = "pk_rubric_criteria"
	UqRubricCriteriaLessonPosition = "uq_rubric_criteria_lesson_position"
	FkRubricCriteriaLesson         = "fk_rubric_criteria_lesson"
	CkRubricCriteriaMaxPoints      = "ck_rubric_criteria_max_points"
)

// Bảng homework_attachments.
const (
	PkHomeworkAttachments               = "pk_homework_attachments"
	UqHomeworkAttachmentsLessonPosition = "uq_homework_attachments_lesson_position"
	FkHomeworkAttachmentsLesson         = "fk_homework_attachments_lesson"
	FkHomeworkAttachmentsMedia          = "fk_homework_attachments_media"
)

// Bảng class_lesson_deadlines.
const (
	PkClassLessonDeadlines       = "pk_class_lesson_deadlines"
	FkClassLessonDeadlinesClass  = "fk_class_lesson_deadlines_class"
	FkClassLessonDeadlinesLesson = "fk_class_lesson_deadlines_lesson"
	FkClassLessonDeadlinesSetBy  = "fk_class_lesson_deadlines_set_by"
)

// Bảng submissions.
const (
	PkSubmissions                = "pk_submissions"
	UqSubmissionsMemberLessonNo  = "uq_submissions_member_lesson_no"
	UqSubmissionsOnePending      = "uq_submissions_one_pending"
	FkSubmissionsLesson          = "fk_submissions_lesson"
	FkSubmissionsMember          = "fk_submissions_member"
	CkSubmissionsAttemptNo       = "ck_submissions_attempt_no"
	CkSubmissionsCommitNeedsRepo = "ck_submissions_commit_needs_repo"
	CkSubmissionsRepoUrl         = "ck_submissions_repo_url"
	CkSubmissionsStatus          = "ck_submissions_status"
)

// Bảng submission_files.
const (
	PkSubmissionFiles           = "pk_submission_files"
	UqSubmissionFilesMedia      = "uq_submission_files_media"
	UqSubmissionFilesPosition   = "uq_submission_files_position"
	FkSubmissionFilesMedia      = "fk_submission_files_media"
	FkSubmissionFilesSubmission = "fk_submission_files_submission"
)

// Bảng submission_grades.
const (
	PkSubmissionGrades           = "pk_submission_grades"
	FkSubmissionGradesGradedBy   = "fk_submission_grades_graded_by"
	FkSubmissionGradesSubmission = "fk_submission_grades_submission"
	CkSubmissionGradesComment    = "ck_submission_grades_comment"
	CkSubmissionGradesPoints     = "ck_submission_grades_points"
	CkSubmissionGradesResult     = "ck_submission_grades_result"
)

// Bảng grade_criterion_scores.
const (
	PkGradeCriterionScores          = "pk_grade_criterion_scores"
	FkGradeCriterionScoresCriterion = "fk_grade_criterion_scores_criterion"
	FkGradeCriterionScoresGrade     = "fk_grade_criterion_scores_grade"
	CkGradeCriterionScoresPoints    = "ck_grade_criterion_scores_points"
)

// Bảng quiz_attempts.
const (
	PkQuizAttempts                  = "pk_quiz_attempts"
	UqQuizAttemptsMemberLessonNo    = "uq_quiz_attempts_member_lesson_no"
	UqQuizAttemptsOneOpen           = "uq_quiz_attempts_one_open"
	UqQuizAttemptsOnePending        = "uq_quiz_attempts_one_pending"
	FkQuizAttemptsLesson            = "fk_quiz_attempts_lesson"
	FkQuizAttemptsMember            = "fk_quiz_attempts_member"
	CkQuizAttemptsReviewAfterSubmit = "ck_quiz_attempts_review_after_submit"
	CkQuizAttemptsReviewStatus      = "ck_quiz_attempts_review_status"
	CkQuizAttemptsScored            = "ck_quiz_attempts_scored"
)

// Bảng quiz_attempt_answers.
const (
	PkQuizAttemptAnswers         = "pk_quiz_attempt_answers"
	FkQuizAttemptAnswersAttempt  = "fk_quiz_attempt_answers_attempt"
	FkQuizAttemptAnswersQuestion = "fk_quiz_attempt_answers_question"
)

// Bảng quiz_attempt_reviews.
const (
	PkQuizAttemptReviews         = "pk_quiz_attempt_reviews"
	FkQuizAttemptReviewsAttempt  = "fk_quiz_attempt_reviews_attempt"
	FkQuizAttemptReviewsReviewer = "fk_quiz_attempt_reviews_reviewer"
	CkQuizAttemptReviewsComment  = "ck_quiz_attempt_reviews_comment"
)

// Bảng quiz_review_notes.
const (
	UqQuizReviewNotesReviewQuestion = "uq_quiz_review_notes_review_question"
	FkQuizReviewNotesQuestion       = "fk_quiz_review_notes_question"
	FkQuizReviewNotesReview         = "fk_quiz_review_notes_review"
	CkQuizReviewNotesNote           = "ck_quiz_review_notes_note"
)

// Bảng quiz_attempt_grants.
const (
	PkQuizAttemptGrants          = "pk_quiz_attempt_grants"
	FkQuizAttemptGrantsGrantedBy = "fk_quiz_attempt_grants_granted_by"
	FkQuizAttemptGrantsLesson    = "fk_quiz_attempt_grants_lesson"
	FkQuizAttemptGrantsMember    = "fk_quiz_attempt_grants_member"
	CkQuizAttemptGrantsExtra     = "ck_quiz_attempt_grants_extra"
	CkQuizAttemptGrantsReason    = "ck_quiz_attempt_grants_reason"
)

// Bảng stage_reviews.
const (
	PkStageReviews              = "pk_stage_reviews"
	UqStageReviewsMemberVersion = "uq_stage_reviews_member_version"
	FkStageReviewsAuthor        = "fk_stage_reviews_author"
	FkStageReviewsMember        = "fk_stage_reviews_member"
	FkStageReviewsStageVersion  = "fk_stage_reviews_stage_version"
	CkStageReviewsBody          = "ck_stage_reviews_body"
)

// Bảng scheduled_job_runs.
const (
	PkScheduledJobRuns = "pk_scheduled_job_runs"
)
