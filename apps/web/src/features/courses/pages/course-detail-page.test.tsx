import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { beforeAll, describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { draftWorld, errorResponse, publishedWorld } from "../api/msw-handlers";
import type { CourseVersion } from "../model/schemas";
import { errors, fixtures, ids } from "../test/golden";
import { courseUrl, gate, recordRequests, renderCourses } from "../test/render-courses";

// The route is lazy: load it once up front so the first test's timeouts do not include the import.
beforeAll(async () => {
  await import("./course-detail-page");
});

function card(name: string): HTMLElement {
  const el = screen.getByRole("heading", { name }).closest<HTMLElement>('[data-slot="card"]');
  if (!el) throw new Error(`no card for ${name}`);
  return el;
}

async function stageCard(versionNo: number): Promise<HTMLElement> {
  return card((await screen.findByRole("heading", { name: `Chặng trong v${String(versionNo)}` })).textContent);
}

const putPath = `/course-versions/${ids.v2}/stages`;
const putBody = (requests: ReturnType<typeof recordRequests>) => requests.of("PUT", putPath)[0]?.body;

describe("CourseDetailPage: loading and errors", () => {
  it("shows a skeleton while the course loads", async () => {
    const response = gate();
    renderCourses(courseUrl(ids.course), {
      overrides: [
        http.get(api("/courses/:courseId"), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.courseDetail);
        }),
      ],
    });
    expect(await screen.findByText("Đang tải khóa học")).toBeInTheDocument();
    response.open();
    expect(await screen.findByRole("heading", { level: 1, name: "Lập trình cơ bản" })).toBeInTheDocument();
  });

  it("offers a retry when the course cannot load", async () => {
    let fail = true;
    const { user } = renderCourses(courseUrl(ids.course), {
      overrides: [
        http.get(api("/courses/:courseId"), () =>
          fail ? apiError(404, "NOT_FOUND", "Không tìm thấy khóa học.") : HttpResponse.json(fixtures.courseDetail),
        ),
      ],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tìm thấy khóa học.");
    expect(screen.getByRole("navigation", { name: "Đường dẫn" })).toHaveTextContent("Khóa học");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Lập trình cơ bản" })).toBeInTheDocument();
  });

  it("offers a retry when the version cannot load", async () => {
    let fail = true;
    const { user } = renderCourses(courseUrl(ids.course), {
      overrides: [
        http.get(api("/course-versions/:vid"), () =>
          fail ? apiError(500, "INTERNAL", "Có lỗi xảy ra.") : HttpResponse.json(fixtures.courseVersion),
        ),
      ],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Không tải được phiên bản này.");
    fail = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await stageCard(1)).toBeInTheDocument();
  });

  it("says when a course has no version at all", async () => {
    renderCourses(courseUrl(ids.course), {
      overrides: [http.get(api("/courses/:courseId"), () => HttpResponse.json({ ...fixtures.courseDetail, versions: [] }))],
    });
    expect(await screen.findByText("Khóa học này chưa có phiên bản nào.")).toBeInTheDocument();
  });
});

describe("CourseDetailPage: published version", () => {
  it("shows the latest published version read-only", async () => {
    renderCourses(courseUrl(ids.course));

    expect(await screen.findByRole("heading", { level: 1, name: "Lập trình cơ bản" })).toBeInTheDocument();
    expect(screen.getByText("BASIC")).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Đường dẫn" })).toHaveTextContent("Khóa học/Lập trình cơ bản");
    const stages = await stageCard(1);
    expect(within(stages).getByText("Đã phát hành")).toBeInTheDocument();
    expect(within(stages).getByText("Không sửa được")).toBeInTheDocument();
    expect(within(stages).queryByRole("button", { name: /Thêm chặng/ })).toBeNull();
    expect(within(stages).queryByRole("button", { name: "Gỡ khỏi khóa học" })).toBeNull();
    expect(within(stages).queryByRole("button", { name: /^Dùng v/ })).toBeNull();
    expect(within(stages).getByRole("link", { name: "Database" })).toHaveAttribute(
      "href",
      `/admin/stages/${ids.database}?v=${ids.databaseV1}`,
    );
    expect(within(stages).getByText("2 học liệu · 1 bắt buộc")).toBeInTheDocument();
    expect(within(stages).getByText("v2 đã phát hành")).toBeInTheDocument();
    expect(within(stages).getByText("1 học liệu bắt buộc toàn khóa")).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Lưu trữ" })).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Nhân bản thành bản nháp" })).toBeInTheDocument();
    expect(within(stages).queryByRole("button", { name: /Phát hành/ })).toBeNull();
    expect(document.title).toBe("Lập trình cơ bản v1 · GoUp LMS");
  });

  it("lists the classes attached to the viewed version", async () => {
    renderCourses(courseUrl(ids.course));
    await stageCard(1);
    const classes = card("Lớp gắn với v1");
    expect(within(classes).getByRole("link", { name: "basic01 · Lập trình cơ bản – khóa 1" })).toHaveAttribute(
      "href",
      `/admin/classes/${ids.basic01}`,
    );
    expect(within(classes).getByText("1 học viên")).toBeInTheDocument();
  });

  it("only toasts when deleting a version in use", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course));
    const button = await screen.findByRole("button", { name: "Xóa" });
    const reason = "Đang được dùng bởi lớp basic01. Hãy lưu trữ thay vì xóa.";
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", reason);

    await user.click(button);
    expect(await screen.findByText(reason)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(requests.of("DELETE", `/course-versions/${ids.v1}`)).toHaveLength(0);
  });

  it("archives after confirming", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course));
    await user.click(await screen.findByRole("button", { name: "Lưu trữ" }));
    const dialog = screen.getByRole("dialog", { name: "Lưu trữ v1?" });
    expect(dialog).toHaveTextContent("Lớp mới không gắn được phiên bản lưu trữ");

    await user.click(within(dialog).getByRole("button", { name: "Lưu trữ" }));
    expect(await screen.findByText("Đã lưu trữ.")).toBeInTheDocument();
    expect(requests.of("POST", `/course-versions/${ids.v1}/archive`)).toHaveLength(1);
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("opens the new draft after a shallow clone", async () => {
    const requests = recordRequests();
    const { user, router } = renderCourses(courseUrl(ids.course), { world: publishedWorld() });
    await user.click(await screen.findByRole("button", { name: "Nhân bản thành bản nháp" }));

    expect(await screen.findByText("Đã tạo bản nháp mới (nhân bản nông).")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.search).toBe(`?v=${ids.v2}`);
    });
    expect(requests.of("POST", `/course-versions/${ids.v1}/clone`)).toHaveLength(1);
  });

  it("toasts DRAFT_EXISTS when cloning", async () => {
    const { user } = renderCourses(courseUrl(ids.course), {
      overrides: [http.post(api("/course-versions/:vid/clone"), () => errorResponse(409, errors.draftExists))],
    });
    await user.click(await screen.findByRole("button", { name: "Nhân bản thành bản nháp" }));
    expect(await screen.findByText(errors.draftExists.error.message)).toBeInTheDocument();
  });

  it("links to the existing draft instead of cloning", async () => {
    renderCourses(courseUrl(ids.course, ids.v1), { world: draftWorld() });
    const stages = await stageCard(1);
    expect(within(stages).getByRole("link", { name: "Mở bản nháp v2" })).toHaveAttribute("href", courseUrl(ids.course, ids.v2));
    expect(within(stages).queryByRole("button", { name: "Nhân bản thành bản nháp" })).toBeNull();
  });
});

