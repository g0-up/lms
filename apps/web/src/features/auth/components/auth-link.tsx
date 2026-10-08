import { Link, type LinkProps } from "react-router";
import { cn } from "@/shared/lib/cn";

/** Small secondary link of the auth forms with a 44px hit area. */
export function AuthLink({ className, ...props }: LinkProps) {
  return <Link className={cn("inline-flex min-h-11 items-center text-sm text-navy-700 underline hover:text-navy-900", className)} {...props} />;
}
