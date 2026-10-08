import { Mail } from "lucide-react";
import { Link } from "react-router";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import type { OutdatedRow } from "../model/schemas";

export interface OutdatedAlertsProps {
  outdated: readonly OutdatedRow[];
  failedInvites: number;
}

/**
 * Work waiting for the admin: courses on an older stage version, then failed invitations.
 * Renders nothing when there is none. The link opens the stage on its latest published version,
 * where the apply action lives.
 */
export function OutdatedAlerts({ outdated, failedInvites }: OutdatedAlertsProps) {
  if (outdated.length === 0 && failedInvites === 0) return null;
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
    </div>
  );
}