describe("CourseDetailPage: draft version", () => {
  it("shows the draft editable with publish and delete", async () => {
    renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld() });
    const stages = await stageCard(2);

    expect(within(stages).getByText("Nháp")).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Thêm chặng" })).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Gỡ khỏi khóa học" })).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Xóa bản nháp" })).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Phát hành v2" })).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByText(/^Chưa phát hành được/)).toBeNull();
    expect(screen.getByText("Chưa có lớp nào gắn với v2.")).toBeInTheDocument();
  });

  it("blocks publishing a draft without stages", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld(fixtures.courseVersionDraftEmpty) });
    const stages = await stageCard(2);
    expect(screen.getByText("Chưa phát hành được: Khóa học cần ít nhất một chặng.")).toBeInTheDocument();
    expect(within(stages).getByRole("heading", { name: "Chưa có chặng" })).toBeInTheDocument();
    expect(within(stages).getByText("0 học liệu bắt buộc toàn khóa")).toBeInTheDocument();
    const publish = within(stages).getByRole("button", { name: "Phát hành v2" });
    expect(publish).toHaveAttribute("aria-disabled", "true");

    await user.click(publish);
    expect(await screen.findByText("Khóa học cần ít nhất một chặng.", { selector: "[data-sonner-toast] *" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(requests.of("POST", `/course-versions/${ids.v2}/publish`)).toHaveLength(0);
  });

  it("names the attached stage versions that are not published yet", async () => {
    renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld(fixtures.courseVersionDraftUnpublishedStage) });
    const stages = await stageCard(2);
    expect(await screen.findByText("Chưa phát hành được: Chặng chưa phát hành: Database v2.")).toBeInTheDocument();
    expect(within(stages).getByRole("button", { name: "Phát hành v2" })).toHaveAttribute("aria-disabled", "true");
  });

  it("publishes after confirming", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld() });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Phát hành v2" }));
    const dialog = screen.getByRole("dialog", { name: "Phát hành Lập trình cơ bản v2?" });
    expect(dialog).toHaveTextContent("danh sách và thứ tự 1 chặng không sửa được");

    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));
    expect(await screen.findByText("Đã phát hành v2.")).toBeInTheDocument();
    expect(requests.of("POST", `/course-versions/${ids.v2}/publish`)).toHaveLength(1);
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("keeps the publish dialog open with the server's refusal", async () => {
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), {
      world: draftWorld(),
      overrides: [http.post(api("/course-versions/:vid/publish"), () => errorResponse(422, errors.validation))],
    });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Phát hành v2" }));
    const dialog = screen.getByRole("dialog", { name: "Phát hành Lập trình cơ bản v2?" });
    await user.click(within(dialog).getByRole("button", { name: "Phát hành" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(errors.validation.error.message);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("stays on the course after deleting a draft", async () => {
    const requests = recordRequests();
    const { user, router } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld() });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Xóa bản nháp" }));
    const dialog = screen.getByRole("dialog", { name: "Xóa Lập trình cơ bản v2?" });
    expect(dialog).toHaveTextContent("Các phiên bản chặng không bị ảnh hưởng.");
    await user.click(within(dialog).getByRole("button", { name: "Xóa phiên bản" }));

    expect(await screen.findByText("Đã xóa.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname + router.state.location.search).toBe(courseUrl(ids.course));
    });
    expect(requests.of("DELETE", `/course-versions/${ids.v2}`)).toHaveLength(1);
  });

  it("returns to the course list when deleting its last version", async () => {
    const { user, router } = renderCourses(courseUrl(ids.course, ids.v2), {
      world: draftWorld(),
      overrides: [http.delete(api("/course-versions/:vid"), () => HttpResponse.json({ courseDeleted: true }))],
    });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Xóa bản nháp" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Xóa phiên bản" }));

    expect(await screen.findByText("Đã xóa.")).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/admin/courses");
    });
  });
});

