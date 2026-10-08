import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError } from "@/shared/test/msw";
import { errorResponse } from "../api/msw-handlers";
import { errors, fixtures, ids } from "../test/golden";
import { classPath, gate, lessonPath, renderLearn } from "../test/render-learn";

const tick = (title: string) => screen.getByRole("checkbox", { name: `Đã học xong: ${title}` });
const blockerOf = (title: string) => tick(title).closest("label")?.getAttribute("title");
const completionUrl = api("/me/classes/:classId/lessons/:lessonId/completion");

async function renderRoadmap(classId = ids.active, overrides: Parameters<typeof renderLearn>[1] = {}) {
  const view = renderLearn(classPath(classId), overrides);
  await screen.findByRole("heading", { level: 1 });
  return view;
}

describe("LearnClassPage", () => {
  it("shows the class head, course progress and every stage with its lessons", async () => {
    await renderRoadmap();

    expect(screen.getByRole("heading", { level: 1, name: "Lập trình cơ bản – khóa 1" })).toBeInTheDocument();
    expect(screen.getByText("Lập trình cơ bản v1 · Giảng viên Lê Thu Hương")).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Đường dẫn" })).getByRole("link", { name: "Lớp của tôi" }))
      .toHaveAttribute("href", "/learn");
    expect(screen.getByText(/^1\/5 học liệu bắt buộc\./)).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "20% hoàn thành" })).toBeInTheDocument();

    expect(screen.getByRole("heading", { level: 2, name: "Database" })).toBeInTheDocument();
    expect(screen.getByText("3 học liệu · 1/2 bắt buộc đã xong")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Data structure" })).toBeInTheDocument();
    expect(screen.getByText("3 học liệu · 0/3 bắt buộc đã xong")).toBeInTheDocument();

    expect(screen.getByRole("link", { name: /Giới thiệu SQL/ })).toHaveAttribute("href", lessonPath(ids.active, ids.video));
    expect(screen.getByText("Video · 18:24")).toBeInTheDocument();
    expect(screen.getByText("Bài đọc · đã mở")).toBeInTheDocument();
    expect(screen.getByText("Bài đọc · không bắt buộc")).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("disables the tick of a lesson never opened, and explains why", async () => {
    await renderRoadmap();

    expect(tick("Stack và Queue")).toBeDisabled();
    expect(blockerOf("Stack và Queue")).toBe("Mở học liệu trước khi tích");
    expect(tick("Thiết kế bảng và khóa")).toBeEnabled();
    expect(tick("Giới thiệu SQL")).toBeChecked();
  });

  it("ticks a lesson and takes the course percent from the response", async () => {
    let body: unknown;
    const { user } = await renderRoadmap(ids.active, {
      overrides: [
        http.put(completionUrl, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(fixtures.completion);
        }),
      ],
    });

    await user.click(tick("Thiết kế bảng và khóa"));

    expect(await screen.findByText("Đã ghi nhận hoàn thành.")).toBeInTheDocument();
    expect(body).toEqual({ completed: true });
    expect(tick("Thiết kế bảng và khóa")).toBeChecked();
    expect(screen.getByRole("img", { name: "40% hoàn thành" })).toBeInTheDocument();
    expect(screen.getByText(/^2\/5 học liệu bắt buộc\./)).toBeInTheDocument();
    expect(screen.getByText("3 học liệu · 2/2 bắt buộc đã xong")).toBeInTheDocument();
  });

  it("unticks a completed lesson", async () => {
    const { user } = await renderRoadmap();

    await user.click(tick("Giới thiệu SQL"));

    expect(await screen.findByText("Đã bỏ tích.")).toBeInTheDocument();
    expect(tick("Giới thiệu SQL")).not.toBeChecked();
    expect(screen.getByRole("img", { name: "20% hoàn thành" })).toBeInTheDocument();
  });

  it("reverts and explains a tick the server refuses because the lesson was never opened", async () => {
    const { user } = await renderRoadmap(ids.active, {
      overrides: [http.put(completionUrl, () => errorResponse(409, errors.notOpened))],
    });

    await user.click(tick("Thiết kế bảng và khóa"));

    expect(await screen.findByText("Mở học liệu trước khi tích hoàn thành.")).toBeInTheDocument();
    expect(tick("Thiết kế bảng và khóa")).not.toBeChecked();
    expect(screen.getByRole("img", { name: "20% hoàn thành" })).toBeInTheDocument();
  });

  it("reverts and reloads the class when it ended meanwhile", async () => {
    let roadmapCalls = 0;
    const { user } = await renderRoadmap(ids.active, {
      overrides: [
        http.put(completionUrl, () => errorResponse(409, errors.classNotActive)),
        http.get(api(`/me/classes/${ids.active}`), () => {
          roadmapCalls += 1;
          return HttpResponse.json(roadmapCalls === 1 ? fixtures.roadmapActive : { ...fixtures.roadmapEnded, class: { ...fixtures.roadmapEnded.class, id: ids.active } });
        }),
      ],
    });

    await user.click(tick("Thiết kế bảng và khóa"));

    expect(await screen.findByText(errors.classNotActive.error.message)).toBeInTheDocument();
    expect(
      await screen.findByText("Lớp đã kết thúc: bạn vẫn xem được học liệu nhưng không tích hoàn thành được nữa."),
    ).toBeInTheDocument();
    expect(tick("Thiết kế bảng và khóa")).not.toBeChecked();
    expect(tick("Thiết kế bảng và khóa")).toBeDisabled();
    expect(roadmapCalls).toBe(2);
  });

  it("leaves for /learn when a tick finds the student is no longer a member", async () => {
    const { user, router } = await renderRoadmap(ids.active, {
      overrides: [http.put(completionUrl, () => errorResponse(404, errors.notMember))],
    });

    await user.click(tick("Thiết kế bảng và khóa"));

    expect(await screen.findByText(errors.notMember.error.message)).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/learn");
    });
  });

  it("shows an ended class read-only: lessons open, every tick disabled", async () => {
    await renderRoadmap(ids.ended);

    expect(
      screen.getByText("Lớp đã kết thúc: bạn vẫn xem được học liệu nhưng không tích hoàn thành được nữa."),
    ).toBeInTheDocument();
    for (const box of screen.getAllByRole("checkbox")) {
      expect(box).toBeDisabled();
      expect(box.closest("label")).toHaveAttribute("title", "Lớp đã kết thúc");
    }
    expect(screen.getByRole("link", { name: /Giới thiệu SQL/ })).toHaveAttribute("href", lessonPath(ids.ended, ids.video));
  });

  it("shows a draft class roadmap without lesson links", async () => {
    await renderRoadmap(ids.draft);

    expect(screen.getByText("Lớp chưa bắt đầu.")).toBeInTheDocument();
    expect(screen.getByText("Giới thiệu SQL")).toBeInTheDocument();
    expect(screen.getByText("Video · 18:24")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Giới thiệu SQL/ })).toBeNull();
    expect(screen.getAllByRole("link").map((a) => a.textContent)).toEqual(["Lớp của tôi"]);
    for (const box of screen.getAllByRole("checkbox")) {
      expect(box).toBeDisabled();
      expect(box.closest("label")).toHaveAttribute("title", "Lớp chưa bắt đầu");
    }
  });

  it("shows the empty state for a course version without lessons", async () => {
    await renderRoadmap(ids.active, {
      overrides: [
        http.get(api(`/me/classes/${ids.active}`), () =>
          HttpResponse.json({ ...fixtures.roadmapActive, stages: fixtures.roadmapActive.stages.map((s) => ({ ...s, lessons: [] })) }),
        ),
      ],
    });

    expect(screen.getByText("Chưa có học liệu")).toBeInTheDocument();
    expect(screen.getByText("Giảng viên chưa thêm học liệu cho lớp này.")).toBeInTheDocument();
  });

  it("shows a skeleton while loading", async () => {
    const response = gate();
    renderLearn(classPath(ids.active), {
      overrides: [
        http.get(api(`/me/classes/${ids.active}`), async () => {
          await response.wait();
          return HttpResponse.json(fixtures.roadmapActive);
        }),
      ],
    });

    expect(await screen.findByText("Đang tải lộ trình")).toBeInTheDocument();
    expect(screen.getByText("Đang tải lộ trình").parentElement).toHaveAttribute("aria-busy", "true");
    response.open();
    expect(await screen.findByRole("heading", { level: 1, name: "Lập trình cơ bản – khóa 1" })).toBeInTheDocument();
  });

  it("shows a server error with a retry", async () => {
    let calls = 0;
    const { user } = renderLearn(classPath(ids.active), {
      overrides: [
        http.get(api(`/me/classes/${ids.active}`), () => {
          calls += 1;
          return calls === 1 ? apiError(500, "INTERNAL", "Có lỗi xảy ra, vui lòng thử lại.") : HttpResponse.json(fixtures.roadmapActive);
        }),
      ],
    });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Có lỗi xảy ra, vui lòng thử lại.");
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Lập trình cơ bản – khóa 1" })).toBeInTheDocument();
  });

  it("sends a former member back to /learn", async () => {
    const { router } = renderLearn(classPath("01990000-0000-7000-8000-00000000dead"));

    expect(await screen.findByText(errors.notMember.error.message)).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/learn");
    });
    expect(await screen.findByRole("heading", { level: 1, name: "Xin chào, An" })).toBeInTheDocument();
  });
});
