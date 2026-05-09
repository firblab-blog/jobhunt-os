# Pipeline Flow Plan

This note captures the planned direction for adding pipeline visualization to JobHunt OS without making the dashboard feel busier.

## Goal

Use graph-like pipeline views to make the application search feel more alive and easier to understand at a glance.

The dashboard should show a compact, glanceable pipeline pulse. The applications page should eventually support a larger diagnostic flow view for deeper analysis.

## Dashboard: Pipeline Pulse

Replace the current top stack on the dashboard:

- Today prompt
- Summary metric cards
- Pipeline count strip

with a single composed top section:

- A compact pipeline flow visualization.
- A "today's move" area with the next useful action and primary call to action.
- Small signal chips for active applications, due follow-ups, interview loops, documents, and stale opportunities.

The dashboard graph should be visually distinctive, but not a full Sankey. It should separate the home page from the rest of the app while keeping the dashboard calm and action-oriented.

### Dashboard Shape

The compact flow should summarize the mature job-search pipeline:

```text
Prospect -> Applied -> Interviewing -> Offer -> Accepted
                     \              \          \
                      No reply       Rejected   Closed
```

The first implementation can map the current application statuses into these display groups:

- Prospect: `prospect`
- Applied: `applied`
- Interviewing: `interviewing`
- Offer: `offer`
- Accepted: `accepted`
- Closed: `declined`, `rejected`, `withdrawn`, `archived`

The design should optimize for a realistic mature search with dozens of applications, not for the current small local sample. Empty and low-data states should be quiet renderings of the same structure, not a separate design target.

### Dashboard Behavior

- Keep the graph compact enough to fit in the top half of the dashboard.
- Prefer lightweight HTML/CSS/SVG over a charting dependency.
- Preserve the dashboard's core job: answer "what should I do next?"
- Use the graph to answer "how healthy is my pipeline right now?"
- Link to the applications page for the fuller flow view.

## Applications Page: Full Flow View

Add a larger flow view on the applications page after the dashboard work is settled.

The applications page should remain table-first, but it can expose a richer graph through a segmented control or view switch:

```text
[Table] [Flow]
```

The full view can eventually show a Sankey-style diagnostic map:

- Applied
- Replies
- No reply
- Recruiter screen
- Technical screen
- Take-home
- Final interview
- Offer
- Accepted
- Rejected before offer
- Rejected after offer
- Withdrawn

This page is the right place for a larger, more analytical visualization because users can spend time diagnosing where the search is leaking without overwhelming the home page.

## Data Model Notes

Phase one can use current application state:

- Status counts from applications.
- Existing follow-up, priority, stale, and this-week activity signals.

A true historical Sankey needs structured transition history. Today, status changes append a generated note event. That is useful for the activity feed and human-readable audit trail, but it is not ideal for analytics because previous and next status are embedded in text rather than stored as fields. Parsing that text later would be brittle and would not reliably answer questions like "how many applications moved from recruiter screen to technical screen last month?"

Future structured transition data could include:

- Application ID: the application whose status changed.
- Previous status: nullable only for initial creation/import events.
- New status: the canonical application status after the transition.
- Transitioned-at timestamp: when the status change took effect, not merely when it was recorded.
- Optional reason or event source: user action, import, automation, company response, cleanup, or another short structured source.

This should be modeled as append-only transition rows rather than replacing the current status field. The application record can continue to store the latest status for fast reads, forms, filters, and dashboard counts. Transition history would be the analytical layer used to reconstruct paths over time.

### Generated Note Event vs. Structured Transition

The generated note event is presentation-oriented:

- It preserves a readable timeline entry for the user.
- It can include friendly wording and context.
- It is appropriate for the activity feed.
- It should remain backward compatible and should not become the source of truth for analytics.

The structured transition row is analysis-oriented:

- It stores previous status, new status, and timestamp as queryable fields.
- It can power aggregate flow counts without text parsing.
- It can distinguish status movement from ordinary notes.
- It can later support richer Sankey paths across intermediate stages.

Both can coexist. A future status change can write the structured transition row and still emit a generated note event for the feed.

### Dashboard Without Transition History

The dashboard compact flow does not need historical transitions. It can group the current application statuses into the compact display buckets and render a present-tense pipeline snapshot:

- Current volume by stage.
- Active, stale, and follow-up signals.
- A calm read on whether the search has enough activity near the top, middle, and end of the pipeline.

This makes the dashboard useful before any migration work. It answers "what does my pipeline look like right now?" rather than "how did applications move between stages over time?"

The full Applications Sankey would benefit from structured transitions later because it can answer path and conversion questions:

- Which stages are applications entering from?
- Where are applications dropping out?
- How long do applications spend before moving to the next stage?
- Which paths lead to offers or accepted roles?

### Migration and Compatibility Considerations

Add transition tracking only when the larger Applications flow view needs historical paths. The migration should be additive:

- Create a new transition table with indexes for application ID and transitioned-at timestamp.
- Keep the existing application status column as the current-status source for normal screens.
- Keep existing generated note events unchanged.
- For older records, optionally backfill a single synthetic transition from an unknown previous status to the current status, marked with an import/backfill source.
- Do not infer detailed historical paths from generated note text unless a one-off migration can do so confidently and safely.
- Make Sankey analytics tolerate missing history by showing current-state snapshots or labeling pre-tracking history as incomplete.

## Suggested Build Order

1. Add dashboard-specific pipeline pulse data structs.
2. Compute compact display groups from existing application statuses.
3. Replace the dashboard top stack with a composed pipeline pulse section.
4. Move the existing summary metrics into the pulse section as compact signal chips.
5. Keep lower dashboard cards focused on operational detail.
6. Add focused tests for dashboard pulse grouping and empty states.
7. Add the larger applications flow view as a second phase.
8. Add structured status transition tracking only when the full flow view needs true historical paths.

## Non-Goals For The First Pass

- Do not add a large Sankey to the dashboard.
- Do not add a charting dependency for the compact dashboard visualization unless the CSS/SVG approach becomes brittle.
- Do not redesign the entire dashboard below the fold.
- Do not block the dashboard improvement on historical transition tracking.
