import { ArrowRight } from "lucide-react";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import type { LessonLink } from "../model/schemas";

function NavButton({ to, edgeTitle, children }: { to: string | undefined; edgeTitle: string; children: ReactNode }) {
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
      <Link to={to}>{children}</Link>
    </Button>
  );
}

export interface LessonNavProps {
  classId: string;
  prev?: LessonLink;
  next?: LessonLink;
}

/** Previous / next lesson across the whole course version, as ordered by the API. */
export function LessonNav({ classId, prev, next }: LessonNavProps) {
  const href = (link: LessonLink | undefined) => link && `/learn/classes/${classId}/lessons/${link.lessonId}`;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <NavButton to={href(prev)} edgeTitle="Đây là bài đầu tiên">
        Bài trước
      </NavButton>
      <NavButton to={href(next)} edgeTitle="Đây là bài cuối cùng">
        Bài tiếp <ArrowRight aria-hidden="true" />
      </NavButton>
    </div>
  );
}
