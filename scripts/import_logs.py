"""One-time import of logs_expanded.md (2024-2025 gym workouts) into the database.

Parses the whole file first and collects every line it cannot place, each with
a short reason label. With failures it prints them to stderr and exits 1;
without, it hands the workouts to confirm_and_write().
--report prints only the failure report and counts.

On top of the strict grammar this file gets three allowances:
- a prose line at the end of a workout goes to the workout note;
- a line with only an exercise name becomes an entry with no sets;
- text after the name or after the sets ("(з резинкою)", "— замінив на гантелі",
  "53 сек") goes to the workout note as "<name>: <text>".

Two weight corrections: a bodyweight exercise written at 0 kg gets BODYWEIGHT_KG,
and the home dumbbell workouts (type "руки") were logged per dumbbell, so their
weights are doubled.
"""

import argparse
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path

from decimal import Decimal

from import_common import (
    BODYWEIGHT_KG,
    ParseError,
    confirm_and_write,
    is_bodyweight_exercise,
    is_set_token,
    parse_exercise_line,
    parse_header_tail,
)

SOURCE = "logs_expanded"
LOG_PATH = Path(__file__).resolve().parent.parent / "logs_expanded.md"
HEADER_RE = re.compile(r"^(\d{2}\.\d{2}\.\d{4})(?:\s+(.*))?$")
DATE_FORMAT = "%d.%m.%Y"
HEADER_ANNOTATION = "←"
# A token starting with one of these ends the exercise name.
NAME_END_CHARS = "(—–~?"
REMAINDER_STRIP_CHARS = " —–"
COMMENT_MIN_WORDS = 4
COMMENT_PUNCTUATION = ",;"
NOTE_SEPARATOR = "; "
PER_DUMBBELL_TYPES = ("руки",)
DUMBBELL_COUNT = 2

NO_HEADER = "no_header"
BAD_HEADER = "bad_header"
NO_NAME = "no_name"
SETS_AFTER_TEXT = "sets_after_text"


def normalize(text):
    return " ".join(text.split()).lower()


def has_digit(token):
    return any(c.isdigit() for c in token)


def strict_names(lines):
    """Normalized names of every exercise line that fits the strict grammar.
    Used to tell a bare exercise name from a prose comment."""
    names = set()
    for line in lines:
        if not line.strip() or HEADER_RE.match(line):
            continue
        try:
            name, _ = parse_exercise_line(line)
        except ParseError:
            continue
        if not any(has_digit(t) for t in name.split()):
            names.add(normalize(name))
    return names


def is_comment(line, name, is_last, known_names, line_counts):
    """A last body line that reads as prose and does not start with a known exercise name."""
    if not is_last or normalize(name) in known_names:
        return False
    if name == line:
        return line_counts[normalize(line)] == 1
    return len(name.split()) >= COMMENT_MIN_WORDS or any(c in line for c in COMMENT_PUNCTUATION)


def parse_body_line(line, is_last, known_names, line_counts):
    """Returns (entry or None, note or None, failure label or None)."""
    tokens = line.split()
    name_end = next(
        (i for i, t in enumerate(tokens) if is_set_token(t) or has_digit(t) or t[0] in NAME_END_CHARS),
        len(tokens),
    )
    name = " ".join(tokens[:name_end])
    if is_comment(line, name, is_last, known_names, line_counts):
        return None, line.strip(), None
    if not name:
        return None, None, NO_NAME
    sets_end = name_end
    while sets_end < len(tokens) and is_set_token(tokens[sets_end]):
        sets_end += 1
    if any(is_set_token(t) for t in tokens[sets_end:]):
        return None, None, SETS_AFTER_TEXT
    sets = []
    if sets_end > name_end:
        _, sets = parse_exercise_line(" ".join(tokens[:sets_end]))
    remainder = " ".join(tokens[sets_end:]).strip(REMAINDER_STRIP_CHARS)
    if remainder.startswith("(") and remainder.endswith(")"):
        remainder = remainder[1:-1]
    note = f"{name}: {remainder}" if remainder else None
    return {"name_as_written": name, "sets": sets}, note, None


