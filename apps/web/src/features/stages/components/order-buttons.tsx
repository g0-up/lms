import { ChevronDown, ChevronUp } from "lucide-react";
import { IconButton } from "@/shared/ui/icon-button";

export interface OrderButtonsProps {
  index: number;
  count: number;
  onMove: (delta: -1 | 1) => void;
  disabled?: boolean;
}

/** "Chuyển lên"/"Chuyển xuống" pair, disabled at either end of the list. */
export function OrderButtons({ index, count, onMove, disabled = false }: OrderButtonsProps) {
  const first = index === 0;
  const last = index === count - 1;
  return (
    <>
      <IconButton
        aria-label="Chuyển lên"
        title={first ? "Đã ở đầu danh sách" : undefined}
        disabled={disabled || first}
        onClick={() => {
          onMove(-1);
        }}
      >
        <ChevronUp aria-hidden="true" />
      </IconButton>
      <IconButton
        aria-label="Chuyển xuống"
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
