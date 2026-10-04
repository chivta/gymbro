"""Shared pieces of the one-time historical import scripts (import_logs.py,
import_telegram.py): the text log grammar from data_model.md, the JSON preview
with the y/n confirmation, and the database writer.

Each script parses its whole file into a list of workout dicts:

    {
        "performed_on": "2025-10-03",
        "workout_type": "upper" | None,
        "kcal": 680 | None,
        "protein_g": 37 | None,
        "note": "..." | None,
        "raw_text": "...",
        "source": "telegram_channel",
        "source_ref": "...",
        "exercises": [
            {"name_as_written": "підтягування",
             "exercise": "підтягування",  # added by apply_merges()
             "sets": [{"weight_kg": "82", "reps": 9}, ...]},
        ],
    }

and hands the list to confirm_and_write(). Writing needs psycopg
(pip install "psycopg[binary]") and DATABASE_URL in the environment.
"""

import json
import os
import re
import sys
from collections import Counter

WORKOUT_TYPES = (
    "lower", "upper", "push", "pull", "full body",
    # 2024-2025 names from logs_expanded.md
    "workout A", "workout B", "день 1", "день 2", "руки", "рест",
    "push+pull", "push+lower",
)
# Other spellings of a type, matched in the header and stored as the type on the right.
TYPE_ALIASES = {"pull+push": "push+pull", "legs": "lower"}
PLUS_RE = re.compile(r"\s*\+\s*")
NOTE_STRIP_CHARS = " ,;"
# Exercises logged with the user's own bodyweight as the weight. Where the source
# wrote no weight (or 0) for one of these, BODYWEIGHT_KG is recorded.
BODYWEIGHT_KG = "85"
BODYWEIGHT_EXERCISES = {"підтягування", "відтискання", "вузьке відтискання", "вис на турніку"}
IDENTITY_PROVIDER = "telegram"

