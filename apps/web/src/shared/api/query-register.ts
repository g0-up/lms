import type { ApiError } from "./errors";

// Types React Query errors and mutation meta app-wide.
declare module "@tanstack/react-query" {
  interface Register {
    defaultError: ApiError | Error;
    mutationMeta: {
      /** The caller renders the error itself; skip the default error toast. */
      silent?: boolean;
    };
  }
}
