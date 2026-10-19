import { http as mock, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, server } from "@/shared/test/msw";
import { ApiError } from "./errors";
import { UPLOAD_FAILED_MESSAGE, UploadFailed, uploadErrorMessage, uploadMedia } from "./upload-media";

const STORAGE_URL = "https://storage.mem/put/schema.sql?X-Amz-Signature=mem";
const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "PUT",
  "Access-Control-Allow-Headers": "Content-Type",
};

/** A document ticket as the API signs it: the content type comes from the file extension, not the browser. */
const ticket = {
  mediaId: "0199ffff-0000-7000-8000-000000000009",
  uploadUrl: STORAGE_URL,
  expiresAt: "2026-10-19T10:15:00+07:00",
  headers: { "Content-Type": "text/plain; charset=utf-8" },
};

const media = {
  id: ticket.mediaId,
  kind: "document",
  fileName: "schema.sql",
  contentType: "text/plain; charset=utf-8",
  sizeBytes: 12,
  status: "ready",
};

interface Seen {
  init?: unknown;
  putContentType?: string | null;
  putRequestedWith?: string | null;
  completed: boolean;
}

function storage(status = 200, path = "/media/uploads"): Seen {
  const seen: Seen = { completed: false };
  server.use(
    mock.post(api(path), async ({ request }) => {
      seen.init = await request.json();
      return HttpResponse.json(ticket, { status: 201 });
    }),
    mock.options("https://storage.mem/put/*", () => new HttpResponse(null, { status: 204, headers: CORS })),
    mock.put("https://storage.mem/put/*", ({ request }) => {
      seen.putContentType = request.headers.get("content-type");
      seen.putRequestedWith = request.headers.get("x-requested-with");
      return new HttpResponse(null, { status, headers: CORS });
    }),
    mock.post(api(`${path}/${ticket.mediaId}/complete`), () => {
      seen.completed = true;
      return HttpResponse.json(media);
    }),
  );
  return seen;
}

describe("uploadMedia", () => {
  it("sends the ticket's signed content type even when the browser knows no type for the file", async () => {
    const seen = storage();
    const file = new File(["SELECT 1;\n"], "schema.sql", { type: "" });

    await expect(uploadMedia("document", file)).resolves.toEqual(media);

    expect(seen.init).toEqual({ kind: "document", fileName: "schema.sql", contentType: "", sizeBytes: file.size });
    expect(seen.putContentType).toBe("text/plain; charset=utf-8");
    expect(seen.putRequestedWith).toBeNull();
    expect(seen.completed).toBe(true);
  });

  it("reserves and confirms a student's homework file through their own endpoint", async () => {
    const seen = storage(200, "/me/uploads");
    const file = new File(["SELECT 1;\n"], "schema.sql", { type: "" });

    await expect(uploadMedia("document", file, { path: "/me/uploads" })).resolves.toEqual(media);

    expect(seen.init).toEqual({ kind: "document", fileName: "schema.sql", contentType: "", sizeBytes: file.size });
    expect(seen.putContentType).toBe("text/plain; charset=utf-8");
    expect(seen.completed).toBe(true);
  });

  it("does not confirm an upload storage refused", async () => {
    const seen = storage(403);
    const file = new File(["%PDF-1.4"], "de-bai.pdf", { type: "application/pdf" });

    const error = await uploadMedia("document", file).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(UploadFailed);
    expect(uploadErrorMessage(error)).toBe(UPLOAD_FAILED_MESSAGE);
    expect(seen.completed).toBe(false);
  });

  it("surfaces the API's own reason when the reservation is refused", async () => {
    server.use(
      mock.post(api("/media/uploads"), () =>
        HttpResponse.json({ error: { code: "VALIDATION_FAILED", message: "Định dạng không hỗ trợ." } }, { status: 422 }),
      ),
    );
    const file = new File(["a,b"], "diem.csv", { type: "text/csv" });

    const error = await uploadMedia("document", file).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(uploadErrorMessage(error)).toBe("Định dạng không hỗ trợ.");
  });

  it("falls back to the generic message for an unknown failure", () => {
    expect(uploadErrorMessage(new Error("boom"))).toBe(UPLOAD_FAILED_MESSAGE);
  });
});
