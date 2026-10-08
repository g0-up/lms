import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { courseKeys, coursesApi, relatedKeys } from "../api/courses-api";
import type { CourseVersion } from "../model/schemas";

export const coursePath = (courseId: string) => `/admin/courses/${courseId}`;

/** Stage lists and details show which course versions use each stage version. */
function invalidateStages(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: relatedKeys.stages }),
    queryClient.invalidateQueries({ queryKey: relatedKeys.stageDetails }),
    queryClient.invalidateQueries({ queryKey: relatedKeys.stageVersions }),
  ]);
}

/** `POST /courses`; the dialog maps CONFLICT to the code field. */
export function useCreateCourse() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: coursesApi.create,
    meta: { silent: true },
    onSuccess: async (course) => {
      queryClient.setQueryData(courseKeys.detail(course.id), course);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: courseKeys.list }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
      ]);
    },
  });
}

/** Clone, publish, archive and delete of one course's versions; each refreshes the course, the lists and the dashboard. */
export function useCourseVersionActions(courseId: string) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const detailPath = coursePath(courseId);

  function refresh() {
    return Promise.all([
      queryClient.invalidateQueries({ queryKey: courseKeys.detail(courseId) }),
      queryClient.invalidateQueries({ queryKey: courseKeys.list }),
      queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
      invalidateStages(queryClient),
    ]);
  }

  function settle(version: CourseVersion) {
    queryClient.setQueryData(courseKeys.version(version.id), version);
    return refresh();
  }

  const clone = useMutation({
    mutationFn: coursesApi.clone,
    meta: { silent: true },
    onSuccess: async (version) => {
      await settle(version);
      toast.success("Đã tạo bản nháp mới (nhân bản nông).");
      await navigate(`${detailPath}?v=${version.id}`);
    },
    // DRAFT_EXISTS carries the server's own copy ("Khóa học đang có bản nháp v{n}…").
    onError: (error) => {
      toast.error(error.message);
      void refresh();
    },
  });

  // Publish and archive errors are shown inside their confirm dialog by the caller.
  const publish = useMutation({
    mutationFn: coursesApi.publish,
    meta: { silent: true },
    onSuccess: async (version) => {
      await settle(version);
      toast.success(`Đã phát hành v${String(version.versionNo)}.`);
    },
    onError: () => void refresh(),
  });

  const archive = useMutation({
    mutationFn: coursesApi.archive,
    meta: { silent: true },
    onSuccess: async (version) => {
      await settle(version);
      toast.success("Đã lưu trữ.");
    },
    onError: () => void refresh(),
  });

  const remove = useMutation({
    mutationFn: coursesApi.remove,
    meta: { silent: true },
    onSuccess: async ({ courseDeleted }, vid) => {
      toast.success("Đã xóa.");
      // Leave the page before dropping its caches so nothing refetches the deleted version.
      if (courseDeleted) {
        await navigate("/admin/courses");
        queryClient.removeQueries({ queryKey: courseKeys.detail(courseId) });
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: courseKeys.list }),
          queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
          invalidateStages(queryClient),
        ]);
      } else {
        await refresh();
        await navigate(detailPath);
      }
      queryClient.removeQueries({ queryKey: courseKeys.version(vid) });
    },
    onError: (error) => {
      toast.error(error.message);
      void refresh();
    },
  });

  return { clone, publish, archive, remove };
}
