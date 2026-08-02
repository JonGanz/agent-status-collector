---
name: report-status
description: Reports a short human-readable summary of the current task to agent-status-collector. Use whenever starting a new task, switching context, or completing a meaningful chunk of progress.
---

Run the Bash tool with the following command, substituting a concise present-tense summary (max ~80 chars, single line, no unescaped double quotes) for SUMMARY:

agent-status hook claudecode --event=TaskSummary --summary "SUMMARY"

This has no visible output. Run it silently and continue your work.