# Different spellings of one exercise, decided in exercise_merges.md: exercise name -> names
# written in the logs that point to it. Entries keep the name as written; the writer also
# stores each variant as an alias so later saves resolve the same way.
EXERCISE_MERGES = {
    "жим лежачи": ["бенчпрес"],
    "жим в нахилі гантелі": ["жим гантелей в нахилі"],
    "жим в нахилі сміт": ["жим в машині сміта"],
    "жим над головою": ["вертикальний жим", "жим на плечі", "жим над головою гантелі", "жим на плечі гантелі"],
    "розведення гантелей": ["розведення гантелі"],
    "front raises": ["front raise"],
    "reverse fly": ["реверс флай"],
    "розведення гантелей ззаду": ["розведення гантелей назад"],
    "тяга на задню дельту": ["тяга на задні дельти"],
    "тяга канату на задню дельту": ["розведення канату на задню дельту"],
    "розведення канату": ["розведення канат"],
    "верт тяга": ["вертикальна тяга", "тяга вертикальна", "тяга верт", "тяга вниз", "тяга вниз спина",
                  "тяга канату вниз", "тяга канату вертикально", "тяга вертикально на спинку"],
    "гориз тяга": ["горизонтальна тяга", "тяга горизонтальна", "тяга гориз",
                   "тяга горизонтально на спинку", "тяга на спинку"],
    "гориз тяга канат": ["тяга канату горизонтально", "тяга канату"],
    "гориз тяга машина": ["гориз тяга в машині"],
    "гориз тяга штанга": ["гориз тяга штанги", "тяга штанги"],
    "рум тяга": ["румунська тяга", "румунська станова тяга", "rdl"],
    "згинання на біцепс": ["згин на біцепс", "скручування на біцепс", "жим на біцепс",
                           "згинання на біцепс штанга", "згинання штанги"],
    "згинання на біцепс гантелі": ["згинання гантелей", "скруч гантелей"],
    "згинання на біцепс канат": ["згинання канату"],
    "молоткове згинання": ["молотковий жим", "мол жим"],
    "preacher curls": ["preacher curl"],
    "розгин на тріцепс": ["розгинання на тріцепс", "розгин тріцепс", "розгинання тріцепс", "розгин тріц",
                          "тріцепс", "тріцепси", "тяга тріцепс", "тяга канату на тріцепс",
                          "трицепс опускання каната", "штовх вниз тріцепс", "штовх вниз на трицепси"],
    "розгин над головою канат": ["розгин над головою", "розгинання над головою", "розгин над головою тріцепс",
                                 "розгинання над головою тріцепс", "розгинання на тріцепс над головою",
                                 "розгинання над головою канат", "розгин на тріцепс над головою канат"],
    "французька тяга": ["франц тяга"],
    "присідання": ["присяд"],
    "розгин ніг": ["розгинання ніг", "розгин ногами"],
    "згин ніг": ["згинання ніг", "згин"],
    "жим ногами": ["прес ногами"],
    "підйом на ікри сидячи": ["підйоми на ікри сидячи"],
    "hip thrust": ["штовх бедрами"],
    "болгарські присідання": ["болгарські випади"],
    "скручування": ["прес", "скручування на прес"],
    "скручування в нахилі": ["скруч в нахилі", "прес в нахилі"],
    "скручування канат": ["скручування з канатом"],
    "скручування машина": ["скруч машина", "скручування на прес машина"],
    "поворот торсу машина": ["поворот тулубом машина", "повороти тулубом машина", "поворот торсу",
                             "повороти тулубом", "повороти торсом"],
    "скручування зап'ясть": ["скручування на зап'ястя", "згинання зап'ясть"],
}
MERGE_TARGETS = {variant: name for name, variants in EXERCISE_MERGES.items() for variant in variants}
# Calf raises written without a position: standing inside this period, sitting outside it.
# The target depends on the date, so these names get no alias.
CALF_NAMES = {"підйом на ікри", "підйоми на ікри", "ікри"}
CALF_STANDING = "підйом на ікри стоячи"
CALF_SITTING = "підйом на ікри сидячи"
CALF_STANDING_FROM = "2025-04-16"
CALF_STANDING_TO = "2025-08-31"
CONFIRM_ANSWER = "y"

# "W-R": set the current weight to W and record R reps. "-R": R reps at the current weight.
WEIGHT_REPS_RE = re.compile(r"^(\d+(?:\.\d{1,2})?)-(\d+)$")
REPS_RE = re.compile(r"^-(\d+)$")
# "(680/37)" or "(340/11 ккал/б перед тренуванням)": kcal/protein, optional trailing words.
INTAKE_RE = re.compile(r"\(\s*(\d+)\s*/\s*(\d+)([^)]*)\)")
TYPE_RE = re.compile(
    r"(?<!\S)(" + "|".join(re.escape(t) for t in sorted((*WORKOUT_TYPES, *TYPE_ALIASES), key=len, reverse=True)) + r")(?![^\s,;])", re.IGNORECASE
)
# Mirrors the name_key / alias_key expression in 001_init.sql.
NAME_KEY_SQL = r"lower(regexp_replace(btrim(%s), '\s+', ' ', 'g'))"


class ParseError(Exception):
    """A line that does not fit the grammar. Carries the line and the offending token."""

    def __init__(self, line, token, reason):
        super().__init__(f"{reason}: token {token!r} in line {line!r}")
        self.line = line
        self.token = token
        self.reason = reason


def is_set_token(token):
    return bool(WEIGHT_REPS_RE.match(token) or REPS_RE.match(token))


