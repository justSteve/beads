---
name: tap-in
description: Initialize session with context briefing
context: fork
allowed-tools: Bash, Read, Glob, Write
---

# Tap In - Session Initialization

Read recent activity and current state to get oriented at session start.

## Workflow

### 1. Get Current Date

```bash
date +%Y-%m-%d
```

---

### 2. Check if Daily Housekeeping Needed

```bash
head -1 /root/projects/beads/DaysActivity.md 2>/dev/null
```

- If date doesn't match today: Run `/daily-housekeeping` first
- STOP here until housekeeping completes
- Then resume tap-in

---

### 3. Read Recent Activity

**Last 2-3 entries from DaysActivity.md**:
- Note open work items
- Note recent state/issues
- Identify continuity threads

```bash
head -80 /root/projects/beads/DaysActivity.md
```

---

### 4. Read CurrentStatus.md

```bash
cat /root/projects/beads/CurrentStatus.md
```

Get operational context.

---

### 5. Check Open Beads

```bash
bd list --status open
```

---

### 6. Check CM Search Substrate

Query CM health to distinguish three failure modes that all look the same to a caller:
(a) server down, (b) server up but empty index, (c) server up but stale index.

```bash
# Port comes from factory.env CM_PORT; Zgent = 3002.
CM_PORT=$(grep -E '^CM_PORT=' /root/projects/beads/factory/factory.env 2>/dev/null | cut -d'"' -f2)
CM_PORT=${CM_PORT:-3002}
curl -s --max-time 3 "http://localhost:${CM_PORT}/api/v1/health" 2>/dev/null
```

Then classify:

- **No response / non-2xx** → CM server down. Flag: "memory search unavailable — start with `systemctl status claude-monitor`."
- **`conversationCount` missing from response** → server is pre-patch (old schema). Flag: "CM health endpoint predates co-s5e patch; apply `factory/patches/claude-monitor/001-health-endpoint-db-stats.patch` for richer signals."
- **`conversationCount == 0`** → DB is empty. Flag: "CM index is empty — run `cd /root/projects/claude-monitor && bun run backfill` to seed."
- **`lastEntryTimestamp` > 24h old** → backfill stale. Flag: "CM index hasn't absorbed new content in >24h. Check `/tmp/cm-backfill.log` and the crontab."
- **Otherwise** → CM is healthy; no warning needed.

Include any warning in the **Current State** section of the briefing.

---

### 7. Output Session Briefing

**Write to**: `/root/projects/beads/session-briefing.md`

```markdown
## Session Briefing - YYYY-MM-DD HH:MM

---

### Recent Activity

**Last Session**: [timestamp] - [brief summary from most recent handoff]

**Open Work (carried forward)**:
- [item 1]
- [item 2]

---

### Current State

[Summary from CurrentStatus.md — version, rigs, attention items]

---

### Open Beads (active/recent)

| Bead | Title | Type |
|------|-------|------|
| id | title | type |

---

### Resumption Guidance

**Carried forward from last session**:
1. [specific next step]
2. [specific next step]

---

### Ready Status

[Ready to proceed | Issues require attention]
```

---

## Pairs With

- `/handoff` - Session end
- `/daily-housekeeping` - Runs before tap-in if date changed

## Re-run Anytime

This skill can be invoked mid-session to refresh context:
```
/tap-in
```

## Removed, deliberately

**Agent-teams detection and the Team State Analysis step.** The first step used
to run `claude config get experimental.agentTeams`. That call **hangs** on
Claude Code 2.1.216 — it never returns, even with stdin closed (verified: 20s
timeout, rc=124) — so it burned the Bash timeout on every session start. Its
only consumer was a Team State Analysis step that ran `jq` against
`.beads/issues.jsonl`, a file that does not exist because the beads store is
Dolt-backed. Nothing ever set the flag the team-aware output was gated on, so
that output was unreachable too. A hang feeding a no-op. Removed 2026-07-21
(co-kavq) (this repo has no `/handoff` skill; if one is added
later, do not copy the call back in from a sibling repo).
