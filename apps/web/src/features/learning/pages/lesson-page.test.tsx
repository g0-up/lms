import { QueryClient } from "@tanstack/react-query";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api } from "@/shared/test/msw";
import { errorResponse } from "../api/msw-handlers";
import { errors, fixtures, ids } from "../test/golden";
import type { LessonPage } from "../model/schemas";
import { classPath, lessonPath, renderLearn } from "../test/render-learn";

const completionUrl = api("/me/classes/:classId/lessons/:lessonId/completion");
const [databaseStage, dataStructureStage] = fixtures.roadmapActive.stages;

async function renderLesson(classId: string, lessonId: string, options: Parameters<typeof renderLearn>[1] = {}) {
  const view = renderLearn(lessonPath(classId, lessonId), options);
  // Markdown lessons may carry their own h1 below the page's.
  await screen.findAllByRole("heading", { level: 1 });
  return view;
}

/** A lesson page response for a roadmap lesson the API has no golden file for. */
function pageFor(lessonIndex: number, over: Partial<LessonPage> = {}): LessonPage {
  const lesson = dataStructureStage.lessons[lessonIndex];
  return {
    ...fixtures.lessonMarkdown,
    lesson: { ...fixtures.lessonMarkdown.lesson, id: lesson.id, title: lesson.title, required: lesson.required, stage: { id: dataStructureStage.id, name: dataStructureStage.name } },
    progress: { firstOpenedAt: new Date().toISOString() },
    ...over,
  };
}

