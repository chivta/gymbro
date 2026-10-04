import type { Workout } from "./api";
import { formatDate, formatIntake, formatSets, formatTimeRange } from "./format";
import { colors, MAX_BUBBLE_WIDTH, radius, space } from "./theme";

/** One workout as a chat bubble: header, intake, time, note, entries. */
export function Bubble({ w }: { w: Workout }) {
  const intake = formatIntake(w);
  const time = formatTimeRange(w);
  return (
    <div style={{
      background: colors.bubble, color: colors.text, borderRadius: radius,
      padding: space.md, maxWidth: MAX_BUBBLE_WIDTH, boxSizing: "border-box",
      alignSelf: "flex-start", width: "100%", lineHeight: 1.4,
    }}>
      <div style={{ fontWeight: 600, color: colors.accent }}>
        {formatDate(w.performed_on)}{w.type ? ` · ${w.type}` : ""}
      </div>
      {intake && <div style={{ color: colors.muted, fontSize: 13 }}>{intake}</div>}
      {time && <div style={{ color: colors.muted, fontSize: 13 }}>{time}</div>}
      {w.note && <div style={{ marginTop: space.sm, fontStyle: "italic" }}>{w.note}</div>}
      {w.entries.map((e) => (
        <div key={e.position} style={{ marginTop: space.sm }}>
          <div style={{ fontWeight: 500 }}>{e.name_as_written}</div>
          {e.exercise !== e.name_as_written && (
            <div style={{ color: colors.muted, fontSize: 13 }}>{e.exercise}</div>
          )}
          {e.sets.length > 0 && <div>{formatSets(e.sets)}</div>}
        </div>
      ))}
    </div>
  );
}
