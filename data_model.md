# Workout data model

This document explains how workout data is represented in this project: what each entity means, how the text log format maps onto it, and which rules hold across the system. It describes the model; it does not prescribe any implementation work.

## System shape

Postgres is the only store. A Go API owns all reads and writes. Frontends (currently a Telegram bot, later a mobile app) talk only to the API and never to the database. The one exception is a pair of one-time Python scripts in `scripts/` that imported historical workouts directly into the database, one from a Markdown log and one from a Telegram channel export; they are not part of the running system. See "Historical import" below for where the imported data departs from the rules in this document.

Nothing in the core model is Telegram-specific. Telegram appears only as an identity provider and as a source of workouts.

## Users and identities

A user is an internal entity with a serial integer ID. That ID is the only user identifier used inside the system, in every foreign key and every API call.

External accounts are bound to a user through identities. An identity is a pair of provider name and external ID, unique across the system, pointing to exactly one user. A user can have several identities. The Telegram bot resolves an incoming Telegram user to an internal user through the identity with provider `telegram` and the Telegram user ID as external ID. A Telegram ID is never used as a user ID.

## Workout

A workout is one gym session. It belongs to one user and holds:

| Field | Meaning |
|---|---|
| Date performed | Calendar date only, no time of day. |
| Workout type | Optional. One of a closed set: `lower`, `upper`, `push`, `pull`, `full body`, `push+pull`, `push+lower`, and the names of the 2024-2025 programs `workout A`, `workout B`, `день 1`, `день 2`, `руки`, `рест`. Stored as text; the allowed set is application configuration, not a database constraint, and is expected to grow. Matching is case-insensitive. The historical import also stored `pull+push` as `push+pull` and `legs` as `lower`; that was cleanup of one user's data, and the bot parser has no such mappings. |
| Calories, protein | Optional integers describing intake before the workout (kcal and grams). Most historical workouts have neither; the user started recording them recently. |
| Note | Optional free text. Anything in the header that is not the date, a recognized workout type, or the intake block. |
| Raw text | The exact text the workout was parsed from. Always present. |
| Source, source ref | Where the workout came from and an identifier within that source (see below). |

Two workouts on the same date are allowed.

The raw text exists so that everything else about a workout can be re-derived. Treat the structured fields as a parse of the raw text, not as an independent record. If the parser changes, workouts can be rebuilt from raw text. For imported workouts the parse includes the import scripts' own rules, so rebuilding them needs those scripts and not the bot parser.

## Exercise entries and sets

A workout contains an ordered list of exercise entries. Each entry corresponds to one line of the log and holds a position (1-based, contiguous, order as written), a reference to an exercise, and the exercise name exactly as written on that line.

The same exercise may appear more than once in one workout as separate entries at different positions. Entries are not merged.

Each entry contains an ordered list of sets. A set has a position (1-based, order as written), a weight in kilograms, and a rep count. The bot never saves an entry without sets, but 37 imported entries have none: the log named the exercise and recorded no numbers.

- Weight is a decimal with up to two fractional digits. It is never null; zero means no external weight was recorded.
- Reps are a positive integer. A set with zero reps does not exist.
- For bodyweight exercises such as pull-ups, the user records their own bodyweight as the weight. The model does not distinguish bodyweight from external load. Any analysis that compares bodyweight exercises over time is mixing bodyweight drift with strength progress, and the data cannot separate the two.

Sets carry no other attributes. There is no notion of warm-up sets, drop sets, supersets, per-side work, RPE, rest time, set timestamps, or units other than kilograms.

## Exercises, names, and aliases

An exercise is just a name owned by a user. There is no global exercise catalog, no muscle groups, no categories, and no relation between users' exercises. Each user's exercise list is exactly the set of names they have ever logged, after merges.

Names are compared through a normalized key: trim, collapse internal whitespace runs to a single space, lowercase. The database computes this key and enforces uniqueness of (user, key). Two names that normalize to the same key are the same exercise. The stored display name of an exercise is the spelling under which it was first created.

When a workout is saved, each exercise name is resolved in this order:

1. If the normalized name matches one of the user's aliases, the entry points to the alias's target exercise.
2. Otherwise, if it matches an existing exercise's key, the entry points to that exercise.
3. Otherwise, a new exercise is created with that name. The user is told about newly created names after the save so typos can be fixed immediately.

