import { isRouteErrorResponse, useRouteError } from "react-router";
import { isApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { AuthLayout } from "./layouts/AuthLayout";
import { NotFound } from "./not-found";

/**
 * Last-resort boundary: a 404 response renders NotFound; anything else (API unreachable during a
 * route guard, a page chunk that failed to load after a deploy) offers a full reload.
 */
export function RouteError() {
  const error = useRouteError();
  if (isRouteErrorResponse(error) && error.status === 404) {
    return (
      <AuthLayout>
        <NotFound />
      </AuthLayout>
    );
  }
  if (import.meta.env.DEV) console.error(error);
  const message = isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE;
  return (
    <AuthLayout>
      <section className="grid justify-items-start gap-4">
        <h1 tabIndex={-1}>Không tải được trang</h1>
        <Alert variant="danger" role="alert" className="w-full">
          {message}
        </Alert>
        <Button
          variant="outline"
          onClick={() => {
            window.location.reload();
          }}
        >
          Tải lại trang
        </Button>
      </section>
    </AuthLayout>
  );
}
