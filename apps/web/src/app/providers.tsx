import { QueryClientProvider, type QueryClient } from "@tanstack/react-query";
import { lazy, Suspense, type ReactNode } from "react";
import { TooltipProvider } from "@/shared/ui/tooltip";

// Devtools are imported only in development so they never reach the production bundle.
const ReactQueryDevtools = import.meta.env.DEV
  ? lazy(() => import("@tanstack/react-query-devtools").then((m) => ({ default: m.ReactQueryDevtools })))
  : null;

export function AppProviders({ queryClient, children }: { queryClient: QueryClient; children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={300}>{children}</TooltipProvider>
      {ReactQueryDevtools ? (
        <Suspense fallback={null}>
          <ReactQueryDevtools buttonPosition="bottom-left" />
        </Suspense>
      ) : null}
    </QueryClientProvider>
  );
}
