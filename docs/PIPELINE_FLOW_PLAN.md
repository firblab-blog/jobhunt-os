# Pipeline Flow Notes

This document records the pipeline visualization behavior as of `v0.1.4` and
the remaining work for historical flow analytics.

## Current Dashboard Behavior

The dashboard starts with a Pipeline Pulse section:

- Sankey graph summarizing current applications across pipeline stages.
- Signal strip for active applications, due follow-ups, interview loops,
  documents, stale opportunities, and applications with no next action.
- Link to the Applications page.

The dashboard graph uses current application state. It does not require
historical transition records.

## Current Applications Behavior

The Applications page includes:

- Next actions section at the top of the page.
- Sankey graph above the applications list.
- Search and status filtering.
- Applications table with status, priority, next action, due date, and update
  date.

The legacy `/follow-ups` route redirects to `/applications#next-actions`.

## Status Grouping

The dashboard and Applications graphs group current statuses into display
buckets:

- Prospect: `prospect`
- Applied: `applied`
- Interviewing: `interviewing`
- Offer: `offer`
- Accepted: `accepted`
- Closed: `declined`, `rejected`, `withdrawn`, `archived`

Closed outcomes remain available as individual filters in the Applications
page. The aggregate closed node links to `?status=closed`.

## Data Sources

The current graphs use:

- application status counts
- next-action due dates
- document count
- active and stale application counts
- application update and timeline event counts

Status changes also create readable timeline events. These events are useful for
the user-facing activity feed, but they are not structured transition records.

## Future Transition History

A historical Sankey needs structured transition history. Future transition rows
could include:

- application ID
- previous status, nullable for the initial state
- new status
- transitioned-at timestamp
- optional source or reason

The transition table should be append-only. The application record should keep
the latest status for forms, filters, and current-state graphs.

## Migration Notes

If transition history is added later:

- Add a transition table with indexes for application ID and transitioned-at.
- Keep the existing application status column.
- Keep generated note events for the timeline.
- Optionally backfill one synthetic transition for older records.
- Do not parse generated note text for normal analytics.
- Label pre-transition history as incomplete where needed.

## Non-Goals

- Do not add a charting dependency unless the current SVG approach becomes hard
  to maintain.
- Do not rely on generated note text as the source of truth for analytics.
- Do not move next-action queue behavior back to a separate Follow-ups page.
