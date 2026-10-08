import { useQuery } from "@tanstack/react-query";
import { meQuery } from "../api/auth-api";

/**
 * Current user (`null` when signed out). Guarded routes resolve it in middleware first,
 * so inside them `data` is already in the cache on first render.
 */
export function useMe() {
  return useQuery(meQuery);
}
