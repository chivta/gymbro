import { errorKey, type ErrorCode } from "./i18n";

export const PAGE_SIZE = 20;
const USER_ID = import.meta.env.VITE_USER_ID ?? "1";

export interface WorkoutSet { weight: string; reps: number }
export interface Entry {
  position: number;
  name_as_written: string;
  exercise: string;
  sets: WorkoutSet[];
}
export interface Workout {
  id: number;
  performed_on: string;
  type: string | null;
  kcal: number | null;
  protein_g: number | null;
  note: string | null;
  started_at: string | null;
  finished_at: string | null;
  source: string;
  entries: Entry[];
}
export interface WorkoutPage { workouts: Workout[]; next_cursor: string | null }

/** Error carrying a translation key, never a message. */
export class ApiError extends Error {
  constructor(public code: ErrorCode) {
    super(code);
  }
}

/** Fetches one page of workouts (newest first) older than `before`. */
export async function fetchWorkouts(before: string | null, signal?: AbortSignal): Promise<WorkoutPage> {
  const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
  if (before) params.set("before", before);
  let res: Response;
  try {
    res = await fetch(`/api/v1/users/${USER_ID}/workouts?${params}`, { signal });
  } catch (e) {
    if (e instanceof DOMException && e.name === "AbortError") throw e;
    throw new ApiError("network");
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(errorKey(body?.error ?? "internal"));
  }
  return res.json();
}
