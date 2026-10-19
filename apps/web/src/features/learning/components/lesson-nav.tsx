import { ArrowRight } from "lucide-react";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import type { LessonLink } from "../model/schemas";

function NavButton({
  to,
  edgeTitle,
  title,
  children,
}: {
  to: string | undefined;
  edgeTitle: string;
  title?: string;
  children: ReactNode;
}) {
  if (!to) {
    // aria-disabled keeps the button focusable so its title can still be read.
    return (
      <Button variant="ghost" size="sm" aria-disabled="true" title={edgeTitle}>
        {children}
      </Button>
    );
  }
  return (
    <Button asChild variant="ghost" size="sm">
      <Link to={to} title={title}>
        {children}
      </Link>
    </Button>
  );
}

export interface LessonNavProps {
  classId: string;
  prev?: LessonLink;
  next?: LessonLink;
  /** `next` starts another bài học: the button says so and its title names that bài học. */
  crossesUnit?: boolean;
  nextUnitTitle?: string;
}

/** Previous / next lesson across the whole course version, as ordered by the API. */
export function LessonNav({ classId, prev, next, crossesUnit = false, nextUnitTitle }: LessonNavProps) {
  const href = (link: LessonLink | undefined) => link && `/learn/classes/${classId}/lessons/${link.lessonId}`;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <NavButton to={href(prev)} edgeTitle="Đây là học liệu đầu tiên">
        Học liệu trước
      </NavButton>
      <NavButton to={href(next)} edgeTitle="Đây là học liệu cuối cùng" title={crossesUnit ? nextUnitTitle : undefined}>
        {crossesUnit ? "Sang bài học tiếp" : "Học liệu tiếp"} <ArrowRight aria-hidden="true" />
      </NavButton>
    </div>
  );
}
