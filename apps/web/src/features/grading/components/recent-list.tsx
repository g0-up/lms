import { Link } from "react-router";
import { shortTitle } from "@/shared/domain";
import { rel } from "@/shared/lib/format";
import { Badge } from "@/shared/ui/badge";
import { EmptyState } from "@/shared/ui/empty-state";
import { itemHref, type GradingBase } from "../model/links";
import type { Recent } from "../model/schemas";

function Verdict({ item }: { item: Recent }) {
  if (item.passed) return <Badge variant="ok">Đạt</Badge>;
  return <Badge variant="warn">{item.kind === "quiz" ? "Chưa đạt" : "Cần làm lại"}</Badge>;
}

/** The grader's latest gradings and quiz confirmations, newest first. */
export function RecentList({ items, base }: { items: Recent[]; base: GradingBase }) {
  if (items.length === 0) {
    return <EmptyState title="Chưa xử lý bài nào" text="Bài đã chấm và lượt quiz đã xác nhận sẽ hiện ở đây." />;
  }
  return (
    <ul>
      {items.map((item) => (
        <li
          key={`${item.kind}-${item.itemId}-${item.at}`}
          className="flex items-center gap-3 border-b border-line px-5 py-3 last:border-b-0 max-[720px]:px-4"
        >
          <div className="min-w-0 flex-1">
            <Link
              to={itemHref(base, item.kind, item.itemId)}
              className="-my-2.5 inline-block py-2.5 font-medium text-ink no-underline hover:text-navy-700 hover:underline"
            >
              {item.student.name} · {shortTitle(item.lesson.title)}
            </Link>
            <div className="flex flex-wrap items-center gap-x-2 text-xs text-ink-3">
              <span>{item.class.code}</span>
              <span>{item.kind === "quiz" ? `Quiz · lượt #${String(item.no)}` : `Lần nộp #${String(item.no)}`}</span>
              <span>
                {item.points}/{item.maxPoints} điểm
              </span>
              <span>
                {item.by.name} · {rel(item.at)}
              </span>
            </div>
          </div>
          <Verdict item={item} />
        </li>
      ))}
    </ul>
  );
}
