import { QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { createMemoryRouter } from "react-router";
import { RouterProvider } from "react-router/dom";
import { TooltipProvider } from "@/shared/ui/tooltip";
import { createAppQueryClient } from "./query-client";
import { buildRoutes } from "./router";

/**
 * Test helper: the real route tree, middleware and query client (401/403 handling included) in a
 * memory router starting at `path`. Retries are off so failures surface immediately.
 */
export function renderApp(path: string) {
  const queryClient = createAppQueryClient({
    navigate: (to) => {
      void router.navigate(to, { replace: true });
    },
    getLocation: () => router.state.location,
  });
  queryClient.setDefaultOptions({ queries: { ...queryClient.getDefaultOptions().queries, retry: false } });
  const router = createMemoryRouter(buildRoutes(queryClient), { initialEntries: [path] });
  const user = userEvent.setup();
  const view = render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <RouterProvider router={router} />
      </TooltipProvider>
    </QueryClientProvider>,
  );
  const location = () => router.state.location.pathname + router.state.location.search;
  return { ...view, router, queryClient, user, location };
}
