export type VersionStatus = "draft" | "published" | "archived";

export type ClassStatus = "draft" | "active" | "ended";

const VERSION_NEXT: Record<VersionStatus, VersionStatus | null> = {
  draft: "published",
  published: "archived",
  archived: null,
};

const CLASS_NEXT: Record<ClassStatus, ClassStatus | null> = {
  draft: "active",
  active: "ended",
  ended: null,
};

/** Versions only move forward: draft → published → archived. */
export function canTransition(from: VersionStatus, to: VersionStatus): boolean {
  return VERSION_NEXT[from] === to;
}

/** Classes only move forward: draft → active → ended. */
export function canTransitionClass(from: ClassStatus, to: ClassStatus): boolean {
  return CLASS_NEXT[from] === to;
}

export const STATUS_VI = {
  draft: "Nháp",
  published: "Đã phát hành",
  archived: "Lưu trữ",
  active: "Đang chạy",
  ended: "Đã kết thúc",
  queued: "Đang chờ gửi",
  sent: "Đã gửi",
  failed: "Gửi thất bại",
  invited: "Chưa đăng nhập",
  disabled: "Vô hiệu hóa",
  dropped: "Đã rời lớp",
} as const;

export type StatusKey = keyof typeof STATUS_VI;
