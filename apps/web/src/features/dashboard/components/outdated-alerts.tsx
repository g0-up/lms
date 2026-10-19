import { Clock, Mail } from "lucide-react";
import { Link } from "react-router";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import type { OutdatedRow } from "../model/schemas";

export interface OutdatedAlertsProps {
  outdated: readonly OutdatedRow[];
  failedInvites: number;
  /** Queue items waiting past the grading deadline, across every class. */
  overdueCount: number;
}

/**
 * Work waiting for the admin: courses on an older stage version, failed invitations, then grading past three days.
 * Renders nothing when there is none. The outdated link opens the stage on its latest published version, where the
 * apply action lives; the grading link opens the queue filtered to overdue items.
 */
export function OutdatedAlerts({ outdated, failedInvites, overdueCount }: OutdatedAlertsProps) {
  if (outdated.length === 0 && failedInvites === 0 && overdueCount === 0) return null;
  return (
    <div className="grid gap-3">
      {outdated.map((row) => (
        <Alert key={`${row.courseId}:${row.stageId}`} variant="warn">
          <strong className="font-semibold">
            {row.courseName} v{row.courseVersionNo}
          </strong>{" "}
          vẫn dùng {row.stageName} v{row.currentVersionNo} trong khi v{row.latestPublishedNo} đã phát hành.
          <AlertActions>
            <Button asChild variant="outline" size="sm">
              <Link
                to={`/admin/stages/${encodeURIComponent(row.stageId)}?v=${encodeURIComponent(row.latestVersionId)}`}
              >
                Xem và áp dụng
              </Link>
            </Button>
          </AlertActions>
        </Alert>
      ))}
      {failedInvites > 0 ? (
        <Alert variant="danger" icon={<Mail aria-hidden="true" />}>
          <strong className="font-semibold">{failedInvites} lời mời gửi thất bại.</strong> Kiểm tra email học viên rồi gửi
          lại trong trang lớp.
        </Alert>
      ) : null}
      {overdueCount > 0 ? (
        <Alert variant="danger" icon={<Clock aria-hidden="true" />}>
          <strong className="font-semibold">{overdueCount} bài chờ chấm hoặc xác nhận quá 3 ngày.</strong> Học viên đang
          chờ phản hồi để làm tiếp.
          <AlertActions>
            <Button asChild variant="outline" size="sm">
              <Link to="/admin/grading?overdue=1">Xem bài quá hạn chấm</Link>
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
    </div>
  );
}
