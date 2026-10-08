import { Link } from "react-router";
import { useMe } from "@/features/auth";
import { homeOf } from "@/shared/domain";
import { Button } from "@/shared/ui/button";

/** Unknown path. Links to the role's home, or to `/` (which routes guests to /login). */
export function NotFound() {
  const { data: user } = useMe();
  return (
    <section className="grid justify-items-start gap-4">
      <h1 tabIndex={-1}>Không tìm thấy trang</h1>
      <Button asChild variant="outline">
        <Link to={user ? homeOf(user.role) : "/"}>Về trang chủ</Link>
      </Button>
    </section>
  );
}
