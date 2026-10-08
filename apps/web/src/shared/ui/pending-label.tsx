/**
 * Button label that swaps to a busy text without resizing the button: both labels share one grid
 * cell and the inactive one is `invisible` and `aria-hidden`, so the accessible name is the active one.
 */
export function PendingLabel({ pending, idle, busy }: { pending: boolean; idle: string; busy: string }) {
  return (
    <span className="grid">
      <span className={pending ? "invisible col-start-1 row-start-1" : "col-start-1 row-start-1"} aria-hidden={pending}>
        {idle}
      </span>
      <span className={pending ? "col-start-1 row-start-1" : "invisible col-start-1 row-start-1"} aria-hidden={!pending}>
        {busy}
      </span>
    </span>
  );
}
