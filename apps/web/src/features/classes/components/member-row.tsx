import { Mail } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { Button } from "@/shared/ui/button";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { StatusDot, type StatusDotVariant } from "@/shared/ui/status-dot";
import { TableCell, TableRow } from "@/shared/ui/table";
import { ACCOUNT_LABEL, accountState, inviteCell, type AccountState } from "../model/member-view";
import type { Member } from "../model/schemas";

const ACCOUNT_DOT: Record<AccountState, StatusDotVariant> = {
  disabled: "disabled",
  "invited-expired": "invited",
  invited: "invited",
  active: "active",
};

export type MemberAction = "resend" | "disable" | "enable" | "remove";

export interface MemberRowProps {
  member: Member;
  /** Whole-course percent from the class report; undefined while it loads. */
  percent: number | undefined;
  /** Disables the row's buttons while one of its actions runs. */
  busy: boolean;
  onAction: (action: MemberAction, member: Member) => void;
}

function InviteCell({ member }: { member: Member }) {
  const cell = inviteCell(member);
  if (!cell.dot) return <span className="text-ink-3">—</span>;
  return (
    <div className="flex flex-col gap-0.5">
      <StatusDot variant={cell.dot} label={cell.label} />
      {cell.secondary ? (
        <span className="text-xs text-ink-3" title={cell.secondaryTitle ?? undefined}>
          {cell.secondary}
        </span>
      ) : null}
    </div>
  );
}

/** One member of the students tab: account, invitation, progress and what can be done. */
export function MemberRow({ member, percent, busy, onAction }: MemberRowProps) {
  const dropped = member.memberStatus === "dropped";
  const state = accountState(member);
  const disabled = member.accountStatus === "disabled";

  return (
    // Tinted rather than faded: lowering opacity takes the grey secondary text below 4.5:1.
    <TableRow className={cn(dropped && "bg-surface-100")}>
      <TableCell>
        <div className="flex flex-col gap-0.5">
          <span className="font-medium text-ink">{member.fullName}</span>
          <span className="text-xs text-ink-3">{member.email}</span>
        </div>
      </TableCell>
      <TableCell>
        <StatusDot variant={ACCOUNT_DOT[state]} label={ACCOUNT_LABEL[state]} />
      </TableCell>
      <TableCell>
        <InviteCell member={member} />
      </TableCell>
      <TableCell>
        {dropped || percent === undefined ? <span className="text-ink-3">—</span> : <ProgressBar value={percent} />}
      </TableCell>
      <TableCell className="text-right">
        {dropped ? (
          <span className="text-sm text-ink-3">Đã rời lớp</span>
        ) : (
          <div className="flex flex-wrap justify-end gap-1">
            {member.accountStatus === "invited" ? (
              <Button variant="ghost" size="sm" disabled={busy} onClick={() => { onAction("resend", member); }}>
                <Mail aria-hidden="true" />
                Gửi lại
              </Button>
            ) : null}
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => { onAction(disabled ? "enable" : "disable", member); }}
            >
              {disabled ? "Kích hoạt lại" : "Vô hiệu hóa"}
            </Button>
            <Button variant="ghost" size="sm" disabled={busy} onClick={() => { onAction("remove", member); }}>
              Gỡ khỏi lớp
            </Button>
          </div>
        )}
      </TableCell>
    </TableRow>
  );
}
