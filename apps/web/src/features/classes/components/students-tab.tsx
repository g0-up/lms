import { Mail } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { DEFAULT_FILTER, useReport } from "@/features/reports";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { EmptyState } from "@/shared/ui/empty-state";
import { Skeleton } from "@/shared/ui/skeleton";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import { useMembers } from "../hooks/use-members";
import { useRemoveMember, useResend } from "../hooks/use-member-mutations";
import { useSetUserEnabled } from "../hooks/use-user-mutations";
import { ENDED_CLASS } from "../model/invite-form";
import { sortMembers } from "../model/member-view";
import type { ClassDetail, Member } from "../model/schemas";
import { InviteDialog } from "./invite-dialog";
import { MemberRow, type MemberAction } from "./member-row";

/** Every member, those who left included, for the progress column. */
const ALL_MEMBERS = { ...DEFAULT_FILTER, includeDropped: true };

type Confirming = { action: Exclude<MemberAction, "enable">; member: Member } | null;

function confirmCopy({ action, member }: NonNullable<Confirming>) {
  switch (action) {
    case "resend":
      return {
        title: `Gửi lại lời mời cho ${member.fullName}?`,
        text: "Mật khẩu tạm cũ mất hiệu lực ngay; mật khẩu tạm mới có hiệu lực 72 giờ kể từ bây giờ.",
        confirm: "Gửi lại",
        danger: false,
      };
    case "remove":
      return {
        title: `Gỡ ${member.fullName} khỏi lớp?`,
        text: "Thành viên chuyển sang trạng thái đã rời lớp; dữ liệu tiến độ được giữ nguyên.",
        confirm: "Gỡ khỏi lớp",
        danger: true,
      };
    case "disable":
      return {
        title: `Vô hiệu hóa tài khoản ${member.email}?`,
        text: "Tài khoản không đăng nhập được, phiên hiện tại bị hủy. Tiến độ học giữ nguyên.",
        confirm: "Vô hiệu hóa",
        danger: true,
      };
  }
}

/** Students tab: members with their account and invitation state, invite and per-member actions. */
export function StudentsTab({ cls }: { cls: ClassDetail }) {
  const members = useMembers(cls.id);
  const report = useReport(cls.id, ALL_MEMBERS);
  const resend = useResend(cls.id);
  const remove = useRemoveMember(cls.id);
  const setEnabled = useSetUserEnabled(cls.id);
  const [inviting, setInviting] = useState(false);
  const [confirming, setConfirming] = useState<Confirming>(null);
  const [closing, setClosing] = useState<Confirming>(null);
  const ended = cls.status === "ended";

  const percents = useMemo(() => new Map(report.data?.rows.map((r) => [r.memberId, r.percent])), [report.data]);
  const rows = useMemo(() => (members.data ? sortMembers(members.data) : []), [members.data]);
  const working = resend.isPending || remove.isPending || setEnabled.isPending;

  const openInvite = () => {
    if (ended) toast.error(ENDED_CLASS);
    else setInviting(true);
  };

  const onAction = (action: MemberAction, member: Member) => {
    if (action === "enable") setEnabled.mutate({ member, enabled: true });
    else setConfirming({ action, member });
  };

  const close = () => {
    // Keep the copy while the dialog animates out.
    setClosing(confirming);
    setConfirming(null);
  };

  const onConfirm = () => {
    if (!confirming) return;
    const { action, member } = confirming;
    const settled = { onSettled: close };
    if (action === "resend") resend.mutate(member, settled);
    else if (action === "remove") remove.mutate(member, settled);
    else setEnabled.mutate({ member, enabled: false }, settled);
  };

  const inviteButton = (
    <Button
      size="sm"
      onClick={openInvite}
      aria-disabled={ended || undefined}
      title={ended ? "Không mời được vào lớp đã kết thúc" : undefined}
    >
      <Mail aria-hidden="true" />
      Mời học viên
    </Button>
  );

  let body;
  if (members.isPending) {
    body = (
      <div className="grid gap-3 p-6" aria-busy="true">
        <span className="sr-only">Đang tải danh sách học viên</span>
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </div>
    );
  } else if (members.error) {
    body = (
      <div className="p-6">
        <Alert variant="danger" role="alert">
          Không tải được danh sách học viên.
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void members.refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </div>
    );
  } else {
    body = (
      <TableWrap>
        <Table>
          <TableCaption>Học viên lớp {cls.code}</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Học viên</TableHead>
              <TableHead>Tài khoản</TableHead>
              <TableHead>Lời mời</TableHead>
              <TableHead>Tiến độ</TableHead>
              <TableHead className="text-right">Thao tác</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5}>
                  <EmptyState
                    title="Chưa có học viên"
                    text="Mời học viên bằng email; họ sẽ nhận mật khẩu tạm có hiệu lực 72 giờ."
                    action={
                      ended ? undefined : (
                        <Button variant="outline" size="sm" onClick={openInvite}>
                          <Mail aria-hidden="true" />
                          Mời học viên
                        </Button>
                      )
                    }
                  />
                </TableCell>
              </TableRow>
            ) : (
              rows.map((member) => (
                <MemberRow
                  key={member.id}
                  member={member}
                  percent={percents.get(member.id)}
                  busy={working}
                  onAction={onAction}
                />
              ))
            )}
          </TableBody>
        </Table>
      </TableWrap>
    );
  }

  const shown = confirming ?? closing;
  const copy = shown ? confirmCopy(shown) : null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>Học viên trong lớp</CardTitle>
        {inviteButton}
      </CardHeader>
      {body}
      <InviteDialog cls={cls} open={inviting} onOpenChange={setInviting} />
      {copy ? (
        <ConfirmDialog
          open={confirming !== null}
          onOpenChange={(open) => { if (!open) close(); }}
          title={copy.title}
          text={copy.text}
          confirm={copy.confirm}
          danger={copy.danger}
          loading={working}
          onConfirm={onConfirm}
        />
      ) : null}
    </Card>
  );
}
