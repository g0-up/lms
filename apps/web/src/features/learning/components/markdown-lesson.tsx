import { CardBody } from "@/shared/ui/card";
import { MarkdownContent } from "@/shared/ui/markdown-content";

/** Reading lesson: HTML rendered by the API, sanitized again by `MarkdownContent`. */
export function MarkdownLesson({ html }: { html: string }) {
  return (
    <CardBody>
      <MarkdownContent html={html} />
    </CardBody>
  );
}
