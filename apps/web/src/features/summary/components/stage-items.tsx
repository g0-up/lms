import { Link } from "react-router";
import { Badge, ToneBadge } from "@/shared/ui/badge";
import { TypeIcon } from "@/shared/ui/type-icon";
import { itemDetail } from "../model/item-detail";
import type { SummaryItem, SummaryUnit } from "../model/schemas";

function ItemRow({ item, to }: { item: SummaryItem; to: string | null }) {
  return (
    <li className="flex items-center gap-3 border-b border-line px-6 py-3 last:border-b-0 max-[720px]:px-4">
      <TypeIcon type={item.type} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2 font-medium text-ink">
          {to ? (
            <Link to={to} className="text-ink">
              {item.title}
            </Link>
          ) : (
            item.title
          )}
          {item.required ? null : <Badge variant="subtle">Không bắt buộc</Badge>}
        </div>
        <div className="mt-0.5 text-xs text-ink-3">{itemDetail(item)}</div>
      </div>
      <ToneBadge tone={item.status.tone} className="shrink-0">
        {item.status.label}
      </ToneBadge>
    </li>
  );
}

export interface StageItemsProps {
  units: SummaryUnit[];
  /** Link of a lesson for the student; leave out where lessons cannot be opened (staff, draft class). */
  lessonLink?: (lessonId: string) => string;
}

/** A stage's lessons grouped by bài học: type, title, a detail line by type and the status badge. */
export function StageItems({ units, lessonLink }: StageItemsProps) {
  return (
    <>
      {units.map((unit) => (
        <section key={unit.id}>
          <h3 className="border-b border-line bg-surface-50 px-6 py-2 text-xs font-semibold tracking-[.02em] text-ink-3 max-[720px]:px-4">
            {unit.title}
          </h3>
          <ul>
            {unit.items.map((item) => (
              <ItemRow key={item.lessonId} item={item} to={lessonLink ? lessonLink(item.lessonId) : null} />
            ))}
          </ul>
        </section>
      ))}
    </>
  );
}
