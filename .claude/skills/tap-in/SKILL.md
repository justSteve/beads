---
name: tap-in
description: Initialize session with context briefing (team-aware)
context: fork
allowed-tools: Bash, Read, Glob, Write
---

# Tap In - Session Initialization

Read recent activity and current state to get oriented at session start. **Team-aware**: Detects experimental agent teams and provides team coordination guidance.

## Workflow

### 1. Check Session Capabilities

```bash
# Check if experimental agent teams are enabled
claude config get experimental.agentTeams 2>/dev/null || echo "teams_disabled"
```

**Set flag**: `TEAMS_ENABLED=true/false` for subsequent steps

---

### 2. Get Current Date

```bash
date +%Y-%m-%d
```

---

### 3. Check if Daily Housekeeping Needed

```bash
head -1 /root/projects/beads/DaysActivity.md 2>/dev/null
```

- If date doesn't match today: Run `/daily-housekeeping` first
- STOP here until housekeeping completes
- Then resume tap-in

---

### 4. Read Recent Activity

**Last 2-3 entries from DaysActivity.md**:
- Note open work items
- Note recent state/issues
- Identify continuity threads
- **TEAMS**: Look for team mentions (Team Alpha, Team Beta, etc.)

```bash
head -80 /root/projects/beads/DaysActivity.md
```

---

### 5. Read CurrentStatus.md

```bash
cat /root/projects/beads/CurrentStatus.md
```

Get operational context.

---

### 6. Check Open Beads

```bash
tail -30 /root/projects/beads/.beads/issues.jsonl | jq -r 'select(.status == "open") | [.id, .type, .title] | @tsv' | column -t
```

---

### 6.5. Check CM Search Substrate

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

### 7. Team State Analysis (if TEAMS_ENABLED)

**7a. Scan beads for team assignments**:

```bash
# Get all open beads with team metadata
jq -r 'select(.status == "open") | [.id, .title, .metadata.team // "none", .metadata.priority // "none"] | @tsv' \
  /root/projects/beads/.beads/issues.jsonl | column -t
```

**7b. Group beads by team**:

```bash
# Count beads per team
jq -r 'select(.status == "open" and .metadata.team) | .metadata.team' \
  /root/projects/beads/.beads/issues.jsonl | sort | uniq -c
```

**7c. Identify ready-to-launch teams**:

Teams with open beads assigned and no blocking dependencies.

---

### 8. Output Session Briefing

**Write to**: `/root/projects/beads/session-briefing.md`

```markdown
## Session Briefing - YYYY-MM-DD HH:MM

**Session Mode**: [Standard | Team Execution]
**Teams Enabled**: [Yes | No]

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

### Team Status (if teams enabled)

**Active Teams**: [count]

| Team | Open Beads | Status | Blockers |
|------|------------|--------|----------|
| Alpha | 3 | In Progress | None |
| Beta | 2 | Blocked | Waiting on Alpha.3 |

**Ready to Launch**: [teams with no blockers]

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

**If continuing team work**:
1. Check Team [X] progress
2. Launch Team [Y]: Ready, no blockers

---

### Ready Status

[Ready to proceed | Issues require attention | Team coordination needed]
```

---

## Pairs With

- `/handoff` - Session end (records team state)
- `/daily-housekeeping` - Runs before tap-in if date changed

## Re-run Anytime

This skill can be invoked mid-session to refresh context:
```
/tap-in
```
