import { KpiCard } from "@/shared/ui/kpi-card";
import type { Dashboard } from "../model/schemas";

export const kpiGridClass = "grid grid-cols-4 gap-4 max-[1024px]:grid-cols-2 max-[560px]:grid-cols-1";

export function KpiGrid({ kpis, hints }: Pick<Dashboard, "kpis" | "hints">) {
  return (
    <div className={kpiGridClass}>
      <KpiCard label="Lớp đang chạy" value={kpis.classes} hint={`${String(hints.draftClasses)} lớp nháp`} />
      <KpiCard
        label="Học viên đang học"
        value={kpis.students}
        hint={`${String(hints.notLoggedIn)} chưa đăng nhập lần đầu`}
      />
      <KpiCard label="Lời mời thất bại" value={hints.failedInvites} hint="Cần gửi lại" />
      <KpiCard
        label="Khóa học dùng chặng cũ"
        value={hints.outdatedCourses}
        hint={hints.outdatedCourses > 0 ? "Có thể áp dụng một thao tác" : "Mọi khóa học đã cập nhật"}
      />
    </div>
  );
}
