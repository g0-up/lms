import { TriangleAlert } from "lucide-react";
import { Link, useNavigate } from "react-router";
import { Badge } from "@/shared/ui/badge";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import { Lineage } from "@/shared/ui/version-pill";
import type { StageListItem } from "../model/schemas";
import { sortVersions, toVersionRef } from "../model/select-version";
import { navigateFromRow } from "./row-link";

export function StageTable({ items }: { items: readonly StageListItem[] }) {
  const navigate = useNavigate();
  return (
    <TableWrap>
      <Table>
        <TableCaption>Danh sách chặng</TableCaption>
        <TableHeader>
          <TableRow>
            <TableHead>Chặng</TableHead>
            <TableHead>Phiên bản</TableHead>
            <TableHead className="text-right">Khóa học dùng</TableHead>
            <TableHead>Cảnh báo</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((stage) => {
            const href = `/admin/stages/${stage.id}`;
            return (
              <TableRow
                key={stage.id}
                className="cursor-pointer"
                onClick={(event) => {
                  navigateFromRow(event, () => void navigate(href));
                }}
              >
                <TableCell>
                  <div className="flex flex-col gap-0.5">
                    <Link to={href} className="font-medium text-ink no-underline hover:text-navy-700 hover:underline">
                      {stage.name}
                    </Link>
                    <span className="text-xs text-ink-3">{stage.code}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <Lineage versions={sortVersions(stage.versions).map(toVersionRef)} hrefFor={(v) => `${href}?v=${v.id}`} />
                </TableCell>
                <TableCell className="text-right tabular-nums">{stage.usedByCourseCount}</TableCell>
                <TableCell>
                  {stage.outdatedCourseCount > 0 ? (
                    <Badge variant="warn">
                      <TriangleAlert aria-hidden="true" className="size-3" /> {stage.outdatedCourseCount} khóa học dùng bản cũ
                    </Badge>
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