Fixing a typo is a replace operation from a bad name to a correct name. It does two things: it repoints every entry that referenced the bad exercise to the correct one (if the correct one doesn't exist, this is effectively a rename) and removes the bad exercise, and it records the bad name as an alias of the correct exercise. The alias is what makes the fix permanent. Without it, re-saving an old message that still contains the typo would recreate the bad exercise.

Because every entry keeps the name as written, a wrong merge can be undone by finding entries whose written name differs from their exercise's name.

## Sources and idempotency

Every workout records a source and a source ref, unique together per user.

| Source | Source ref | Origin |
|---|---|---|
| `telegram_channel` | `<message id>` | Historical import from the user's Telegram channel export (`messages.html`). The export carries no channel id. |
| `logs_expanded` | ISO date, with `#2`, `#3` appended when a date repeats | Historical import of the 2024-2025 Markdown log (`logs_expanded.md`). |
| `telegram_bot` | `<chat id>:<message id>` | Workouts logged through the Telegram bot. |

Saving a workout from the bot is an upsert on this key. If the user edits a message they already saved and saves again, the existing workout is replaced, including all its entries and sets. This is the only editing mechanism. There is no separate edit flow and no delete flow in the MVP.

Future frontends need their own source name and a stable client-side identifier per workout to get the same behavior.

## Text log format

Users write workouts in a compact text format. The bot parses it into the structured form above before sending it to the API; the API receives structured data plus the raw text.

```
03.10 upper (680/37)
жим в нахилі сміт 60-10 -9 50-11 -10
підтягування 82-9 -9
скручування -10 -20 -30
```

The first non-empty line is the header. It starts with the date as `DD.MM` or `DD.MM.YYYY`. Without a year, the year of the post date is used, with no other adjustment: a `31.12` workout posted on 2 January gets the new year unless written as `31.12.YYYY`. Right after the date may come a workout type from the closed set, matched case-insensitively as a prefix of the remaining header text. An intake block `(kcal/protein)` may appear anywhere in the header. Any other text becomes the note.

Every following non-empty line is an exercise line. The exercise name is everything before the first set token, so names can contain spaces and any script. After the name come whitespace-separated set tokens of two forms:

- `W-R` sets the current weight to W and records a set of R reps at W.
- `-R` records a set of R reps at the current weight.

The current weight starts at zero on every line and carries forward until another `W-R` token changes it. Weights may use a dot as decimal separator.

The example above produces:

| Entry | Sets (weight × reps) |
|---|---|
| жим в нахилі сміт | 60×10, 60×9, 50×11, 50×10 |
| підтягування | 82×9, 82×9 |
| скручування | 0×10, 0×20, 0×30 |

Parsing is all-or-nothing per message. A line that has no set tokens, has no name, or contains a token that is neither form makes the whole message invalid; nothing is saved and the error identifies the line and token. There are no note lines inside the body.

## Historical import

`scripts/import_logs.py` and `scripts/import_telegram.py` loaded 329 workouts (2024-09-28 to 2026-10-03) for the user with Telegram id 685751256. They share `scripts/import_common.py`. The imported data departs from the rules above in these ways.

Parsing is looser than the bot grammar:

- A body line with only an exercise name becomes an entry with no sets.
- A prose line at the end of a workout, and text after the name or the sets on an exercise line (parentheticals, `— замінив на гантелі`, `53 сек`), go to the workout note. Text taken from an exercise line is prefixed with the exercise name.
- Messages in the channel whose first line is not a date are skipped. Five were, all training programs or notes.

Some weights are corrected by rule, so they differ from the raw text:

| Rule | Applies to |
|---|---|
| Bodyweight of 85 kg where the log wrote 0 or no weight | `підтягування`, `відтискання`, `вузьке відтискання`, `вис на турніку` |
| Weight 0 where the log wrote bare reps | every other exercise, since crunches count added weight only |
| Seconds stored as reps | `вис на турніку` |
| Weight doubled, because the log counted one dumbbell | workouts of type `руки` in `logs_expanded` |
| Weight halved, because the log counted both sides of the rope | `тяга на задню дельту`, `тяга канату на задню дельту`, `розведення канату на задню дельту` in `telegram_channel` |

Some source lines were edited before import, so the raw text is not what was originally written: typos in exercise names, sets with swapped weight and reps, and missing weights or reps filled in from neighbouring sessions. In `logs_expanded.md` that covers 16 lines of March and April 2025.

Exercise names were merged at import. Each entry keeps its name as written and points to the merged exercise, and every merged spelling was saved as an alias (88 aliases, 73 exercises). The map is `EXERCISE_MERGES` in `scripts/import_common.py`.

Calf raises written without a position (`підйом на ікри`, `підйоми на ікри`, `ікри`) point to `підйом на ікри стоячи` from 2025-04-16 to 2025-08-31 and to `підйом на ікри сидячи` outside that period. The target depends on the date, so these three spellings have no alias. A new workout that uses one of them will create a new exercise under that name.

## Where rules are enforced

| Rule | Enforced by |
|---|---|
| Unique identity per provider and external ID | Database |
| Unique exercise per user and normalized name | Database (computed key) |
| Unique alias per user and normalized alias | Database (computed key) |
| Unique workout per user, source, and source ref | Database |
| Contiguous ordered positions for entries and sets | Database (unique per parent) plus writer |
| Non-negative weight, positive reps, non-negative intake | Database |
| Workout type in the allowed set | Application |
| Alias resolution before exercise creation | Application |
| An alias key must not equal a live exercise key | Application (replace removes the bad exercise) |
| Entries and exercises belong to the same user | Application |
| Text format grammar | Bot (parser) |

The normalized key relies on the database lowercasing non-ASCII text, which requires a UTF-8 or ICU locale on the database. Any code that normalizes names outside the database must produce the same key: trim, collapse whitespace, lowercase.
