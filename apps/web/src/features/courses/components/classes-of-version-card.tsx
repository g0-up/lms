import { ItemBody, ItemMeta, ItemTitleLink, List, ListItem } from "@/features/stages";
import { StatusBadge } from "@/shared/ui/badge";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import type { VersionClass } from "../model/schemas";

/** Classes attached to the course version being viewed. */
export function ClassesOfVersionCard({ versionNo, classes }: { versionNo: number; classes: readonly VersionClass[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Lớp gắn với v{versionNo}</CardTitle>
      </CardHeader>
      {classes.length === 0 ? (
        <CardBody className="text-sm text-ink-3">Chưa có lớp nào gắn với v{versionNo}.</CardBody>
      ) : (
        <List>
          {classes.map((c) => (
            <ListItem key={c.classId}>
              <ItemBody>
                <ItemTitleLink to={`/admin/classes/${c.classId}`}>
                  {c.code} · {c.name}
                </ItemTitleLink>
                <ItemMeta>
                  <span>{c.memberCount} học viên</span>
                </ItemMeta>
              </ItemBody>
              <StatusBadge status={c.status} />
            </ListItem>
          ))}
        </List>
      )}
    </Card>
  );
}