def parse_exercise_line(line):
    """Strict grammar from data_model.md. Returns (name_as_written, sets).
    The name is everything before the first set token; every token after it
    must be a set token. Raises ParseError otherwise."""
    tokens = line.split()
    first = next((i for i, t in enumerate(tokens) if is_set_token(t)), None)
    if first is None:
        raise ParseError(line, "", "no set tokens")
    if first == 0:
        raise ParseError(line, tokens[0], "no exercise name")
    name = " ".join(tokens[:first])
    sets = []
    weight = "0"
    for token in tokens[first:]:
        m = WEIGHT_REPS_RE.match(token)
        if m:
            weight, reps = m.group(1), int(m.group(2))
        else:
            m = REPS_RE.match(token)
            if not m:
                raise ParseError(line, token, "not a set token")
            reps = int(m.group(1))
        if reps == 0:
            raise ParseError(line, token, "zero reps")
        sets.append({"weight_kg": weight, "reps": reps})
    return name, sets


def is_bodyweight_exercise(name):
    return name_key(name) in BODYWEIGHT_EXERCISES


def name_key(name):
    """Same normalization as name_key / alias_key in 001_init.sql."""
    return " ".join(name.split()).lower()


def merged_exercise(name, performed_on):
    """The exercise an entry written as `name` on ISO date `performed_on` points to."""
    key = name_key(name)
    if key in CALF_NAMES:
        return CALF_STANDING if CALF_STANDING_FROM <= performed_on <= CALF_STANDING_TO else CALF_SITTING
    return MERGE_TARGETS.get(key, name)


def apply_merges(workouts):
    """Adds "exercise" to every entry: the name of the exercise it points to."""
    for w in workouts:
        for e in w["exercises"]:
            e["exercise"] = merged_exercise(e["name_as_written"], w["performed_on"])


def parse_header_tail(tail):
    """Splits the header text after the date into (workout_type, kcal, protein_g, note).
    The type is the first standalone word from WORKOUT_TYPES or TYPE_ALIASES; the intake block is
    "(kcal/protein)"; everything left over becomes the note."""
    kcal = protein = None
    m = INTAKE_RE.search(tail)
    if m:
        kcal, protein = int(m.group(1)), int(m.group(2))
        tail = tail[: m.start()] + " " + tail[m.end() :]
    workout_type = None
    tail = PLUS_RE.sub("+", tail)
    m = TYPE_RE.search(tail)
    if m:
        found = m.group(1).lower()
        found = TYPE_ALIASES.get(found, found)
        workout_type = next(t for t in WORKOUT_TYPES if t.lower() == found)
        tail = tail[: m.start()] + " " + tail[m.end() :]
    note = " ".join(tail.split()).strip(NOTE_STRIP_CHARS) or None
    return workout_type, kcal, protein, note


def print_summary(workouts):
    """Totals plus every distinct exercise (after merges) with its entry count, so typos stand out."""
    names = Counter()
    display = {}
    sets = 0
    for w in workouts:
        for e in w["exercises"]:
            key = name_key(e["exercise"])
            names[key] += 1
            display.setdefault(key, e["exercise"])
            sets += len(e["sets"])
    dates = sorted(w["performed_on"] for w in workouts)
    print(f"\n{len(workouts)} workouts, {sum(names.values())} entries, {sets} sets", file=sys.stderr)
    if dates:
        print(f"from {dates[0]} to {dates[-1]}", file=sys.stderr)
    print(f"{len(names)} distinct exercises:", file=sys.stderr)
    for key, count in sorted(names.items(), key=lambda kv: (-kv[1], kv[0])):
        print(f"  {count:4d}  {display[key]}", file=sys.stderr)


def confirm_and_write(workouts, telegram_user_id):
    """Prints the parsed workouts as JSON, asks for confirmation, then writes.
    Nothing touches the database unless the answer is exactly CONFIRM_ANSWER."""
    apply_merges(workouts)
    print(json.dumps(workouts, ensure_ascii=False, indent=2))
    print_summary(workouts)
    print(f"\nwrite to the database? [{CONFIRM_ANSWER}/N] ", end="", file=sys.stderr, flush=True)
    try:
        answer = input()
    except EOFError:
        answer = ""
    if answer != CONFIRM_ANSWER:
        print("nothing written", file=sys.stderr)
        return
    created = write_workouts(workouts, telegram_user_id)
    print(f"wrote {len(workouts)} workouts", file=sys.stderr)
    if created:
        print("new exercises: " + ", ".join(created), file=sys.stderr)


