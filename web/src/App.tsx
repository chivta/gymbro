import { useCallback, useEffect, useState } from "react";
import { ApiError, fetchMe, logout } from "./api";
import { Feed } from "./Feed";
import { t, type ErrorCode } from "./i18n";
import { SignIn } from "./SignIn";
import { colors, FONT, space } from "./theme";

type Auth =
  | { state: "loading" }
  | { state: "signedOut" }
  | { state: "signedIn"; userId: number }
  | { state: "error"; code: ErrorCode };

const linkButton = { color: colors.accent, background: "none", border: "none", cursor: "pointer", fontSize: 14 } as const;

/** Resolves the session via /auth/me, then shows the feed or the sign-in screen. */
export function App() {
  const [auth, setAuth] = useState<Auth>({ state: "loading" });

  const loadMe = useCallback(async (signal?: AbortSignal) => {
    setAuth({ state: "loading" });
    try {
      const me = await fetchMe(signal);
      setAuth({ state: "signedIn", userId: me.user_id });
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      const code = e instanceof ApiError ? e.code : "internal";
      setAuth(code === "unauthorized" ? { state: "signedOut" } : { state: "error", code });
    }
  }, []);

  useEffect(() => {
    const ctrl = new AbortController();
    loadMe(ctrl.signal);
    return () => ctrl.abort();
  }, [loadMe]);

  const onSignedIn = useCallback((userId: number) => setAuth({ state: "signedIn", userId }), []);

  const signOut = async () => {
    try {
      await logout();
    } catch {
      // The session is gone or unreachable either way; show the sign-in screen.
    }
    setAuth({ state: "signedOut" });
  };

  return (
    <div style={{
      height: "100dvh", display: "flex", flexDirection: "column",
      background: colors.bg, color: colors.text, fontFamily: FONT,
    }}>
      {auth.state === "signedIn" && (
        <header style={{
          display: "flex", justifyContent: "space-between", alignItems: "center",
          padding: `${space.sm}px ${space.lg}px`, background: colors.chip, flexShrink: 0,
        }}>
          <span style={{ fontWeight: 600 }}>{t.appTitle}</span>
          <button onClick={signOut} style={linkButton}>{t.signOut}</button>
        </header>
      )}
      {auth.state === "loading" && <div style={{ margin: "auto", color: colors.muted }}>{t.loading}</div>}
      {auth.state === "error" && (
        <div style={{ margin: "auto", color: colors.error }}>
          {t.errors[auth.code]}{" "}
          <button onClick={() => loadMe()} style={linkButton}>{t.retry}</button>
        </div>
      )}
      {auth.state === "signedOut" && <SignIn onSignedIn={onSignedIn} />}
      {auth.state === "signedIn" && <Feed key={auth.userId} userId={auth.userId} />}
    </div>
  );
}
