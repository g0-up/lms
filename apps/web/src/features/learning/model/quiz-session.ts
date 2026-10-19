/** Time-limit warnings, in minutes left. */
export const WARNING_MINUTES = [5, 1] as const;
export type WarningMinute = (typeof WARNING_MINUTES)[number];

interface Answerable {
  id: string;
}

/** Answers picked so far by question id; an empty list is no answer. */
export type Answers = Readonly<Record<string, readonly string[] | undefined>>;

const isAnswered = (answers: Answers, questionId: string) => (answers[questionId]?.length ?? 0) > 0;

/** Questions without an answer yet. */
export function unansweredCount(questions: readonly Answerable[], answers: Answers): number {
  return questions.filter((q) => !isAnswered(answers, q.id)).length;
}

/**
 * Question to show when the attempt opens: the one the student was on (kept per tab) when it still exists,
 * else the first unanswered one, else the first.
 */
export function resumeIndex(questions: readonly Answerable[], answers: Answers, storedIndex: number | null): number {
  if (storedIndex !== null && Number.isInteger(storedIndex) && storedIndex >= 0 && storedIndex < questions.length) {
    return storedIndex;
  }
  const firstOpen = questions.findIndex((q) => !isAnswered(answers, q.id));
  return firstOpen < 0 ? 0 : firstOpen;
}

/**
 * Milliseconds left before `deadlineAt`, by the server's clock: `serverNow` was read when the attempt reached
 * the client at `receivedAt` (client clock), so a client clock that runs early or late does not move the
 * deadline. Never negative.
 */
export function remaining(deadlineAt: string, serverNow: string, clientNow: number, receivedAt: number): number {
  const serverAt = Date.parse(serverNow) + (clientNow - receivedAt);
  return Math.max(0, Date.parse(deadlineAt) - serverAt);
}

/** Warnings whose mark has been reached and that have not fired yet, largest first; none once time is up. */
export function warningsDue(remainingMs: number, fired: ReadonlySet<WarningMinute>): WarningMinute[] {
  if (remainingMs <= 0) return [];
  return WARNING_MINUTES.filter((m) => !fired.has(m) && remainingMs <= m * 60_000);
}

/** "mm:ss" of a countdown, rounded up so the last second still shows 00:01. */
export function fmtCountdown(ms: number): string {
  const total = Math.ceil(Math.max(0, ms) / 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(Math.floor(total / 60))}:${pad(total % 60)}`;
}

/** The answer after the student clicks `optionId`: a radio replaces, a checkbox toggles. */
export function toggleOption(type: "single" | "multi", current: readonly string[], optionId: string): string[] {
  if (type === "single") return [optionId];
  return current.includes(optionId) ? current.filter((id) => id !== optionId) : [...current, optionId];
}

const positionKey = (attemptId: string) => `quiz-position:${attemptId}`;

/** Question index the student was on in this tab, or null; storage may be unavailable (private mode). */
export function readPosition(attemptId: string): number | null {
  try {
    const raw = window.sessionStorage.getItem(positionKey(attemptId));
    if (raw === null) return null;
    const value = Number(raw);
    return Number.isInteger(value) ? value : null;
  } catch {
    return null;
  }
}

export function writePosition(attemptId: string, index: number): void {
  try {
    window.sessionStorage.setItem(positionKey(attemptId), String(index));
  } catch {
    // Without storage a reload resumes at the first unanswered question instead.
  }
}

export function clearPosition(attemptId: string): void {
  try {
    window.sessionStorage.removeItem(positionKey(attemptId));
  } catch {
    // Nothing to clear without storage.
  }
}
