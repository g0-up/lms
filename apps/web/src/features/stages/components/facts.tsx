import type { ReactNode } from "react";

export interface Fact {
  label: string;
  value: ReactNode;
}

/** Label/value pairs of a settings card, as a description list that wraps into columns. */
export function Facts({ items }: { items: readonly Fact[] }) {
  return (
    <dl className="m-0 grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-x-6 gap-y-3">
      {items.map((fact) => (
        <div key={fact.label} className="grid gap-0.5">
          <dt className="text-xs text-ink-3">{fact.label}</dt>
          <dd className="m-0 text-sm text-ink">{fact.value}</dd>
        </div>
      ))}
    </dl>
  );
}
