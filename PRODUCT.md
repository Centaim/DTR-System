# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

React and TypeScript, mobile-first. Tauri 2 wraps the same interface for Android and iOS, so the design language stays web. Go Fiber API, PostgreSQL, S3-compatible photo storage. Confirmed in PLAN.md.

## Users

Primary: office staff of a Philippine government agency, clocking in at or near the office, often in a hurry at shift start, on a mid-range Android phone with patchy 4G. Secondary, confirmed in PLAN.md: supervisors who approve requests, HR admins who audit photos and print DTRs, and timekeepers. Field and work-from-home staff are supported later.

## Product Purpose

A Daily Time Record system for government agencies, bid through PhilGEPS. Employees clock in and out with a live photo as proof of attendance. The system produces the CSC Form No. 48 DTR. Success is a clock-in in three taps or fewer that confirms within five seconds, and a DTR an agency can print and certify.

## Positioning

Photo as proof of attendance without automatic face recognition, mobile-first, with offline sync and Form 48 output built for Philippine civil service rules. The agency is unknown at bid time, so rules and branding are configurable.

## Operating Context

Employees certify their DTR as true and correct, supervisors verify it against prescribed office hours, and the accomplished DTR is submitted monthly. Offices may have unreliable connectivity, so logs queue on the device and sync later. Interface text is English and Filipino.

## Capabilities and Constraints

Live camera capture only, no gallery uploads. Server time is the official time. Raw logs are never edited. Anomaly flags go to HR and never penalize anyone automatically. Photos are restricted and every view is logged. Agency-specific attendance rules are settings. Unverified: current CSC attendance rules, NPC requirements for face photos, and the current official Form 48 layout (see PLAN.md section 6).

## Brand Commitments

None. The product name is undecided. The demo stays agency-neutral with a placeholder name ("DTR"). No agency seal or logo, and the accent is swappable per agency.

## Evidence on Hand

PLAN.md and CLAUDE.md only. No real agency data, logos, photos or testimonials exist. Do not fabricate them.

## Product Principles

1. Clock-in is the one job. It is always the most visible thing on the home screen.
2. Always show what was recorded and whether it has synced, so no employee wonders if their log counted.
3. Plain, official, trustworthy. Government users are not designers.
4. Every number and rule can be traced, because this record may be audited.
5. Works on bad networks and cheap phones.

## Accessibility & Inclusion

WCAG 2.1 AA. Touch targets of at least 48 px. English and Filipino. Readable outdoors in bright light, and respects system text scaling.