def correct_weights(entry, workout_type):
    """Applies the two weight corrections described in the module docstring."""
    for s in entry["sets"]:
        if is_bodyweight_exercise(entry["name_as_written"]) and Decimal(s["weight_kg"]) == 0:
            s["weight_kg"] = BODYWEIGHT_KG
        elif workout_type in PER_DUMBBELL_TYPES:
            s["weight_kg"] = f"{(Decimal(s['weight_kg']) * DUMBBELL_COUNT).normalize():f}"


def split_workouts(lines):
    """Groups numbered lines into [(header_no, header, [(no, line), ...])]; blank lines dropped.
    Lines before the first header go in a group with header None."""
    groups = []
    for no, line in enumerate(lines, start=1):
        if not line.strip():
            continue
        if HEADER_RE.match(line):
            groups.append((no, line, []))
        else:
            if not groups:
                groups.append((no, None, []))
            groups[-1][2].append((no, line))
    return groups


def parse_file(lines):
    """Returns (workouts, failures). failures is [(line_no, line, reason)]."""
    known_names = strict_names(lines)
    line_counts = Counter(normalize(l) for l in lines if l.strip())
    failures = []
    workouts = []
    seen_dates = Counter()
    for header_no, header, body in split_workouts(lines):
        if header is None:
            failures.append((header_no, body[0][1], NO_HEADER))
            continue
        date_text, tail = HEADER_RE.match(header).groups()
        tail = tail or ""
        try:
            iso = datetime.strptime(date_text, DATE_FORMAT).date().isoformat()
        except ValueError:
            failures.append((header_no, header, BAD_HEADER))
            continue
        if HEADER_ANNOTATION in tail:
            failures.append((header_no, header, BAD_HEADER))
            continue
        seen_dates[iso] += 1
        workout_type, kcal, protein_g, note = parse_header_tail(tail)
        notes = [note] if note else []
        exercises = []
        for i, (no, line) in enumerate(body):
            entry, line_note, failure = parse_body_line(line, i == len(body) - 1, known_names, line_counts)
            if failure:
                failures.append((no, line, failure))
            if entry:
                correct_weights(entry, workout_type)
                exercises.append(entry)
            if line_note:
                notes.append(line_note)
        workouts.append({
            "performed_on": iso,
            "workout_type": workout_type,
            "kcal": kcal,
            "protein_g": protein_g,
            "note": NOTE_SEPARATOR.join(notes) or None,
            "raw_text": "\n".join([header] + [l for _, l in body]),
            "source": SOURCE,
            "source_ref": iso if seen_dates[iso] == 1 else f"{iso}#{seen_dates[iso]}",
            "exercises": exercises,
        })
    return workouts, failures


def print_report(workouts, failures):
    by_reason = defaultdict(list)
    for no, line, reason in failures:
        by_reason[reason].append((no, line))
    for reason, items in sorted(by_reason.items(), key=lambda kv: (-len(kv[1]), kv[0])):
        print(f"\n{reason} ({len(items)})", file=sys.stderr)
        for no, line in items:
            print(f"  {no}: {line}", file=sys.stderr)
    print(f"\nworkouts parsed: {len(workouts)}, lines failed: {len(failures)}", file=sys.stderr)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--telegram-user-id", help="required unless --report")
    ap.add_argument("--report", action="store_true", help="print only the failure report and counts")
    args = ap.parse_args()
    if not args.report and not args.telegram_user_id:
        ap.error("--telegram-user-id is required")
    lines = LOG_PATH.read_text(encoding="utf-8").splitlines()
    workouts, failures = parse_file(lines)
    if args.report or failures:
        print_report(workouts, failures)
        sys.exit(1 if failures else 0)
    confirm_and_write(workouts, args.telegram_user_id)


if __name__ == "__main__":
    main()
