import { http as mock, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { z } from "zod";
import { api, server } from "@/shared/test/msw";
import { ApiError, NETWORK_ERROR_MESSAGE } from "./errors";
import { http } from "./http";

const itemSchema = z.object({ id: z.string(), title: z.string() });

async function rejection(promise: Promise<unknown>): Promise<ApiError> {
  try {
    await promise;
  } catch (error) {
    if (error instanceof ApiError) return error;
    throw error;
  }
  throw new Error("expected the request to fail");
}

describe("http", () => {
  it("sends X-Requested-With and the session cookie policy on GET", async () => {
    let seen: Request | undefined;
    server.use(
      mock.get(api("/items/1"), ({ request }) => {
        seen = request;
        return HttpResponse.json({ id: "1", title: "Bài 1" });
      }),
    );

    await expect(http("/items/1", { schema: itemSchema })).resolves.toEqual({ id: "1", title: "Bài 1" });
    expect(seen?.headers.get("X-Requested-With")).toBe("fetch");
    expect(seen?.headers.get("Accept")).toBe("application/json");
    expect(seen?.credentials).toBe("same-origin");
  });

  it("sends X-Requested-With and a JSON body on POST", async () => {
    let headers: Headers | undefined;
    let body: unknown;
    server.use(
      mock.post(api("/items"), async ({ request }) => {
        headers = request.headers;
        body = await request.json();
        return HttpResponse.json({ id: "2", title: "Bài 2" }, { status: 201 });
      }),
    );

    await http("/items", { method: "POST", body: { title: "Bài 2" }, schema: itemSchema });
    expect(headers?.get("X-Requested-With")).toBe("fetch");
    expect(headers?.get("Content-Type")).toBe("application/json");
    expect(body).toEqual({ title: "Bài 2" });
  });

  it("resolves 204 to undefined without reading a body", async () => {
    server.use(mock.delete(api("/items/1"), () => new HttpResponse(null, { status: 204 })));
    await expect(http("/items/1", { method: "DELETE", schema: itemSchema })).resolves.toBeUndefined();
  });

  it("turns the error envelope into ApiError with the server's code, message and details", async () => {
    server.use(
      mock.post(api("/items"), () =>
        HttpResponse.json(
          { error: { code: "VALIDATION", message: "Tiêu đề không được trống.", details: { title: "required" } } },
          { status: 422 },
        ),
      ),
    );

    const error = await rejection(http("/items", { method: "POST", body: {} }));
    expect(error.status).toBe(422);
    expect(error.code).toBe("VALIDATION");
    expect(error.message).toBe("Tiêu đề không được trống.");
    expect(error.details).toEqual({ title: "required" });
  });

  it("falls back to the generic copy when the error body is not the envelope", async () => {
    server.use(mock.get(api("/items"), () => new HttpResponse("<html>Bad gateway</html>", { status: 502 })));

    const error = await rejection(http("/items"));
    expect(error.status).toBe(502);
    expect(error.code).toBe("HTTP_502");
    expect(error.message).toBe(NETWORK_ERROR_MESSAGE);
  });

  it("reports a payload that fails the schema as INVALID_RESPONSE naming the request", async () => {
    server.use(mock.get(api("/items/1"), () => HttpResponse.json({ id: 1 })));

    const error = await rejection(http("/items/1", { schema: itemSchema }));
    expect(error.code).toBe("INVALID_RESPONSE");
    expect(error.message).toContain("GET /items/1");
    expect(Array.isArray(error.details)).toBe(true);
  });

  it("maps a network failure to status 0 with the connection copy", async () => {
    server.use(mock.get(api("/items"), () => HttpResponse.error()));

    const error = await rejection(http("/items"));
    expect(error.status).toBe(0);
    expect(error.code).toBe("NETWORK_ERROR");
    expect(error.message).toBe(NETWORK_ERROR_MESSAGE);
  });

  it("lets an abort propagate instead of reporting a network error", async () => {
    server.use(mock.get(api("/items"), () => HttpResponse.json([])));
    const controller = new AbortController();
    controller.abort();

    await expect(http("/items", { signal: controller.signal })).rejects.toMatchObject({ name: "AbortError" });
  });
});
