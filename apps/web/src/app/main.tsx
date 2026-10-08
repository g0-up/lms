// Weight files declare every subset with its unicode-range, so the browser downloads only the
// latin + vietnamese files the text needs. The vietnamese-only subset files carry just the
// Vietnamese diacritic glyphs (no basic Latin) and no unicode-range, so they cannot be used alone.
import "@fontsource/inter-tight/400.css";
import "@fontsource/inter-tight/500.css";
import "@fontsource/inter-tight/600.css";
import "@fontsource/inter-tight/700.css";
import "@fontsource/inter-tight/400-italic.css";
import "@fontsource/manrope/600.css";
import "@fontsource/manrope/700.css";
import "@fontsource/roboto/400.css";
import "@fontsource/roboto/700.css";
import "@/styles/app.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter } from "react-router";
import { RouterProvider } from "react-router/dom";
import { AppProviders } from "./providers";
import { createAppQueryClient } from "./query-client";
import { buildRoutes } from "./router";

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Thiếu phần tử #root trong index.html");

// The query client and the router need each other: the client navigates on 401, the routes'
// middleware reads the current user from the client. Bind navigation once the router exists.
// Both callbacks run only after `router` below is initialised (no query runs before render).
const queryClient = createAppQueryClient({
  navigate: (to) => {
    void router.navigate(to, { replace: true });
  },
  getLocation: () => router.state.location,
});
const router = createBrowserRouter(buildRoutes(queryClient));

createRoot(rootElement).render(
  <StrictMode>
    <AppProviders queryClient={queryClient}>
      <RouterProvider router={router} />
    </AppProviders>
  </StrictMode>,
);