def write_workouts(workouts, telegram_user_id):
    """Writes everything in one transaction. A workout with an existing
    (user, source, source_ref) is replaced. Returns names of exercises created."""
    import psycopg

    created = []
    with psycopg.connect(os.environ["DATABASE_URL"]) as conn, conn.cursor() as cur:
        user_id = find_or_create_user(cur, telegram_user_id)
        exercise_ids = {}
        aliases = {}
        for w in workouts:
            cur.execute(
                "DELETE FROM workouts WHERE user_id = %s AND source = %s AND source_ref = %s",
                (user_id, w["source"], w["source_ref"]),
            )
            cur.execute(
                "INSERT INTO workouts (user_id, performed_on, workout_type, kcal, protein_g,"
                " note, raw_text, source, source_ref)"
                " VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s) RETURNING id",
                (user_id, w["performed_on"], w["workout_type"], w["kcal"], w["protein_g"],
                 w["note"], w["raw_text"], w["source"], w["source_ref"]),
            )
            workout_id = cur.fetchone()[0]
            for position, e in enumerate(w["exercises"], start=1):
                name = e["name_as_written"]
                key = name_key(e["exercise"])
                if key not in exercise_ids:
                    exercise_ids[key], is_new = resolve_exercise(cur, user_id, e["exercise"])
                    if is_new:
                        created.append(e["exercise"])
                if name_key(name) in MERGE_TARGETS:
                    aliases[name_key(name)] = (name, exercise_ids[key])
                cur.execute(
                    "INSERT INTO workout_exercises (workout_id, exercise_id, position, name_as_written)"
                    " VALUES (%s, %s, %s, %s) RETURNING id",
                    (workout_id, exercise_ids[key], position, name),
                )
                entry_id = cur.fetchone()[0]
                for set_position, s in enumerate(e["sets"], start=1):
                    cur.execute(
                        "INSERT INTO exercise_sets (workout_exercise_id, position, weight_kg, reps)"
                        " VALUES (%s, %s, %s, %s)",
                        (entry_id, set_position, s["weight_kg"], s["reps"]),
                    )
        for alias, exercise_id in aliases.values():
            cur.execute(
                "INSERT INTO exercise_aliases (user_id, exercise_id, alias) VALUES (%s, %s, %s)"
                " ON CONFLICT (user_id, alias_key) DO NOTHING",
                (user_id, exercise_id, alias),
            )
    return created


def find_or_create_user(cur, telegram_user_id):
    cur.execute(
        "SELECT user_id FROM user_identities WHERE provider = %s AND external_id = %s",
        (IDENTITY_PROVIDER, str(telegram_user_id)),
    )
    row = cur.fetchone()
    if row:
        return row[0]
    cur.execute("INSERT INTO users DEFAULT VALUES RETURNING id")
    user_id = cur.fetchone()[0]
    cur.execute(
        "INSERT INTO user_identities (user_id, provider, external_id) VALUES (%s, %s, %s)",
        (user_id, IDENTITY_PROVIDER, str(telegram_user_id)),
    )
    return user_id


def resolve_exercise(cur, user_id, name):
    """Resolution order from data_model.md: alias, then existing exercise, then create.
    Returns (exercise_id, created)."""
    cur.execute(
        f"SELECT exercise_id FROM exercise_aliases WHERE user_id = %s AND alias_key = {NAME_KEY_SQL}",
        (user_id, name),
    )
    row = cur.fetchone()
    if row:
        return row[0], False
    cur.execute(
        f"SELECT id FROM exercises WHERE user_id = %s AND name_key = {NAME_KEY_SQL}",
        (user_id, name),
    )
    row = cur.fetchone()
    if row:
        return row[0], False
    cur.execute(
        "INSERT INTO exercises (user_id, name) VALUES (%s, %s) RETURNING id", (user_id, name)
    )
    return cur.fetchone()[0], True
