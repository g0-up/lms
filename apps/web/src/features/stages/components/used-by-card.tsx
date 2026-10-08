import { StatusBadge } from "@/shared/ui/badge";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import type { UsedByRow } from "../model/schemas";
import { ItemBody, ItemMeta, ItemTitleLink, List, ListItem } from "./list";

/** Course versions referencing the stage version being viewed, with their classes. */
export function UsedByCard({ versionNo, usedBy }: { versionNo: number; usedBy: readonly UsedByRow[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Đang được dùng ở</CardTitle>
      </CardHeader>
      {usedBy.length === 0 ? (
        <CardBody className="text-sm text-ink-3">Chưa có khóa học nào dùng v{versionNo}.</CardBody>
      ) : (
        <List>
          {usedBy.map((row) => (
            <ListItem key={row.courseVersionId}>
              <ItemBody>
                <ItemTitleLink to={`/admin/courses/${row.courseId}?v=${row.courseVersionId}`}>
                  {row.courseName} v{row.versionNo}
                </ItemTitleLink>
                <ItemMeta>
                  <StatusBadge status={row.status} />
                  <span>{row.classCodes.length > 0 ? row.classCodes.join(", ") : "chưa có lớp"}</span>
                </ItemMeta>
              </ItemBody>
            </ListItem>
          ))}
        </List>
      )}
    </Card>
  );
}
