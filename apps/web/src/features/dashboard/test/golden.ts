/**
 * Fixtures for the dashboard tests, straight from the API's golden JSON
 * (`apps/api/internal/features/reports/testdata`). Parsed with the app's Zod schema, so a contract
 * change on the API turns these tests red. Test-only: nothing in the app imports this module.
 */
import dashboardJson from "../../../../../api/internal/features/reports/testdata/dashboard.json";
import { dashboardSchema } from "../model/schemas";

export const fixtures = {
  dashboard: dashboardSchema.parse(dashboardJson),
};
