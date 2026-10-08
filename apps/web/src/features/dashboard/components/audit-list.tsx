import { ItemBody, ItemMeta, ItemTitle, List, ListItem } from "@/features/stages";
import { fmtDateTime, rel } from "@/shared/lib/format";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { RECENT_ACTIVITY_LIMIT, type Activity } from "../model/schemas";

export function AuditList({ activity }: { activity: readonly Activity[] }) {
  const recent = activity.slice(0, RECENT_ACTIVITY_LIMIT);
  return (
    <Card>
      <CardHeader>
        <CardTitle>Nhật ký thao tác</CardTitle>
      </CardHeader>
      {recent.length === 0 ? (
        <EmptyState title="Chưa có thao tác" />
      ) : (
        <List>
          {recent.map((a) => (
            <ListItem key={a.id}>
              <ItemBody>
                <ItemTitle>{a.actionLabel}</ItemTitle>
                <ItemMeta>
                  {a.target.label} · {a.actorName}
                </ItemMeta>
              </ItemBody>
              <time dateTime={a.at} title={fmtDateTime(a.at)} className="shrink-0 text-xs whitespace-nowrap text-ink-3">
                {rel(a.at)}
              </time>
            </ListItem>
          ))}
        </List>
      )}
    </Card>
  );
}
