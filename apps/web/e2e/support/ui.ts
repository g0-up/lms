import type { Locator, Page } from "@playwright/test";

/** Giá trị của KpiCard có nhãn `label` (card: nhãn → giá trị → gợi ý). */
export function kpi(page: Page, label: string): Locator {
  return page
    .locator('[data-slot="card"]')
    .filter({ has: page.getByText(label, { exact: true }) })
    .locator("div")
    .nth(1);
}

/** Hàng của bảng chứa đoạn chữ (thường là email). */
export function rowWith(page: Page, text: string): Locator {
  return page.getByRole("row").filter({ hasText: text });
}
