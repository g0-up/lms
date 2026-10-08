import type { LessonPage } from "../model/schemas";
import { MarkdownLesson } from "./markdown-lesson";
import { VideoPlayer } from "./video-player";

/** Picks the renderer for the lesson's content type; a new type adds a component and a branch. */
export function LessonViewer({ page }: { page: LessonPage }) {
  const { content } = page;
  switch (content.type) {
    case "video":
      return <VideoPlayer lessonId={page.lesson.id} content={content} />;
    case "markdown":
      return <MarkdownLesson html={content.html} />;
  }
}
