import { describe, expect, it, vi } from "vitest";
import { toast } from "sonner";
import { ApiError } from "@/shared/api/errors";
import { users } from "@/shared/test/msw";
import { createAppQueryClient, shouldRetry } from "./query-client";

function setup(pathname = "/admin/stages", search = "") {
  const navigate = vi.fn<(to: string) => void>();
  const queryClient = createAppQueryClient({ navigate, getLocation: () => ({ pathname, search }) });
  queryClient.setDefaultOptions({ queries: { retry: false } });
  return { queryClient, navigate };
}

const fail = (error: Error) => () => Promise.reject(error);
const unauthorized = new ApiError(401, "UNAUTHENTICATED", "Vui lòng đăng nhập");

describe("createAppQueryClient", () => {
  it("clears the cache and goes to /login?next= when any query answers 401", async () => {
    const { queryClient, navigate } = setup("/admin/stages", "?q=a");
    queryClient.setQueryData(["auth", "me"], users.admin);
    queryClient.setQueryData(["admin", "courses"], ["secret"]);
    const clear = vi.spyOn(queryClient, "clear");

    await expect(queryClient.query({ queryKey: ["admin", "stages"], queryFn: fail(unauthorized) })).rejects.toBe(
      unauthorized,
    );

    expect(clear).toHaveBeenCalledOnce();
    expect(queryClient.getQueryData(["admin", "courses"])).toBeUndefined();
    expect(queryClient.getQueryData(["auth", "me"])).toBeNull();
    expect(navigate).toHaveBeenCalledWith("/login?next=%2Fadmin%2Fstages%3Fq%3Da");
  });

  it("clears and redirects on a 401 from a mutation too, without a toast", async () => {
    const { queryClient, navigate } = setup();
    const clear = vi.spyOn(queryClient, "clear");
    const toastError = vi.spyOn(toast, "error");

    await expect(
      queryClient.getMutationCache().build(queryClient, { mutationFn: fail(unauthorized) }).execute(undefined),
    ).rejects.toBe(unauthorized);

    expect(clear).toHaveBeenCalledOnce();
    expect(navigate).toHaveBeenCalledWith("/login?next=%2Fadmin%2Fstages");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("leaves a 401 on /login to the form (wrong credentials)", async () => {
    const { queryClient, navigate } = setup("/login");
    const clear = vi.spyOn(queryClient, "clear");

    await expect(queryClient.query({ queryKey: ["x"], queryFn: fail(unauthorized) })).rejects.toBe(unauthorized);

    expect(clear).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("marks the user and goes to /first-login on 403 PASSWORD_CHANGE_REQUIRED", async () => {
    const { queryClient, navigate } = setup("/learn");
    queryClient.setQueryData(["auth", "me"], users.student);
    const error = new ApiError(403, "PASSWORD_CHANGE_REQUIRED", "Bạn cần đổi mật khẩu.");

    await expect(queryClient.query({ queryKey: ["x"], queryFn: fail(error) })).rejects.toBe(error);

    expect(queryClient.getQueryData(["auth", "me"])).toMatchObject({ mustChangePassword: true });
    expect(navigate).toHaveBeenCalledWith("/first-login");
  });

  it("toasts other mutation errors unless the mutation is silent", async () => {
    const { queryClient, navigate } = setup();
    const toastError = vi.spyOn(toast, "error");
    const conflict = new ApiError(409, "CONFLICT", "Phiên bản đã được phát hành.");
    const cache = queryClient.getMutationCache();

    await expect(cache.build(queryClient, { mutationFn: fail(conflict) }).execute(undefined)).rejects.toBe(conflict);
    expect(toastError).toHaveBeenCalledWith("Phiên bản đã được phát hành.");

    toastError.mockClear();
    await expect(
      cache.build(queryClient, { mutationFn: fail(conflict), meta: { silent: true } }).execute(undefined),
    ).rejects.toBe(conflict);
    expect(toastError).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });
});

describe("shouldRetry", () => {
  it("never retries a 4xx answer", () => {
    expect(shouldRetry(0, new ApiError(400, "VALIDATION", "x"))).toBe(false);
    expect(shouldRetry(0, unauthorized)).toBe(false);
    expect(shouldRetry(0, new ApiError(404, "NOT_FOUND", "x"))).toBe(false);
  });

  it("retries network failures and 5xx twice", () => {
    const network = new ApiError(0, "NETWORK_ERROR", "x");
    const server = new ApiError(503, "HTTP_503", "x");
    expect([0, 1, 2].map((n) => shouldRetry(n, network))).toEqual([true, true, false]);
    expect([0, 1, 2].map((n) => shouldRetry(n, server))).toEqual([true, true, false]);
    expect(shouldRetry(0, new Error("boom"))).toBe(true);
  });
});
