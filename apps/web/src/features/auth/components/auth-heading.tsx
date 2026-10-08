import type { ReactNode } from "react";

/** Page title and lede inside the auth card (the brand comes from AuthLayout). */
export function AuthHeading({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <>
      <h1 tabIndex={-1} className="mb-1">
        {title}
      </h1>
      {children ? <p className="mb-6 text-ink-3">{children}</p> : null}
    </>
  );
}
