import { t } from "./i18n";
import type { Entry, Workout } from "./api";

const MS_PER_MIN = 60_000;
const MIN_PER_HOUR = 60;
const LOCALE = "en-GB";

/** "2026-10-04" -> "Sat, 4 Oct 2026" (parsed as a local date, no TZ shift). */
export function formatDate(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString(LOCALE, {
    weekday: "short", day: "numeric", month: "short", year: "numeric",
  }).replace(/^(\w+) /, "$1, ");
}

/** "13:36–14:37 · 1 h 1 min" in local time, or null if either time is missing. */
export function formatTimeRange(w: Workout): string | null {
  if (!w.started_at || !w.finished_at) return null;
  const a = new Date(w.started_at), b = new Date(w.finished_at);
  const hm = (d: Date) => d.toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" });
  const mins = Math.max(0, Math.round((b.getTime() - a.getTime()) / MS_PER_MIN));
  const h = Math.floor(mins / MIN_PER_HOUR), m = mins % MIN_PER_HOUR;
  const dur = [h ? `${h} ${t.hour}` : "", m || !h ? `${m} ${t.minute}` : ""].filter(Boolean).join(" ");
  return `${hm(a)}–${hm(b)} · ${dur}`;
}

/** "400 kcal · 14 g protein" with whichever parts exist, or null. */
export function formatIntake(w: Workout): string | null {
  const parts = [
    w.kcal != null ? `${w.kcal} ${t.kcal}` : "",
    w.protein_g != null ? `${w.protein_g} ${t.protein}` : "",
  ].filter(Boolean);
  return parts.length ? parts.join(" · ") : null;
}

/** "60×8, 7, 5; 40×10": consecutive same-weight sets share the weight. */
export function formatSets(sets: Entry["sets"]): string {
  const groups: { weight: string; reps: number[] }[] = [];
  for (const s of sets) {
    const last = groups[groups.length - 1];
    if (last && last.weight === s.weight) last.reps.push(s.reps);
    else groups.push({ weight: s.weight, reps: [s.reps] });
  }
  return groups.map((g) => `${g.weight}×${g.reps.join(", ")}`).join("; ");
}
