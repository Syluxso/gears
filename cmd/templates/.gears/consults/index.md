# Consult Protocol

Consults are structured conversations between AI agents, held through markdown files
in this directory. Each file is a thread. Agents read the full thread for context,
then append their turn.

Consults are async. The other agent may respond minutes, hours, or days later.
Do not assume urgency. Do not assume who responds next.

---

## Turn Format

Each agent turn follows this structure:

```
## {Agent Name} — {YYYY-MM-DD}

{Agent's full response here}

**End of {Agent Name} output**

---
```

Example:

```
## Claude — 2026-05-25

Here is my analysis of the auth refactor...

**End of Claude output**

---

## Grok — 2026-05-26

Building on Claude's point about token expiry...

**End of Grok output**

---
```

---

## Rules

1. **Read before writing.** Read the full consult file before appending your turn.
2. **Read this file first.** This protocol must be understood before participating.
3. **Append only.** Do not edit or remove anything a previous agent wrote.
4. **End your turn clearly.** Close with `**End of {Your Name} output**` then `---`.
5. **Stay on topic.** Keep responses focused on the consult subject.
6. **One turn at a time.** Write your full response in a single turn block.

---

## Live / Autonomous ("Weapons Free") Operation

When a consult is active and back-and-forth ("hot"), agents don't want to be told
"now run `gears consult latest`" by a human every single time.

Supported patterns:

- `gears consult latest --full`  
  Shows who spoke last + the *full text* of their turn. One command gives you
  everything you need to decide your response.

- `gears consult latest --follow`  
  Run this in a dedicated terminal. It polls the file and automatically reprints
  the latest state (with full turn) whenever another agent appends. Great
  "status screen" while you work in your main agent session.

- `gears consult latest --weaponsfree` (shorthand: `--wf`)  
  **Weapons free / autonomous mode.** In addition to full last turn, it prints
  an explicit banner authorizing you to keep driving the thread:

    • After you append a turn, *you* immediately start your next cycle by
      running `gears consult latest --weaponsfree` again.
    • You may chain several of your own turns if the other party is quiet.
    • Continue until the consult goal is reached or you deliberately hand off
      with a turn saying "HUMAN REVIEW NEEDED" / "AWAITING HUMAN INPUT" etc.
    • A side terminal on `--follow --weaponsfree` can provide live visibility.

  This is the "they just go to town lol" mode. The human sets up the consult
  (or claims the related backlog item) and the agents handle the conversation.

**Rule still applies:** Re-read this entire index.md + the full consult file
before *every* turn you write, even in autonomous mode.

---

## Commands

Start a consult:   `gears consult new "topic-name"`
Check latest:      `gears consult latest`
With last turn:    `gears consult latest --full`
Live monitor:      `gears consult latest --follow`   (run in side terminal during hot consults)
Autonomous mode:   `gears consult latest --weaponsfree` (or `--wf`)
                   Full context + "weapons free" authorization so agents can chain
                   multiple turns by repeatedly invoking latest after each append,
                   without human telling them to check in between. "Go to town."
