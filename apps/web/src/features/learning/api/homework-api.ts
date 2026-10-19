import { http } from "@/shared/api/http";
import { uploadMedia, type Media } from "@/shared/api/upload-media";
import type { SubmitBody } from "../model/submission-form";
import { mySubmissionsSchema, type MySubmissions } from "../model/schemas";
import { lessonPath } from "./learning-api";

const submissionsPath = (classId: string, lessonId: string) => `${lessonPath(classId, lessonId)}/submissions`;

export const homeworkApi = {
  /** The student's submissions of one homework, newest first, with whether they may submit again. */
  mySubmissions: (classId: string, lessonId: string, signal?: AbortSignal): Promise<MySubmissions> =>
    http(submissionsPath(classId, lessonId), { schema: mySubmissionsSchema, signal }),
  /** 201 with the updated list; a pending earlier submission comes back `superseded`. */
  submit: (classId: string, lessonId: string, body: SubmitBody): Promise<MySubmissions> =>
    http(submissionsPath(classId, lessonId), { method: "POST", body, schema: mySubmissionsSchema }),
  /** Uploads one file of a submission through the student's own upload endpoint. */
  uploadFile: (file: File, options: { onProgress?: (pct: number) => void; signal?: AbortSignal } = {}): Promise<Media> =>
    uploadMedia("document", file, { ...options, path: "/me/uploads" }),
};
