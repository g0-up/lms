import { useEffect } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { toast } from "sonner";
import { HomeworkTab, QuizTab } from "@/features/grading";
import { SummaryTab } from "@/features/summary";
import { isApiError } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Crumbs } from "@/shared/ui/crumbs";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { TabBar } from "@/shared/ui/tab-bar";
import { ReportPanel } from "../components/report-panel";
import { useTeachClass } from "../hooks/use-teach-classes";
import { fmtDay } from "../model/day";
import { NOT_OWN_CLASS } from "../model/messages";
import { TEACH_CLASS_TABS, TEACH_CLASS_TAB_LABEL, parseTeachTab } from "../model/teach-class-tabs";

const myClassesCrumb = { label: "Lớp của tôi", to: "/teach" };

export function Component() {
  const { classId = "" } = useParams();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const tab = parseTeachTab(params.get("tab"));
  const { data, error, isPending, refetch } = useTeachClass(classId);
  useDocumentTitle(data?.code ?? "Lớp");

  // The server decides what a teacher may see; the page only leaves.
  const denied = isApiError(error) && (error.status === 403 || error.status === 404);
  useEffect(() => {
    if (!denied) return;
    toast.error(NOT_OWN_CLASS, { id: NOT_OWN_CLASS });
    void navigate("/teach", { replace: true });
  }, [denied, navigate]);

  if (denied) return null;
  if (isPending) {
    return (
      <div className="grid gap-6" aria-busy="true">
        <span className="sr-only">Đang tải lớp</span>
        <Skeleton className="h-9 w-1/2" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (error) {
    return (
      <>
        <Crumbs items={[myClassesCrumb]} />
        <Alert variant="danger" role="alert">
          {error.message}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </>
    );
  }

  return (
    <>
      <PageHead
        crumbs={[myClassesCrumb, { label: data.code }]}
        title={`${data.code} · ${data.name}`}
        badges={<StatusBadge status={data.status} />}
        lede={`${data.courseVersion.courseName} v${String(data.courseVersion.versionNo)} · ${fmtDay(data.startDate)} → ${fmtDay(data.endDate)}`}
      />
      <TabBar
        tabs={TEACH_CLASS_TABS.map((key) => ({ key, label: TEACH_CLASS_TAB_LABEL[key] }))}
        active={tab}
        hrefFor={(key) => `?tab=${key}`}
      />
      {tab === "report" ? <ReportPanel classId={data.id} basePath={`/teach/classes/${data.id}`} /> : null}
      {tab === "summary" ? <SummaryTab classId={data.id} basePath={`/teach/classes/${data.id}?tab=summary`} /> : null}
      {tab === "homework" ? <HomeworkTab classId={data.id} basePath="/teach" /> : null}
      {tab === "quiz" ? <QuizTab classId={data.id} basePath="/teach" /> : null}
    </>
  );
}
