import { useParams, useSearchParams } from "react-router";
import { ReportPanel } from "@/features/reports";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Crumbs } from "@/shared/ui/crumbs";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { ClassTabs } from "../components/class-tabs";
import { LifecycleAction } from "../components/lifecycle-action";
import { SettingsTab } from "../components/settings-tab";
import { StudentsTab } from "../components/students-tab";
import { useClass } from "../hooks/use-class";
import { parseTab } from "../model/class-tabs";
import { fmtDay } from "../model/day";

const classesCrumb = { label: "Lớp học", to: "/admin/classes" };

export function Component() {
  const { classId = "" } = useParams();
  const [params] = useSearchParams();
  const tab = parseTab(params.get("tab"));
  const { data, error, isPending, refetch } = useClass(classId);
  useDocumentTitle(data?.code ?? "Lớp");

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
        <Crumbs items={[classesCrumb, { label: "Lớp" }]} />
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

  const version = data.courseVersion;
  return (
    <>
      <PageHead
        crumbs={[classesCrumb, { label: data.code }]}
        title={`${data.code} · ${data.name}`}
        badges={<StatusBadge status={data.status} />}
        lede={`${version.courseName} v${String(version.versionNo)} · ${fmtDay(data.startDate)} → ${fmtDay(data.endDate)} · Giảng viên ${data.teacher.name}`}
        actions={data.status === "ended" ? undefined : <LifecycleAction cls={data} />}
      />
      <ClassTabs current={tab} studentCount={data.memberCount} />
      {tab === "students" ? <StudentsTab cls={data} /> : null}
      {tab === "report" ? <ReportPanel classId={data.id} basePath={`/admin/classes/${data.id}?tab=report`} /> : null}
      {tab === "settings" ? <SettingsTab cls={data} /> : null}
    </>
  );
}
