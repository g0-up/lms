import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { api, apiError, server } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { homeworkTabHandlers } from "../api/msw-handlers";
import type { ClassHomework } from "../model/schemas";
import { fixtures, ids } from "../test/golden";
import { polyfillRadix } from "../test/render-grading";
import { HomeworkTab } from "./homework-tab";

polyfillRadix();

const homeworkUrl = api("/classes/:classId/homework");
const deadlineUrl = api("/classes/:classId/deadlines/:lessonId");
const [item] = fixtures.classHomework.items;

function renderTab(data: ClassHomework = fixtures.classHomework, overrides: Parameters<typeof server.use> = []) {
  // Each `use` takes precedence over the previous ones.
  server.use(...homeworkTabHandlers);
  server.use(http.get(homeworkUrl, () => HttpResponse.json(data)));
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes(
    [{ path: "/teach/classes/:classId", element: <HomeworkTab classId={ids.classId} basePath="/teach" /> }],
    { initialEntries: [`/teach/classes/${ids.classId}`], withToaster: true },
  );
}

const withItem = (patch: Partial<ClassHomework["items"][number]>): ClassHomework => ({
  ...fixtures.classHomework,
  items: [{ ...item, ...patch }],
});

/** The counts line entry ending with `label`, e.g. "1 chờ chấm". */
const stat = (label: string) => within(card()).getByText(new RegExp(`${label}$`));

const card = () => screen.getByRole("region", { name: "Bài tập: truy vấn SELECT" });
const rowOf = (name: string) => {
  const row = within(card()).getByText(name).closest("tr");
  if (!row) throw new Error(`row of ${name}`);
  return within(row);
};

