#!/usr/bin/env python3
"""One-time import of the Telegram channel HTML export (messages.html) into
workouts. Parses the whole file first and collects every failure; with any
failure it prints them grouped by reason and exits 1. Otherwise it hands the
workouts to import_common.confirm_and_write().

    python3 import_telegram.py --report
    python3 import_telegram.py --telegram-user-id U </dev/null
"""

import argparse
import re
import sys
from collections import Counter, defaultdict
from datetime import date, datetime
from decimal import Decimal
from html.parser import HTMLParser
from pathlib import Path

from import_common import (
    BODYWEIGHT_KG,
    ParseError,
    confirm_and_write,
    is_bodyweight_exercise,
    name_key,
    parse_exercise_line,
    parse_header_tail,
)

SOURCE = "telegram_channel"
MESSAGES_FILE = Path(__file__).resolve().parent.parent / "messages.html"
POST_DATE_FORMAT = "%d %B %Y, %H:%M:%S"
HEADER_RE = re.compile(r"^(\d{1,2})\.(\d{1,2})(?:\s+(.*))?$")
MESSAGE_ID_RE = re.compile(r"^message(\d+)$")

REASON_NOT_A_WORKOUT = "not_a_workout"
REASON_BAD_HEADER = "bad_header"
REASON_NO_SETS = "no_sets"
REASON_BARE_REPS = "bare_reps"
REASON_MISSING_REPS = "missing_reps"
REASON_NEGATIVE_WEIGHT = "negative_weight"
REASON_DURATION = "duration"
REASON_TRAILING_TEXT = "trailing_text"
REASON_NO_NAME = "no_name"
REASON_ZERO_REPS = "zero_reps"
REASON_MALFORMED_TOKEN = "malformed_token"

BARE_NUMBER_RE = re.compile(r"^\d+(?:\.\d+)?$")
# A set written without a weight: bare reps ("підтягування 8 5 3") or a hang in
# seconds ("вис на турніку 30с 30с", seconds stored as reps). It gets BODYWEIGHT_KG
# for a bodyweight exercise and NO_ADDED_WEIGHT_KG otherwise (crunches count added weight only).
NO_ADDED_WEIGHT_KG = "0"
# Rear delt pulls up to July 2025 were logged at double the real load (both sides of
# the rope counted), so their weights are halved.
DOUBLE_COUNTED_EXERCISES = {
    "тяга на задню дельту", "тяга канату на задню дельту", "розведення канату на задню дельту",
}
DOUBLE_COUNT = 2
BODYWEIGHT_SET_RE = re.compile(r"^(\d+)с?$")
PAREN_RE = re.compile(r"\(([^)]*)\)")
NOTE_SEPARATOR = "; "
MISSING_REPS_RE = re.compile(r"^(?:\d+(?:\.\d+)?)?-$")
CHAINED_SET_RE = re.compile(r"^\d+(?:\.\d+)?-\d+-\d+$")
NEGATIVE_WEIGHT_RE = re.compile(r"^-\d+(?:\.\d+)?-\d+$")
DURATION_RE = re.compile(r"^\d+(?:\.\d+)?(?:с|c|s|m|min|хв|сек)$", re.IGNORECASE)


class MessageParser(HTMLParser):
    """Collects (message id, post timestamp, text) for every regular message."""

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.messages = []
        self.current = None
        self.in_date = False
        self.in_text = False
        self.text_depth = 0

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        classes = (a.get("class") or "").split()
        if tag == "div" and "message" in classes:
            m = MESSAGE_ID_RE.match(a.get("id") or "")
            if "default" in classes and m:
                self.current = {"id": int(m.group(1)), "posted": None, "parts": []}
                self.messages.append(self.current)
            else:
                self.current = None
        if self.current is None:
            return
        if tag == "div" and "date" in classes and "details" in classes:
            self.current["posted"] = datetime.strptime(a["title"], POST_DATE_FORMAT).date()
        elif tag == "div" and "text" in classes and not self.in_text:
            self.in_text = True
            self.text_depth = 1
        elif self.in_text:
            if tag == "br":
                self.current["parts"].append("\n")
            elif tag == "div":
                self.text_depth += 1

    def handle_endtag(self, tag):
        if self.in_text and tag == "div":
            self.text_depth -= 1
            if self.text_depth == 0:
                self.in_text = False

    def handle_data(self, data):
        if self.in_text:
            self.current["parts"].append(data)


