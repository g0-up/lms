import { rel } from "@/shared/lib/format";
import type { AttemptGrant } from "../model/quiz-schemas";

/** Extra attempts granted on one quiz, newest first; nothing at all before the first grant. */
export function GrantHistory({ grants }: { grants: readonly AttemptGrant[] }) {
  if (grants.length === 0) return null;
  return (
    <div className="border-t border-line px-6 py-4 max-[720px]:px-4">
      <h3 className="text-xs font-medium tracking-[1px] text-ink-3 uppercase">Lịch sử cấp thêm lượt</h3>
      <ul className="mt-2">
        {grants.map((g) => (
          <li key={g.id} className="flex items-start gap-3 border-b border-line py-2 last:border-b-0">
            <div className="grid min-w-0 flex-1 gap-0.5">
              <span className="font-medium text-ink">
                +{g.extraAttempts} lượt cho {g.studentName}
              </span>
              <span className="flex flex-wrap gap-x-2 text-xs text-ink-3">
                <span className="break-words">{g.reason}</span>
                <span>{g.grantedByName}</span>
              </span>
            </div>
            <span className="text-xs whitespace-nowrap text-ink-3 tabular-nums">{rel(g.grantedAt)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
