import { useEffect, useState } from "react";
import { ApiError, pollLogin, startLogin } from "./api";
import { t, type ErrorCode } from "./i18n";
import { colors, radius, space } from "./theme";

const POLL_INTERVAL_MS = 2000;

type Step =
  | { step: "idle" }
  | { step: "waiting"; botUrl: string }
  | { step: "error"; code: ErrorCode };

/**
 * Sign in with Telegram: starts a login request, opens the bot deep link in a
 * new tab and polls every 2 s until the bot confirms or the request expires.
 */
export function SignIn({ onSignedIn }: { onSignedIn: (userId: number) => void }) {
  const [s, setS] = useState<Step>({ step: "idle" });

  const start = async () => {
    // Open the tab synchronously inside the click so popup blockers allow it,
    // then point it at the bot link once the API answers.
    const tab = window.open("", "_blank");
    if (tab) tab.opener = null;
    try {
      const login = await startLogin();
      if (tab) tab.location.href = login.bot_url;
      setS({ step: "waiting", botUrl: login.bot_url });
    } catch (e) {
      tab?.close();
      setS({ step: "error", code: e instanceof ApiError ? e.code : "internal" });
    }
  };

  // Poll while waiting. Each poll is scheduled after the previous one finishes,
  // so they never overlap; unmounting or leaving the waiting step stops it.
  const waiting = s.step === "waiting";
  useEffect(() => {
    if (!waiting) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const tick = async () => {
      try {
        const res = await pollLogin();
        if (stopped) return;
        if (res.status === "confirmed" && res.user_id) {
          onSignedIn(res.user_id);
          return;
        }
        timer = setTimeout(tick, POLL_INTERVAL_MS);
      } catch (e) {
        if (stopped) return;
        setS({ step: "error", code: e instanceof ApiError ? e.code : "internal" });
      }
    };
    timer = setTimeout(tick, POLL_INTERVAL_MS);
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [waiting, onSignedIn]);

  const button = {
    background: colors.accent, color: colors.onAccent, border: "none", borderRadius: radius,
    padding: `${space.md}px ${space.lg}px`, fontSize: 16, cursor: "pointer",
  } as const;

  return (
    <div style={{
      margin: "auto", display: "flex", flexDirection: "column", alignItems: "center",
      gap: space.md, padding: space.lg, textAlign: "center",
    }}>
      {s.step === "waiting" ? (
        <>
          <div>{t.waitingConfirmation}</div>
          <div style={{ color: colors.muted, fontSize: 13 }}>{t.signInHint}</div>
          <a href={s.botUrl} target="_blank" rel="noopener noreferrer" style={{ color: colors.accent }}>
            {t.openTelegram}
          </a>
        </>
      ) : (
        <>
          {s.step === "error" && <div style={{ color: colors.error }}>{t.errors[s.code]}</div>}
          <button onClick={start} style={button}>{s.step === "error" ? t.retry : t.signIn}</button>
        </>
      )}
    </div>
  );
}