def load_messages(path):
    """Returns [{"id", "posted", "text"}] for messages that have a text block."""
    parser = MessageParser()
    parser.feed(path.read_text(encoding="utf-8"))
    out = []
    for m in parser.messages:
        text = "".join(m["parts"]).strip("\n")
        if text.strip():
            out.append({"id": m["id"], "posted": m["posted"], "text": text})
    return out


def resolve_date(day, month, posted):
    """Post year, or the previous year if that date would be after the post date."""
    performed = date(posted.year, month, day)
    if performed > posted:
        performed = date(posted.year - 1, month, day)
    return performed


def classify_token(token):
    """Reason label for a token that is not a valid set token, or None if it looks like name text."""
    if NEGATIVE_WEIGHT_RE.match(token):
        return REASON_NEGATIVE_WEIGHT
    if MISSING_REPS_RE.match(token):
        return REASON_MISSING_REPS
    if DURATION_RE.match(token):
        return REASON_DURATION
    if CHAINED_SET_RE.match(token):
        return REASON_MALFORMED_TOKEN
    if BARE_NUMBER_RE.match(token):
        return REASON_BARE_REPS
    return None


def classify_line_error(err):
    """Maps a ParseError from parse_exercise_line to a stable reason label."""
    if err.reason == "no exercise name":
        return REASON_NO_NAME
    if err.reason == "zero reps":
        return REASON_ZERO_REPS
    if err.reason == "no set tokens":
        for token in err.line.split()[1:]:
            reason = classify_token(token)
            if reason:
                return reason
        return REASON_NO_SETS
    return classify_token(err.token) or REASON_TRAILING_TEXT


def parse_line(line):
    """Returns (name, sets, note). On top of the strict grammar: parenthesized text
    moves to the note as "<name>: <text>", and a name followed only by bare reps or
    seconds becomes sets at BODYWEIGHT_KG or NO_ADDED_WEIGHT_KG."""
    asides = PAREN_RE.findall(line)
    stripped = PAREN_RE.sub(" ", line)
    tokens = stripped.split()
    first = next((i for i, t in enumerate(tokens) if BODYWEIGHT_SET_RE.match(t)), None)
    if first and all(BODYWEIGHT_SET_RE.match(t) for t in tokens[first:]):
        name = " ".join(tokens[:first])
        weight = BODYWEIGHT_KG if is_bodyweight_exercise(name) else NO_ADDED_WEIGHT_KG
        sets = [
            {"weight_kg": weight, "reps": int(BODYWEIGHT_SET_RE.match(t).group(1))}
            for t in tokens[first:]
        ]
    else:
        name, sets = parse_exercise_line(stripped)
    if name_key(name) in DOUBLE_COUNTED_EXERCISES:
        for s in sets:
            s["weight_kg"] = f"{(Decimal(s['weight_kg']) / DOUBLE_COUNT).normalize():f}"
    note = NOTE_SEPARATOR.join(f"{name}: {a.strip()}" for a in asides) or None
    return name, sets, note


def parse_message(msg):
    """Returns (workout or None, [(reason, line, detail)]). A message whose
    first line is not a DD.MM header is not_a_workout; any failing line
    rejects the whole message."""
    lines = [l.strip() for l in msg["text"].split("\n") if l.strip()]
    m = HEADER_RE.match(lines[0])
    if not m:
        return None, [(REASON_NOT_A_WORKOUT, lines[0], len(lines))]
    day, month, tail = int(m.group(1)), int(m.group(2)), m.group(3) or ""
    try:
        performed = resolve_date(day, month, msg["posted"])
    except ValueError:
        return None, [(REASON_BAD_HEADER, lines[0], "invalid date")]
    workout_type, kcal, protein, note = parse_header_tail(tail)
    errors = []
    exercises = []
    for line in lines[1:]:
        try:
            name, sets, line_note = parse_line(line)
            exercises.append({"name_as_written": name, "sets": sets})
            if line_note:
                note = NOTE_SEPARATOR.join(filter(None, [note, line_note]))
        except ParseError as e:
            errors.append((classify_line_error(e), line, e.token))
    if len(lines) == 1:
        errors.append((REASON_NO_SETS, lines[0], "no exercise lines"))
    if errors:
        return None, errors
    return {
        "performed_on": performed.isoformat(),
        "workout_type": workout_type,
        "kcal": kcal,
        "protein_g": protein,
        "note": note,
        "raw_text": msg["text"],
        "source": SOURCE,
        "source_ref": str(msg["id"]),
        "exercises": exercises,
    }, []


