import { QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { users } from "@/shared/test/msw";
import { createTestQueryClient } from "@/shared/test/render";
import { meKey } from "../api/auth-api";
import { useLogin } from "./use-login";

describe("useLogin", () => {
  it("clears the cache before storing the new user", async () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(["admin", "stages"], ["data of the previous user"]);
    const clear = vi.spyOn(queryClient, "clear");
    const setQueryData = vi.spyOn(queryClient, "setQueryData");
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useLogin(), { wrapper });
    act(() => {
      result.current.mutate({ email: users.teacher.email, password: "secret-pass" });
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(clear).toHaveBeenCalledOnce();
    const meWrite = setQueryData.mock.calls.findIndex(([key]) => JSON.stringify(key) === JSON.stringify(meKey));
    expect(meWrite).toBeGreaterThanOrEqual(0);
    expect(clear.mock.invocationCallOrder[0]).toBeLessThan(setQueryData.mock.invocationCallOrder[meWrite] ?? 0);
    expect(queryClient.getQueryData(["admin", "stages"])).toBeUndefined();
    expect(queryClient.getQueryData(meKey)).toMatchObject({ email: users.teacher.email });
  });
});
