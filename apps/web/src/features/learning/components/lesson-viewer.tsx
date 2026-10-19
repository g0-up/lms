import type { LessonPage } from "../model/schemas";
import { HomeworkView } from "./homework-view";
import { MarkdownLesson } from "./markdown-lesson";
import { QuizView } from "./quiz-view";
import { VideoPlayer } from "./video-player";

/**
 * Picks the renderer for the lesson's content type; a new type adds a component and a branch. A quiz brings its
 * own bottom bar (start, step and result actions), so the page leaves out the completion bar for it.
 */
export function LessonViewer({ classId, page }: { classId: string; page: LessonPage }) {
  const { content } = page;
  switch (content.type) {
    case "video":
      return <VideoPlayer lessonId={page.lesson.id} content={content} />;
    case "markdown":
      return <MarkdownLesson html={content.html} />;
    case "homework":
      return <HomeworkView homework={content.homework} />;
    case "quiz":
      return <QuizView classId={classId} page={page} />;
  }
}
