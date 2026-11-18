---
version: 1
slug: "apps-web-src-app-tsx"
primary_target: "apps/web/src/App.tsx"
related_targets: []
---

# Surface brief: clock-in home (apps/web/src/App.tsx)

Mode: Operate. Phone first (360px), code-led. Primary task: clock in or out in three taps or fewer and see it recorded and synced.

Audience: office staff in a hurry at shift start, daylight, mid-range Android, patchy 4G.

Unresolved: product name (placeholder "DTR"), real agency branding, Filipino copy review.

## Direction contract

THESIS: Clock-in is the one job, so the screen is one working panel stack where the punch button owns the page and every log shows its sync state as a line form, not decoration. Refuses the dashboard of same-size stat cards.

OWN-WORLD: Dark-first graphite console, translated for daylight. Default theme is light: cool paper ground, graphite ink, white panels split by 1px hairline seams and one elevation step, no shadows for depth. Dark graphite theme follows the system setting. One blue accent for the primary action only. Green, amber, red appear only as state (synced, pending, failed). Compact system sans with tabular figures, small-caps panel headers, destructive actions isolated. Translation named: the source is graphite-first, but the use scene is daytime sun and office doors, so light leads and graphite is the dark theme.

STORY: The employee sees who they are and the time, taps one large button, takes a live photo, and sees a confirmation with the recorded time and sync state. They trust the record because every punch shows when it was saved and whether it reached the server.

FIRST VIEWPORT: At 360x640. Top bar with agency placeholder and online/offline status. Large live clock with date. Below, one full-width primary button "Clock in" (or "Clock out"), at least 56px tall, in thumb reach. Below that, the next expected log line and today's four slots (AM in, AM out, PM in, PM out) on one vertical seam rail. Bottom tab bar: Home, Attendance, Requests, Alerts, More.

FORM: Dark-first developer console from the assigned roll, candidate 5 of 5, seed key 8027a51e. Signature move: the seam rail, where each punch slot is a marker on one hairline. Solid is synced, hollow is empty, dashed is waiting to sync, struck is rejected.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
