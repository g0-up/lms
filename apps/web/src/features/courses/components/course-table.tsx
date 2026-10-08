import { TriangleAlert } from "lucide-react";
import { Link, useNavigate } from "react-router";
import { latestPublished, navigateFromRow, sortVersions, toVersionRef } from "@/features/stages";
import { Badge } from "@/shared/ui/badge";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import { Lineage } from "@/shared/ui/version-pill";
import type { CourseListItem } from "../model/schemas";
import { coursePath } from "../hooks/use-course-mutations";

function StageCount({ course }: { course: CourseListItem }) {
  const latest = latestPublished(course.versions);
  if (!latest) return <span className="text-ink-3">—</span>;
  return (
    <span className="inline-flex flex-wrap items-center justify-end gap-2">
      {latest.outdatedStageCount > 0 ? (
        <Badge variant="warn">
          <TriangleAlert aria-hidden="true" className="size-3" /> {latest.outdatedStageCount} chặng cũ
        </Badge>
      ) : null}
      <span className="tabular-nums">{latest.stageCount}</span>
    </span>
  );
}

export function CourseTable({ items }: { items: readonly CourseListItem[] }) {
  const navigate = useNavigate();
  return (
    <TableWrap>
      <Table>
        <TableCaption>Danh sách khóa học</TableCaption>
        <TableHeader>
          <TableRow>
            <TableHead>Khóa học</TableHead>
            <TableHead>Phiên bản</TableHead>
            <TableHead className="text-right">Số chặng</TableHead>
            <TableHead>Lớp đang dùng</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((course) => {
            const href = coursePath(course.id);
            return (
              <TableRow
                key={course.id}
                className="cursor-pointer"
                onClick={(event) => {
                  navigateFromRow(event, () => void navigate(href));
                }}
              >
                <TableCell>
                  <div className="flex flex-col gap-0.5">
                    <Link to={href} className="font-medium text-ink no-underline hover:text-navy-700 hover:underline">
                      {course.name}
                    </Link>
                    <span className="text-xs text-ink-3">{course.code}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <Lineage versions={sortVersions(course.versions).map(toVersionRef)} hrefFor={(v) => `${href}?v=${v.id}`} />
                </TableCell>
                <TableCell className="text-right">
                  <StageCount course={course} />
                </TableCell>
                <TableCell>
                  {course.classesUsing.length > 0 ? (
                    course.classesUsing.map((c) => `${c.code} (v${String(c.versionNo)})`).join(", ")
                  ) : (
                    <span className="text-ink-3">—</span>
                  )}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </TableWrap>
  );
}
