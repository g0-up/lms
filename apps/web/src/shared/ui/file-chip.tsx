import { Download } from "lucide-react";
import { API_BASE } from "@/shared/api/http";
import { fmtSize } from "@/shared/lib/format";

export interface FileChipProps {
  mediaId: string;
  fileName: string;
  sizeBytes?: number;
}

/**
 * Download link for a stored file. The API redirects to a short-lived signed URL that answers with
 * `Content-Disposition: attachment`, so the browser saves the file instead of previewing it.
 */
export function FileChip({ mediaId, fileName, sizeBytes }: FileChipProps) {
  return (
    <a
      href={`${API_BASE}/media/${encodeURIComponent(mediaId)}/content`}
      download={fileName}
      className="inline-flex min-h-11 max-w-full items-center gap-2 border border-line bg-white px-3 text-sm text-ink no-underline hover:border-navy-700 hover:text-navy-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-navy-700"
    >
      <Download className="size-4 shrink-0" aria-hidden="true" />
      <span className="truncate">{fileName}</span>
      {sizeBytes === undefined ? null : <span className="shrink-0 text-xs text-ink-3">{fmtSize(sizeBytes)}</span>}
    </a>
  );
}
