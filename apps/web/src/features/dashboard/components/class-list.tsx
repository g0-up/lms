import { Link } from "react-router";
import { ItemBody, ItemMeta, ItemTitleLink, List, ListItem } from "@/features/stages";
import { StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { ProgressBar } from "@/shared/ui/progress-bar";
import type { DashboardClass } from "../model/schemas";

export function ClassList({ classes }: { classes: readonly DashboardClass[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Lớp học</CardTitle>
        <Button asChild variant="ghost" size="sm">
          <Link to="/admin/classes">Tất cả lớp</Link>
        </Button>
      </CardHeader>
      {classes.length === 0 ? (
        <EmptyState title="Chưa có lớp" />
      ) : (
        <List>
          {classes.map((c) => (
            <ListItem key={c.id}>
              <ItemBody>
                <ItemTitleLink to={`/admin/classes/${c.id}`}>
                  {c.code} · {c.name}
                </ItemTitleLink>
                <ItemMeta>
                  <span>
                    {c.courseName} v{c.courseVersionNo}
                  </span>
                  <span>{c.memberCount} học viên</span>
                </ItemMeta>
              </ItemBody>
              <StatusBadge status={c.status} />
              {c.status === "draft" ? null : <ProgressBar value={c.avgPercent} className="w-[120px] shrink-0" />}
            </ListItem>
          ))}
        </List>
      )}
    </Card>
  );
}
