export const CLASS_TABS = ["students", "report", "settings"] as const;
export type ClassTab = (typeof CLASS_TABS)[number];

export const CLASS_TAB_LABEL: Record<ClassTab, string> = {
  students: "Học viên",
  report: "Tiến độ",
  settings: "Cài đặt",
};

/** `?tab=` of the class page; a missing or unknown tab opens the students. */
export function parseTab(value: string | null): ClassTab {
  return CLASS_TABS.find((tab) => tab === value) ?? "students";
}
