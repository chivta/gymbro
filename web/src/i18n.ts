// Single translation map: all user-facing text lives here.
export const t = {
  loading: "Loading…",
  allLoaded: "All workouts loaded",
  empty: "No workouts yet",
  retry: "Retry",
  kcal: "kcal",
  protein: "g protein",
  hour: "h",
  minute: "min",
  errors: {
    invalid_request: "Invalid request.",
    unauthorized: "Not authorized. Check the API secret.",
    user_not_found: "User not found.",
    internal: "Server error. Try again.",
    network: "Cannot reach the server.",
  },
} as const;

export type ErrorCode = keyof typeof t.errors;

/** Maps a backend error code to its translation key; the only such mapping. */
export function errorKey(code: string): ErrorCode {
  return code in t.errors ? (code as ErrorCode) : "internal";
}
