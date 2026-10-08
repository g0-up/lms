import { http } from "@/shared/api/http";
import {
  completionSchema,
  lessonPageSchema,
  myClassesSchema,
  roadmapSchema,
  type Completion,
  type LessonPage,
  type MyClasses,
  type Roadmap,
} from "../model/schemas";

const classPath = (classId: string) => `/me/classes/${encodeURIComponent(classId)}`;
const lessonPath = (classId: string, lessonId: string) =>
  `${classPath(classId)}/lessons/${encodeURIComponent(lessonId)}`;

export const learningApi = {
  myClasses: (signal?: AbortSignal): Promise<MyClasses> => http("/me/classes", { schema: myClassesSchema, signal }),
  myClass: (classId: string, signal?: AbortSignal): Promise<Roadmap> =>
    http(classPath(classId), { schema: roadmapSchema, signal }),
  /** Also records the first open of the lesson (FR-31) when it is not recorded yet. */
  lesson: (classId: string, lessonId: string, signal?: AbortSignal): Promise<LessonPage> =>
    http(lessonPath(classId, lessonId), { schema: lessonPageSchema, signal }),
  setCompletion: (classId: string, lessonId: string, completed: boolean): Promise<Completion> =>
    http(`${lessonPath(classId, lessonId)}/completion`, {
      method: "PUT",
      body: { completed },
      schema: completionSchema,
    }),
};

export const learningKeys = {
  myClasses: ["my-classes"] as const,
  myClass: (classId: string) => ["my-class", classId] as const,
  lesson: (classId: string, lessonId: string) => ["lesson", classId, lessonId] as const,
  lessonVideo: (lessonId: string) => ["lesson-video", lessonId] as const,
};