def parse_all(messages):
    """Returns (workouts, failures, skipped, header_tails); failures are
    (reason, message id, line, detail); skipped are (id, posted, first line, line count)."""
    workouts, failures, skipped, tails = [], [], [], Counter()
    for msg in messages:
        workout, errors = parse_message(msg)
        if workout:
            workouts.append(workout)
            tails[HEADER_RE.match(msg["text"].strip().split("\n")[0].strip()).group(3) or ""] += 1
        elif errors[0][0] == REASON_NOT_A_WORKOUT:
            _, first, count = errors[0]
            skipped.append((msg["id"], msg["posted"], first, count))
        else:
            failures.extend((reason, msg["id"], line, detail) for reason, line, detail in errors)
    same_date = defaultdict(list)
    for msg in messages:
        m = HEADER_RE.match(msg["text"].strip().split("\n")[0].strip())
        if m:
            same_date[resolve_date(int(m.group(1)), int(m.group(2)), msg["posted"]) if _valid(m) else None].append(msg["id"])
    return workouts, failures, skipped, tails, same_date


def _valid(m):
    return 1 <= int(m.group(2)) <= 12 and 1 <= int(m.group(1)) <= 31


def print_failures(failures):
    by_reason = defaultdict(list)
    for reason, mid, line, detail in failures:
        by_reason[reason].append((mid, line, detail))
    for reason, items in sorted(by_reason.items()):
        print(f"\n{reason}: {len(items)} lines, {len({i[0] for i in items})} messages", file=sys.stderr)
        for mid, line, detail in items:
            print(f"  message{mid}: {line!r} [{detail}]", file=sys.stderr)


def print_report(messages, workouts, failures, skipped, tails, same_date):
    print_failures(failures)
    print(f"\nnot_a_workout: {len(skipped)} messages", file=sys.stderr)
    for mid, posted, first, count in skipped:
        print(f"  message{mid} {posted}: {first!r} ({count} lines)", file=sys.stderr)
    dupes = {d: ids for d, ids in same_date.items() if len(ids) > 1}
    print(f"\ndates shared by several messages: {len(dupes)}", file=sys.stderr)
    for d, ids in sorted(dupes.items(), key=lambda kv: str(kv[0])):
        print(f"  {d}: messages {ids}", file=sys.stderr)
    print("\nheader tails:", file=sys.stderr)
    for tail, count in sorted(tails.items(), key=lambda kv: (-kv[1], kv[0])):
        print(f"  {count:4d}  {tail!r} -> {parse_header_tail(tail)}", file=sys.stderr)
    print(
        f"\nmessages seen: {len(messages)}\nworkouts parsed: {len(workouts)}\n"
        f"messages skipped as non-workouts: {len(skipped)}\nlines failed: {len(failures)}",
        file=sys.stderr,
    )
    dates = sorted(w["performed_on"] for w in workouts)
    if dates:
        print(f"parsed date range: {dates[0]} .. {dates[-1]}", file=sys.stderr)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--report", action="store_true", help="print failures and counts, then exit")
    ap.add_argument("--telegram-user-id")
    ap.add_argument("--file", type=Path, default=MESSAGES_FILE)
    args = ap.parse_args()
    if not args.report and not args.telegram_user_id:
        ap.error("--telegram-user-id is required unless --report is given")

    messages = load_messages(args.file)
    workouts, failures, skipped, tails, same_date = parse_all(messages)
    if args.report:
        print_report(messages, workouts, failures, skipped, tails, same_date)
        return
    if failures:
        print_failures(failures)
        sys.exit(1)
    confirm_and_write(workouts, args.telegram_user_id)


if __name__ == "__main__":
    main()
