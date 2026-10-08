import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { createMemoryRouter, type RouteObject } from "react-router";
import { RouterProvider } from "react-router/dom";
import { Toaster } from "@/shared/ui/sonner";
import { TooltipProvider } from "@/shared/ui/tooltip";

export function createTestQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}

export interface RenderRoutesOptions {
  initialEntries?: string[];
  queryClient?: QueryClient;
  /** Adds a Toaster next to the router (pages that toast). */
  withToaster?: boolean;
}

/** Renders `routes` in a memory data router with the app's providers. */
export function renderRoutes(routes: RouteObject[], options: RenderRoutesOptions = {}) {
  const queryClient = options.queryClient ?? createTestQueryClient();
  const router = createMemoryRouter(routes, { initialEntries: options.initialEntries ?? ["/"] });
  const user = userEvent.setup();
  const view = render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <RouterProvider router={router} />
        {options.withToaster ? <Toaster /> : null}
      </TooltipProvider>
    </QueryClientProvider>,
  );
  return { ...view, router, queryClient, user };
}
