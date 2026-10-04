import { errorKey, type ErrorCode } from "./i18n";

export const PAGE_SIZE = 20;
const HTTP_NO_CONTENT = 204;

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

export interface Me { user_id: number }
export interface LoginStart { bot_url: string; expires_at: string }
export interface LoginPoll { status: "pending" | "confirmed"; user_id?: number }

/**
 * Sends a request to the API through the /api proxy. The browser attaches the
 * session cookie; non-2xx answers throw ApiError with the mapped code.
 */
async function request<T>(method: string, path: string, signal?: AbortSignal): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/api${path}`, { method, signal });
  } catch (e) {
    if (e instanceof DOMException && e.name === "AbortError") throw e;
    throw new ApiError("network");
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(errorKey(body?.error ?? "internal"));
  }
  return res.status === HTTP_NO_CONTENT ? (undefined as T) : res.json();
}

/** Current session's user; throws ApiError("unauthorized") when signed out. */
export const fetchMe = (signal?: AbortSignal) => request<Me>("GET", "/auth/me", signal);

/** Starts a Telegram sign-in; sets the login cookie and returns the bot link. */
export const startLogin = () => request<LoginStart>("POST", "/auth/telegram/login");

/** Polls the sign-in; on "confirmed" the session cookie is set. Throws login_expired when it can no longer succeed. */
export const pollLogin = () => request<LoginPoll>("POST", "/auth/telegram/poll");

export const logout = () => request<void>("POST", "/auth/logout");

/** Fetches one page of the user's workouts (newest first) older than `before`. */
export function fetchWorkouts(userId: number, before: string | null, signal?: AbortSignal): Promise<WorkoutPage> {
  const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
  if (before) params.set("before", before);
  return request<WorkoutPage>("GET", `/v1/users/${userId}/workouts?${params}`, signal);
}
