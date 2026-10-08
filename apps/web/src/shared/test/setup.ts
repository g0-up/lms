import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";
import { server, setSession } from "./msw";

// Every request must hit a handler: an unexpected call is a test failure, never a real fetch.
beforeAll(() => {
  server.listen({ onUnhandledFrame: "error" });
});

// Vitest `globals` is off, so Testing Library cannot clean the DOM by itself.
afterEach(() => {
  cleanup();
  server.resetHandlers();
  setSession(null);
  window.history.replaceState(null, "", "/");
});

afterAll(() => {
  server.close();
});
