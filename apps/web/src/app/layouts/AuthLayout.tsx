import type { ReactNode } from "react";
import { Outlet } from "react-router";
import { BrandMark } from "@/shared/layout/brand-mark";
import { Card } from "@/shared/ui/card";

/** Centered 440px card with the GoUp LMS brand; no topbar. Used by the auth pages. */
export function AuthLayout({ children }: { children?: ReactNode }) {
  return (
    <main
      id="main"
      tabIndex={-1}
      className="flex items-start justify-center px-6 py-16 focus:outline-none max-[720px]:px-4"
    >
      <Card className="w-full max-w-[440px] p-8 max-[720px]:p-6">
        <div className="mb-8 flex items-center gap-3 text-navy-900">
          <BrandMark className="size-9" />
          <span className="font-display text-[22px] font-bold tracking-[-0.3px]">GoUp LMS</span>
        </div>
        {children ?? <Outlet />}
      </Card>
    </main>
  );
}
