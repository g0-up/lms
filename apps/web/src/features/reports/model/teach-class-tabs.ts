export const TEACH_CLASS_TABS = ["report", "summary", "homework", "quiz"] as const;
export type TeachClassTab = (typeof TEACH_CLASS_TABS)[number];

export const TEACH_CLASS_TAB_LABEL: Record<TeachClassTab, string> = {
  report: "Tiến độ",
  summary: "Tổng kết",
  homework: "Bài tập",
  quiz: "Quiz",
};

/** `?tab=` of the teacher's class page; a missing or unknown tab opens the report. */
export function parseTeachTab(value: string | null): TeachClassTab {
  return TEACH_CLASS_TABS.find((tab) => tab === value) ?? "report";
}
