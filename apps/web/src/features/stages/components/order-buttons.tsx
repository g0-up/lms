import { ChevronDown, ChevronUp } from "lucide-react";
import { IconButton } from "@/shared/ui/icon-button";

interface OrderButtonsBase {
  onMove: (delta: -1 | 1) => void;
  disabled?: boolean;
  /** Accessible names when the row needs its own, e.g. "Chuyển lên: {title}". */
  labels?: { up: string; down: string };
}

/**
 * Where the row sits: its place in one flat list, or the ends given directly for a row that can
 * move past its own list's edge (a lesson goes on into the next bài học).
 */
export type OrderButtonsProps = OrderButtonsBase &
  ({ index: number; count: number } | { first: boolean; last: boolean });

const DEFAULT_LABELS = { up: "Chuyển lên", down: "Chuyển xuống" };

/** Up/down pair ("Chuyển lên"/"Chuyển xuống" by default), disabled at either end of the list. */
export function OrderButtons({ onMove, disabled = false, labels = DEFAULT_LABELS, ...place }: OrderButtonsProps) {
  const first = "first" in place ? place.first : place.index === 0;
  const last = "last" in place ? place.last : place.index === place.count - 1;
  return (
    <>
      <IconButton
        aria-label={labels.up}
        title={first ? "Đã ở đầu danh sách" : undefined}
        disabled={disabled || first}
        onClick={() => {
          onMove(-1);
        }}
      >
        <ChevronUp aria-hidden="true" />
      </IconButton>
      <IconButton
        aria-label={labels.down}
        title={last ? "Đã ở cuối danh sách" : undefined}
        disabled={disabled || last}
        onClick={() => {
          onMove(1);
        }}
      >
        <ChevronDown aria-hidden="true" />
      </IconButton>
    </>
  );
}
