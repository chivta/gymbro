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
  appTitle: "gymbro",
  signIn: "Sign in with Telegram",
  signInHint: "Confirm the sign-in in the Telegram bot.",
  openTelegram: "Open Telegram",
  waitingConfirmation: "Waiting for confirmation in Telegram…",
  signOut: "Sign out",
  errors: {
    invalid_request: "Invalid request.",
    unauthorized: "Not signed in.",
    forbidden: "You do not have access to this.",
    login_invalid: "This sign-in link is invalid. Try again.",
    login_expired: "The sign-in expired. Try again.",
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