describe("CourseDetailPage: editing the draft's stages", () => {
  const twoStageDraft: CourseVersion = { ...fixtures.courseVersionDraft, stages: fixtures.stageRows };

  it("adds a published stage version picked from the stage list", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld(fixtures.courseVersionDraftEmpty) });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Thêm chặng" }));
    const dialog = await screen.findByRole("dialog", { name: "Thêm chặng vào khóa học" });
    expect(within(dialog).getByRole("combobox", { name: /Phiên bản chặng/ })).toHaveTextContent("Database v1 (mới nhất)");
    expect(dialog).toHaveTextContent("Chỉ liệt kê phiên bản đã phát hành của các chặng chưa có trong khóa học.");

    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));
    expect(await screen.findByText("Đã thêm chặng.")).toBeInTheDocument();
    expect(putBody(requests)).toBe(JSON.stringify({ stageVersionIds: [ids.databaseV1] }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("keeps the add dialog open with the server's refusal", async () => {
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), {
      world: draftWorld(fixtures.courseVersionDraftEmpty),
      overrides: [http.put(api("/course-versions/:vid/stages"), () => errorResponse(422, errors.validation))],
    });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Thêm chặng" }));
    const dialog = await screen.findByRole("dialog", { name: "Thêm chặng vào khóa học" });
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(errors.validation.error.message);
    expect(screen.queryByText("Đã thêm chặng.")).toBeNull();
  });

  it("toasts instead of opening the dialog when no stage is left to add", async () => {
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld() });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Thêm chặng" }));
    expect(await screen.findByText("Không còn chặng đã phát hành nào để thêm.")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("swaps an outdated stage to its newest published version", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), {
      world: draftWorld(fixtures.courseVersionDraftOutdated, fixtures.stageListV2Published),
    });
    const stages = await stageCard(2);
    expect(within(stages).getByText("v2 đã phát hành")).toBeInTheDocument();
    await user.click(within(stages).getByRole("button", { name: "Dùng v2" }));

    expect(await screen.findByText("Đã đổi sang phiên bản mới.")).toBeInTheDocument();
    expect(putBody(requests)).toBe(JSON.stringify({ stageVersionIds: [ids.databaseV2] }));
  });

  it("toasts when the newer stage version cannot be found", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld(fixtures.courseVersionDraftOutdated) });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Dùng v2" }));

    expect(await screen.findByText("Không tìm thấy phiên bản mới của chặng. Tải lại trang rồi thử lại.")).toBeInTheDocument();
    expect(requests.of("PUT", putPath)).toHaveLength(0);
  });

  it("removes a stage from the draft", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld() });
    await user.click(within(await stageCard(2)).getByRole("button", { name: "Gỡ khỏi khóa học" }));

    expect(await screen.findByText("Đã gỡ chặng khỏi bản nháp.")).toBeInTheDocument();
    expect(putBody(requests)).toBe(JSON.stringify({ stageVersionIds: [] }));
  });

  it("reorders optimistically and sends the new order", async () => {
    const requests = recordRequests();
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), { world: draftWorld(twoStageDraft) });
    const stages = await stageCard(2);
    expect(within(stages).getAllByRole("listitem")[0]).toHaveTextContent("v1");
    await user.click(within(stages).getAllByRole("button", { name: "Chuyển xuống" })[0]);

    await waitFor(() => {
      expect(putBody(requests)).toBe(JSON.stringify({ stageVersionIds: [ids.databaseV2, ids.databaseV1] }));
    });
  });

  it("explains VERSION_IMMUTABLE and rolls the edit back", async () => {
    const { user } = renderCourses(courseUrl(ids.course, ids.v2), {
      world: draftWorld(),
      overrides: [http.put(api("/course-versions/:vid/stages"), () => errorResponse(409, errors.versionImmutable))],
    });
    const stages = await stageCard(2);
    await user.click(within(stages).getByRole("button", { name: "Gỡ khỏi khóa học" }));

    expect(await screen.findByText("Phiên bản đã phát hành, không sửa được.")).toBeInTheDocument();
    expect(within(stages).getByRole("link", { name: "Database" })).toBeInTheDocument();
  });
});