describe("LessonPage", () => {
  it("shows a markdown lesson with crumbs, navigation and the stage's lessons", async () => {
    await renderLesson(ids.active, ids.markdown);

    expect(screen.getAllByRole("heading", { level: 1, name: "Thiết kế bảng và khóa" })[0]).toBeInTheDocument();
    expect(screen.getByText("Mỗi bảng cần một khóa chính.")).toBeInTheDocument();

    const crumbs = within(screen.getByRole("navigation", { name: "Đường dẫn" }));
    expect(crumbs.getByRole("link", { name: "Lớp của tôi" })).toHaveAttribute("href", "/learn");
    expect(crumbs.getByRole("link", { name: "basic01" })).toHaveAttribute("href", classPath(ids.active));
    expect(crumbs.getByRole("link", { name: "Database" })).toHaveAttribute(
      "href",
      `${classPath(ids.active)}#stage-${databaseStage.id}`,
    );

    expect(screen.getByRole("link", { name: "Bài trước" })).toHaveAttribute("href", lessonPath(ids.active, ids.video));
    expect(screen.getByRole("link", { name: "Bài tiếp" })).toHaveAttribute("href", lessonPath(ids.active, ids.optional));

    const side = within(screen.getByRole("complementary", { name: "Học liệu chặng Database" }));
    expect(side.getByRole("link", { name: "Thiết kế bảng và khóa" })).toHaveAttribute("aria-current", "page");
    expect(side.getByRole("link", { name: /Giới thiệu SQL/ })).not.toHaveAttribute("aria-current");
    expect(side.getByRole("img", { name: "Đã học xong" })).toBeInTheDocument();
    expect(side.getByRole("link", { name: "Toàn bộ lộ trình" })).toHaveAttribute("href", classPath(ids.active));
  });

  it("plays a video lesson from the signed URL in the response", async () => {
    await renderLesson(ids.active, ids.video);

    const content = fixtures.lessonVideo.content;
    expect(content.type).toBe("video");
    expect(document.querySelector("video")).toHaveAttribute("src", content.type === "video" ? content.url : "");
    expect(screen.getByText("Đã tích 05/10/2026 15:00")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bỏ tích" })).not.toHaveAttribute("aria-disabled");
  });

  it("disables Bài trước on the first lesson, keeping its reason readable", async () => {
    await renderLesson(ids.active, ids.video);

    const prev = screen.getByRole("button", { name: "Bài trước" });
    expect(prev).toHaveAttribute("aria-disabled", "true");
    expect(prev).toHaveAttribute("title", "Đây là bài đầu tiên");
    expect(screen.getByRole("link", { name: "Bài tiếp" })).toBeInTheDocument();
  });

  it("disables Bài tiếp on the last lesson", async () => {
    const last = pageFor(2, { prev: { lessonId: dataStructureStage.lessons[1].id, title: "Stack và Queue" }, next: undefined });
    await renderLesson(ids.active, last.lesson.id, {
      overrides: [http.get(api(`/me/classes/${ids.active}/lessons/${last.lesson.id}`), () => HttpResponse.json(last))],
    });

    const next = screen.getByRole("button", { name: "Bài tiếp" });
    expect(next).toHaveAttribute("aria-disabled", "true");
    expect(next).toHaveAttribute("title", "Đây là bài cuối cùng");
    expect(screen.getByRole("link", { name: "Bài trước" })).toBeInTheDocument();
  });

  it("ticks the lesson and marks it in the side list", async () => {
    let body: unknown;
    const { user } = await renderLesson(ids.active, ids.markdown, {
      overrides: [
        http.put(completionUrl, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(fixtures.completion);
        }),
      ],
    });

    await user.click(screen.getByRole("button", { name: "Đã học xong" }));

    expect(await screen.findByText("Đã ghi nhận hoàn thành.")).toBeInTheDocument();
    expect(body).toEqual({ completed: true });
    expect(screen.getByRole("button", { name: "Bỏ tích" })).toBeInTheDocument();
    expect(screen.getByText("Đã tích 05/10/2026 15:00")).toBeInTheDocument();
    const side = within(screen.getByRole("complementary"));
    expect(side.getAllByRole("img", { name: "Đã học xong" })).toHaveLength(2);
  });

  it("reverts a tick refused because the class just ended", async () => {
    const { user } = await renderLesson(ids.active, ids.markdown, {
      overrides: [http.put(completionUrl, () => errorResponse(409, errors.classNotActive))],
    });

    await user.click(screen.getByRole("button", { name: "Đã học xong" }));

    expect(await screen.findByText(errors.classNotActive.error.message)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đã học xong" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Bỏ tích" })).toBeNull();
  });

  it("keeps an ended class's lesson readable but blocks the tick with a toast", async () => {
    let puts = 0;
    const { user } = await renderLesson(ids.ended, ids.optional, {
      overrides: [
        http.put(completionUrl, () => {
          puts += 1;
          return HttpResponse.json(fixtures.completion);
        }),
      ],
    });

    expect(screen.getByText("Không bắt buộc")).toBeInTheDocument();
    expect(screen.getByText("Chỉ mục giúp truy vấn nhanh hơn.")).toBeInTheDocument();
    const button = screen.getByRole("button", { name: "Đã học xong" });
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", "Lớp đã kết thúc");

    await user.click(button);

    expect(await screen.findByText("Lớp đã kết thúc")).toBeInTheDocument();
    expect(puts).toBe(0);
  });

  it("explains that a draft class's lessons open once it starts", async () => {
    const { user, router } = await renderLesson(ids.draft, ids.video);

    const status = await screen.findByRole("status");
    expect(status).toHaveTextContent("Lớp chưa bắt đầu. Bạn sẽ vào học được khi lớp kích hoạt.");
    expect(screen.queryByRole("button", { name: "Đã học xong" })).toBeNull();

    await user.click(within(status).getByRole("link", { name: "Về lộ trình" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe(classPath(ids.draft));
    });
    expect(await screen.findByText("Lớp chưa bắt đầu.")).toBeInTheDocument();
  });

  it("returns to the roadmap when the lesson is not in the class's course version", async () => {
    const { router } = renderLearn(lessonPath(ids.active, "01990000-0000-7000-8000-00000000beef"));

    expect(await screen.findByText(errors.lessonNotInCourse.error.message)).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(classPath(ids.active));
    });
  });

  it("returns to /learn when the student is no longer a member", async () => {
    const { router } = renderLearn(lessonPath("01990000-0000-7000-8000-00000000dead", ids.video));

    expect(await screen.findByText(errors.notMember.error.message)).toBeInTheDocument();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/learn");
    });
  });

  it("unlocks the roadmap tick as soon as the lesson has been opened", async () => {
    const opened = pageFor(1);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
    });
    const { user } = renderLearn(classPath(ids.active), {
      queryClient,
      overrides: [http.get(api(`/me/classes/${ids.active}/lessons/${opened.lesson.id}`), () => HttpResponse.json(opened))],
    });
    const box = await screen.findByRole("checkbox", { name: "Đã học xong: Stack và Queue" });
    expect(box).toBeDisabled();

    await user.click(screen.getByRole("link", { name: /Stack và Queue/ }));
    expect(await screen.findByRole("button", { name: "Đã học xong" })).not.toHaveAttribute("aria-disabled");
    await user.click(screen.getByRole("link", { name: "Toàn bộ lộ trình" }));

    expect(await screen.findByRole("checkbox", { name: "Đã học xong: Stack và Queue" })).toBeEnabled();
    expect(screen.getAllByText("Bài đọc · đã mở")).toHaveLength(2);
  });
});
