import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { summaryKeys } from "@/features/summary";
import { isApiError } from "@/shared/api/errors";
import { homeworkApi } from "../api/homework-api";
import { learningKeys } from "../api/learning-api";
import type { MySubmissions } from "../model/schemas";
import {
  submissionFieldErrors,
  submittedMessage,
  toSubmitBody,
  type SubmissionFormValues,
} from "../model/submission-form";

/** Upload progress of the file being sent: `index` counts from 0. */
export interface UploadProgress {
  index: number;
  total: number;
  pct: number;
}

/** Number of the pending submission the new one replaced, or null. */
function replacedNo({ submissions }: MySubmissions): number | null {
  const previous = submissions.at(1);
  return previous?.status === "superseded" ? previous.no : null;
}

/**
 * Submits a homework: uploads each picked file through `/me/uploads`, then posts the repo, commit and
 * media ids. A file already uploaded by a refused attempt is reused on retry, so fixing the repo URL
 * does not upload (and leave behind) the same file twice. Errors are left to the form.
 */
export function useSubmitHomework(classId: string, lessonId: string) {
  const queryClient = useQueryClient();
  const [progress, setProgress] = useState<UploadProgress | null>(null);
  const uploaded = useRef(new WeakMap<File, string>());
  const controller = useRef<AbortController | null>(null);
  const key = learningKeys.submissions(classId, lessonId);

  // Leaving the page cancels the upload in flight.
  useEffect(
    () => () => {
      controller.current?.abort();
    },
    [],
  );

  const mutation = useMutation({
    mutationFn: async (values: SubmissionFormValues) => {
      const abort = new AbortController();
      controller.current = abort;
      const mediaIds: string[] = [];
      try {
        for (const [index, file] of values.files.entries()) {
          let mediaId = uploaded.current.get(file);
          if (!mediaId) {
            setProgress({ index, total: values.files.length, pct: 0 });
            const media = await homeworkApi.uploadFile(file, {
              signal: abort.signal,
              onProgress: (pct) => {
                setProgress({ index, total: values.files.length, pct });
              },
            });
            mediaId = media.id;
            uploaded.current.set(file, mediaId);
          }
          mediaIds.push(mediaId);
        }
      } finally {
        setProgress(null);
      }
      return homeworkApi.submit(classId, lessonId, toSubmitBody(values, mediaIds));
    },
    meta: { silent: true },
    onSuccess: (result) => {
      uploaded.current = new WeakMap();
      queryClient.setQueryData(key, result);
      // The roadmap, side list and course summary label the homework by its latest submission.
      void queryClient.invalidateQueries({ queryKey: learningKeys.myClass(classId) });
      void queryClient.invalidateQueries({ queryKey: learningKeys.myClasses });
      void queryClient.invalidateQueries({ queryKey: summaryKeys.learner(classId) });
      const [latest] = result.submissions;
      toast.success(submittedMessage({ no: latest.no, isLate: latest.isLate, replaced: replacedNo(result) }));
    },
    onError: (error) => {
      // The server no longer takes those uploads (expired or already submitted): upload them again.
      if (submissionFieldErrors(error).mediaIds) uploaded.current = new WeakMap();
      // A refusal for the class or the result (ended, already passed) changes what the page should offer.
      if (isApiError(error) && error.status === 409) void queryClient.invalidateQueries({ queryKey: key });
    },
  });

  return { ...mutation, progress };
}
