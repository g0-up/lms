import { useLocation } from "react-router";
import type { GradingBase } from "../model/links";

/** Area the grading pages are rendered in; admins and teachers share them. */
export function useGradingBase(): GradingBase {
  return useLocation().pathname.startsWith("/admin") ? "/admin" : "/teach";
}
