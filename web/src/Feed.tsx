import { Fragment, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { fetchWorkouts, ApiError, type Workout } from "./api";
import { Bubble } from "./Bubble";
import { formatDate } from "./format";
import { t, type ErrorCode } from "./i18n";
import { colors, FONT, radius, space } from "./theme";

// Start loading older pages this far before the top is reached.
const TOP_MARGIN_PX = 200;

const centered = { alignSelf: "center", color: colors.muted, fontSize: 13, padding: space.sm } as const;

/** Chat feed of one user's workouts: newest at the bottom, older pages load when scrolling to the top. */
export function Feed({ userId }: { userId: number }) {
  // Stored newest-first as fetched; rendered reversed (oldest first).
  const [workouts, setWorkouts] = useState<Workout[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorCode | null>(null);

  const scroller = useRef<HTMLDivElement>(null);
  const sentinel = useRef<HTMLDivElement>(null);
  const busy = useRef(false);
  // scrollHeight before a prepend (null = initial load -> scroll to bottom).
  const prevHeight = useRef<number | null>(null);

  const loadMore = useCallback(async () => {
    if (busy.current || done) return;
    busy.current = true;
    setLoading(true);
    setError(null);
    try {
      const page = await fetchWorkouts(userId, cursor);
      const el = scroller.current;
      prevHeight.current = cursor && el ? el.scrollHeight : null;
      setWorkouts((prev) => [...prev, ...page.workouts]);
      setCursor(page.next_cursor);
      setDone(page.next_cursor === null);
    } catch (e) {
      setError(e instanceof ApiError ? e.code : "internal");
    } finally {
      busy.current = false;
      setLoading(false);
    }
  }, [userId, cursor, done]);

  // Fires on mount and whenever the sentinel re-enters view after a load.
  useEffect(() => {
    const el = sentinel.current;
    if (!el || error) return;
    const io = new IntersectionObserver(
      (entries) => entries[0].isIntersecting && loadMore(),
      { root: scroller.current, rootMargin: `${TOP_MARGIN_PX}px 0px 0px 0px` },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [loadMore, error, workouts.length]);

  // After render: first page -> jump to bottom; later pages -> keep position.
  useLayoutEffect(() => {
    const el = scroller.current;
    if (!el || workouts.length === 0) return;
    if (prevHeight.current === null) el.scrollTop = el.scrollHeight;
    else el.scrollTop += el.scrollHeight - prevHeight.current;
    prevHeight.current = null;
  }, [workouts]);

  const ordered = [...workouts].reverse();

  return (
    <div ref={scroller} style={{
      flex: 1, minHeight: 0, overflowY: "auto", background: colors.bg, fontFamily: FONT,
      display: "flex", flexDirection: "column", gap: space.sm,
      padding: space.lg, boxSizing: "border-box", overflowAnchor: "none",
    }}>
      <div ref={sentinel} style={{ height: 1, flexShrink: 0 }} />
      {loading && <div style={centered}>{t.loading}</div>}
      {done && workouts.length > 0 && <div style={centered}>{t.allLoaded}</div>}
      {done && workouts.length === 0 && <div style={centered}>{t.empty}</div>}
      {error && (
        <div style={{ ...centered, color: colors.error }}>
          {t.errors[error]}{" "}
          <button onClick={loadMore} style={{ color: colors.accent, background: "none", border: "none", cursor: "pointer" }}>
            {t.retry}
          </button>
        </div>
      )}
      {ordered.map((w, i) => (
        <Fragment key={w.id}>
          {(i === 0 || ordered[i - 1].performed_on !== w.performed_on) && (
            <div style={{
              ...centered, background: colors.chip, color: colors.text,
              borderRadius: radius, padding: `${space.xs}px ${space.md}px`, margin: `${space.sm}px 0`,
            }}>
              {formatDate(w.performed_on)}
            </div>
          )}
          <Bubble w={w} />
        </Fragment>
      ))}
    </div>
  );
}
