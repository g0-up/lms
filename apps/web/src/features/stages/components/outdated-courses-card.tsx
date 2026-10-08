import { Badge } from "@/shared/ui/badge";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import type { OutdatedCourse } from "../model/schemas";
import { GuardedButton } from "./guarded-button";
import { ItemBody, ItemMeta, ItemTitleLink, List, ListItem } from "./list";

export interface OutdatedCoursesCardProps {
  stageName: string;
  courses: readonly OutdatedCourse[];
  onApply: (course: OutdatedCourse) => void;
}

/** Published courses still on an older version of the stage, each with its one-step apply. */
export function OutdatedCoursesCard({ stageName, courses, onApply }: OutdatedCoursesCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Khóa học đang dùng phiên bản cũ của chặng này</CardTitle>
      </CardHeader>
      <List>
        {courses.map((course) => (
          <ListItem key={course.courseId} className="flex-wrap">
            <ItemBody>
              <ItemTitleLink to={`/admin/courses/${course.courseId}?v=${course.courseVersionId}`}>
                {course.courseName} v{course.courseVersionNo}
              </ItemTitleLink>
              <ItemMeta>
                <span>
                  Đang dùng {stageName} v{course.usingVersionNo}
                </span>
                {course.blockedReason ? <Badge variant="warn" className="whitespace-normal">{course.blockedReason}</Badge> : null}
              </ItemMeta>
            </ItemBody>
            {/* Not gradient: the page's primary action is already the publish button. */}
            <GuardedButton
              size="sm"
              blocker={course.canApply ? null : (course.blockedReason ?? "Khóa học này chưa áp dụng được.")}
              onClick={() => {
                onApply(course);
              }}
            >
              Áp dụng v{course.latestVersionNo}
            </GuardedButton>
          </ListItem>
        ))}
      </List>
    </Card>
  );
}
