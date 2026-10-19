import { useEffect, useState } from "react";
import { remaining } from "../model/quiz-session";

interface Clock {
  /** Null when the attempt has no time limit. */
  deadlineAt: string | null;
  /** The server's time when the attempt was read. */
  serverNow: string;
  /** Client time the attempt arrived at (the query's `dataUpdatedAt`). */
  receivedAt: number;
}

/** Milliseconds left on the attempt's clock, ticking every second; null without a deadline. */
export function useCountdown({ deadlineAt, serverNow, receivedAt }: Clock): number | null {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (deadlineAt === null) return;
    const timer = window.setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => {
      window.clearInterval(timer);
    };
  }, [deadlineAt]);

  // A refetch newer than the last tick restarts the count from its own server time.
  return deadlineAt === null ? null : remaining(deadlineAt, serverNow, Math.max(now, receivedAt), receivedAt);
}
