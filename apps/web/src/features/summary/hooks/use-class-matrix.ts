import { useQuery } from "@tanstack/react-query";
import { summaryApi, summaryKeys } from "../api/summary-api";

/** Every active member of a class against every stage; filtered on the client. */
export function useClassMatrix(classId: string) {
  return useQuery({
    queryKey: summaryKeys.matrix(classId),
    queryFn: ({ signal }) => summaryApi.matrix(classId, signal),
  });
}
