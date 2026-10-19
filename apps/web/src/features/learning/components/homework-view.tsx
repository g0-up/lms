import { passPoints } from "@/shared/domain";
import { CardBody } from "@/shared/ui/card";
import { FileChip } from "@/shared/ui/file-chip";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import type { HomeworkContent } from "../model/schemas";

function SectionLabel({ children }: { children: string }) {
  return <h2 className="mt-6 mb-2 text-xs font-semibold tracking-[.5px] text-ink-3 uppercase">{children}</h2>;
}

/** Homework brief: the assignment, its attached files (download only) and the grading rubric. */
export function HomeworkView({ homework }: { homework: HomeworkContent["homework"] }) {
  const { briefHtml, attachments, criteria, maxPoints, passPercent } = homework;
  return (
    <CardBody>
      <MarkdownContent html={briefHtml} />
      {attachments.length > 0 ? (
        <>
          <SectionLabel>Tài liệu kèm theo</SectionLabel>
          <ul className="flex flex-wrap gap-2">
            {attachments.map((file) => (
              <li key={file.mediaId} className="max-w-full">
                <FileChip mediaId={file.mediaId} fileName={file.fileName} sizeBytes={file.sizeBytes} />
              </li>
            ))}
          </ul>
        </>
      ) : null}
      <SectionLabel>
        {`Tiêu chí chấm · đạt từ ${String(passPercent)}% (${String(passPoints(maxPoints, passPercent))}/${String(maxPoints)} điểm)`}
      </SectionLabel>
      <ol className="border border-line">
        {criteria.map((criterion, i) => (
          <li key={criterion.id} className="flex items-center gap-3 border-b border-line px-4 py-3 last:border-b-0">
            <span aria-hidden="true" className="w-5 shrink-0 text-sm font-semibold text-ink-3 tabular-nums">
              {i + 1}
            </span>
            <span className="min-w-0 flex-1">{criterion.description}</span>
            <span className="shrink-0 text-sm text-ink-3 tabular-nums">{criterion.maxPoints} điểm</span>
          </li>
        ))}
      </ol>
    </CardBody>
  );
}