describe("Class homework tab", () => {
  it("shows each homework with its deadline, counts and members' latest submissions", async () => {
    renderTab();

    expect(await screen.findByRole("heading", { level: 2, name: "Bài tập: truy vấn SELECT" })).toBeInTheDocument();
    expect(within(card()).getByText("Database · 100 điểm · đạt từ 70%")).toBeInTheDocument();
    const deadline = within(card()).getByText(/^Hạn 05\/10\/2026 15:00 · quá hạn/);
    expect(deadline).toHaveClass("text-danger");
    expect(within(card()).getByRole("button", { name: "Đổi hạn nộp" })).toBeInTheDocument();
    expect(stat("đã nộp")).toHaveTextContent("2/2 đã nộp");
    expect(stat("chờ chấm")).toHaveTextContent("1 chờ chấm");
    expect(stat("quá hạn chưa nộp")).toHaveTextContent("0 quá hạn chưa nộp");

    const an = rowOf("Nguyễn Hoàng An");
    expect(an.getByText("Đạt")).toBeInTheDocument();
    expect(an.getByText("#2 · 05/10/2026 15:00")).toBeInTheDocument();
    expect(an.getByText("Nộp muộn").parentElement).toHaveTextContent("Nộp muộn · Repo + 1 file");
    expect(an.getByText("85/100")).toBeInTheDocument();
    expect(an.getByRole("link", { name: "Xem bài của Nguyễn Hoàng An: truy vấn SELECT" })).toHaveAttribute(
      "href",
      "/teach/grading/0199ffff-0000-7000-8000-000000000001",
    );

    const khang = rowOf("Vũ Đức Khang");
    expect(khang.getByText("Chờ chấm")).toBeInTheDocument();
    expect(khang.getByText("—")).toBeInTheDocument();
    expect(khang.queryByText(/chưa chấm/)).toBeNull();
    expect(khang.getByRole("link", { name: "Chấm bài của Vũ Đức Khang: truy vấn SELECT" })).toHaveAttribute(
      "href",
      `/teach/grading/${ids.memberPendingSubmission}`,
    );
  });

  it("marks members past the deadline without a submission and submissions waiting too long", async () => {
    const [passed, pending, missing] = fixtures.classHomeworkOverdue.items[0].members;
    renderTab({
      ...fixtures.classHomeworkOverdue,
      items: [{ ...fixtures.classHomeworkOverdue.items[0], members: [passed, { ...pending, waitingDays: 3 }, missing] }],
    });

    await screen.findByRole("heading", { level: 2, name: "Bài tập: truy vấn SELECT" });
    const trang = rowOf("Phạm Thu Trang");
    expect(trang.getByText("Quá hạn")).toBeInTheDocument();
    expect(trang.queryByRole("link")).toBeNull();
    expect(stat("quá hạn chưa nộp")).toHaveTextContent("1 quá hạn chưa nộp");
    expect(stat("đã nộp")).toHaveTextContent("2/3 đã nộp");
    expect(rowOf("Vũ Đức Khang").getByText("Quá 3 ngày chưa chấm")).toBeInTheDocument();
  });

  it("shows members without a deadline as not submitted yet", async () => {
    const [passed, pending, missing] = fixtures.classHomeworkOverdue.items[0].members;
    renderTab(withItem({ deadline: null, members: [passed, pending, missing] }));

    expect(await screen.findByText("Chưa đặt hạn nộp")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đặt hạn nộp" })).toBeInTheDocument();
    expect(rowOf("Phạm Thu Trang").getByText("Chưa nộp")).toBeInTheDocument();
  });

  it("sets a deadline in Vietnam time", async () => {
    const sent: unknown[] = [];
    const { user } = renderTab(withItem({ deadline: null }), [
      http.put(deadlineUrl, async ({ request, params }) => {
        sent.push({ lessonId: params.lessonId, body: await request.json() });
        return HttpResponse.json({ lessonId: params.lessonId, dueAt: "2026-10-20T16:59:00Z" });
      }),
    ]);

    await user.click(await screen.findByRole("button", { name: "Đặt hạn nộp" }));
    const dialog = await screen.findByRole("dialog", { name: "Hạn nộp: truy vấn SELECT" });
    expect(within(dialog).getByText("Lớp basic01. Hạn nộp chỉ áp dụng cho lớp này.")).toBeInTheDocument();
    expect(
      within(dialog).getByText("Theo giờ Việt Nam (GMT+7). Nộp sau hạn vẫn được nhận nhưng đánh dấu nộp muộn."),
    ).toBeInTheDocument();
    expect(within(dialog).queryByRole("checkbox", { name: "Bỏ hạn nộp" })).toBeNull();

    await user.click(within(dialog).getByRole("button", { name: "Lưu hạn nộp" }));
    expect(await within(dialog).findByText("Chọn ngày giờ hạn nộp.")).toBeInTheDocument();

    fireEvent.change(within(dialog).getByLabelText(/Hạn nộp/), { target: { value: "2026-10-20T23:59" } });
    await user.click(within(dialog).getByRole("button", { name: "Lưu hạn nộp" }));

    expect(await screen.findByText("Đã lưu hạn nộp.")).toBeInTheDocument();
    expect(sent).toEqual([{ lessonId: ids.lessonId, body: { dueAt: "2026-10-20T23:59:00+07:00" } }]);
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  it("changes and removes an existing deadline", async () => {
    let deleted = 0;
    const { user } = renderTab(fixtures.classHomework, [
      http.delete(deadlineUrl, () => {
        deleted += 1;
        return new HttpResponse(null, { status: 204 });
      }),
    ]);

    await user.click(await screen.findByRole("button", { name: "Đổi hạn nộp" }));
    const dialog = await screen.findByRole("dialog", { name: "Hạn nộp: truy vấn SELECT" });
    expect(within(dialog).getByLabelText(/Hạn nộp/)).toHaveValue("2026-10-05T15:00");
    await user.click(within(dialog).getByRole("checkbox", { name: "Bỏ hạn nộp" }));
    await user.click(within(dialog).getByRole("button", { name: "Lưu hạn nộp" }));

    expect(await screen.findByText("Đã bỏ hạn nộp.")).toBeInTheDocument();
    expect(deleted).toBe(1);
  });

  it("keeps the dialog open with the API's refusal", async () => {
    const { user } = renderTab(fixtures.classHomework, [
      http.put(deadlineUrl, () => apiError(409, "CLASS_ENDED", "Lớp đã kết thúc, không đổi hạn nộp được.")),
    ]);

    await user.click(await screen.findByRole("button", { name: "Đổi hạn nộp" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText(/Hạn nộp/), { target: { value: "2026-10-25T17:00" } });
    await user.click(within(dialog).getByRole("button", { name: "Lưu hạn nộp" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Lớp đã kết thúc, không đổi hạn nộp được.");
  });

  it("does not change deadlines of an ended class and says why", async () => {
    const { user } = renderTab(fixtures.classHomeworkEnded);

    const button = await screen.findByRole("button", { name: "Đổi hạn nộp" });
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", "Lớp đã kết thúc");
    await user.click(button);

    expect(await screen.findByText("Lớp đã kết thúc, không đổi hạn nộp được.")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(rowOf("Vũ Đức Khang").getByRole("link", { name: /^Chấm bài/ })).toBeInTheDocument();
  });

  it("says when the course has no homework", async () => {
    renderTab(fixtures.classHomeworkNone);

    expect(await screen.findByRole("heading", { name: "Khóa học không có bài tập" })).toBeInTheDocument();
    expect(screen.getByText("Phiên bản khóa học của lớp này không có học liệu loại bài tập.")).toBeInTheDocument();
  });

  it("says when the class has no members yet", async () => {
    renderTab(withItem({ members: [], stats: { submitted: 0, pending: 0, passed: 0, rework: 0, overdueNotSubmitted: 0 } }));

    expect(await screen.findByRole("heading", { name: "Chưa có học viên" })).toBeInTheDocument();
    expect(screen.getByText("Mời học viên vào lớp trước.")).toBeInTheDocument();
  });

  it("offers a retry when the homework does not load", async () => {
    let fail = true;
    const { user } = renderTab(fixtures.classHomework, [
      http.get(homeworkUrl, () => (fail ? apiError(500, "INTERNAL", "Lỗi máy chủ.") : HttpResponse.json(fixtures.classHomework))),
    ]);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Không tải được bài tập của lớp.");
    fail = false;
    await user.click(within(alert).getByRole("button", { name: "Thử lại" }));

    expect(await screen.findByRole("heading", { level: 2, name: "Bài tập: truy vấn SELECT" })).toBeInTheDocument();
  });
});
