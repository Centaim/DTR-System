# Centaim DTR System

Daily Time Record system for Philippine government agencies, being bid through PhilGEPS. Employees clock in and out with a live photo as proof of attendance, on a mobile app or an office kiosk. The full scope, modules and schedule are in `PLAN.md`. Read it before starting any feature.

## Stack

- Frontend: React and TypeScript, mobile-first
- Mobile: Tauri 2 (Android and iOS) sharing the React interface
- Backend: Go with Fiber
- Database: PostgreSQL
- Photos: S3-compatible object storage (MinIO)
- Deployment: Docker Compose

Planned layout: `apps/web`, `apps/mobile`, `packages/{ui,api-client,shared}`, `server/{cmd,internal,migrations}`, `deploy/`. Update this section when the real layout differs.

## Product rules

- **Photo as proof only.** Do not add face recognition, face templates or automatic face matching unless the user explicitly changes the scope.
- **The agency is unknown.** Attendance rules, schedules, thresholds, approval levels and retention periods are settings, never hard-coded.
- **Raw logs are append-only.** Never update or delete an attendance log. Corrections are new records that point to the original.
- **Daily records are computed.** They must always be regenerable from raw logs, schedules and approved requests.
- **Server time is the official time.** Offline logs are flagged, and device clock changes are detected.
- **Flags never penalize.** Anomaly flags go to HR for review and never change an employee's record automatically.
- **Past DTRs keep the rules in force at that time.** Use effective-dated rule versions.
- **Photo access is restricted and logged.** Every photo view writes an audit entry.

## Code conventions

- Use Go idioms. One package per business module under `server/internal`. Handlers stay thin, and business logic lives in services.
- Write database changes as numbered migrations. Never edit an applied migration.
- Make log submission idempotent with a client-generated key, so retries cannot create duplicates.
- Check permissions on the server for every endpoint. Never rely on the client to hide actions.
- Use TypeScript strict mode. Share API types through `packages/shared` and avoid `any`.
- Keep user-facing text in translation files for English and Filipino. Do not hard-code strings.
- Store all timestamps in UTC and display them in Asia/Manila.
- Never log photos, tokens, PINs or personal data.

## UI and design

- Use the Impeccable skill for every UI task: shape new screens, critique and audit finished ones, then polish before release.
- Design at 360 px wide first, then add tablet and desktop layouts.
- Touch targets are at least 48 px. Tables become cards on phones.
- Meet WCAG 2.1 AA, and show a clear loading, empty and error state on every screen.
- Once the first screens ship, record the design system in `DESIGN.md` and follow it.

## Mobile notes

- Android WebView needs native camera permission handling, so clock-in uses a custom Kotlin and Swift camera plugin. Test it on a low-end Android phone before building features on top of it.
- Use official Tauri plugins for geolocation, barcode scanning, biometric unlock, notifications and SQLite where they fit.
- Store tokens in Android Keystore and iOS Keychain.
- iOS builds need a Mac with Xcode.

## Privacy and security

- Follow RA 10173 (Data Privacy Act). Collect only what attendance needs. Capture location only at clock-in and clock-out.
- Encrypt photos at rest and use short-lived signed URLs to serve them.
- Do not commit secrets, real employee data or real photos. Use fixtures with fake data.

## Working agreements

- Confirm unclear agency-specific behavior against `PLAN.md` section 6 before building it. If a rule is unverified there, make it a setting and note it.
- Keep priced add-ons (plan items 21 to 35) out of the base build unless asked.
- Before calling work done, run `cd server && go vet ./... && go test ./...` and `cd apps/web && pnpm typecheck`. See `README.md` for how to run the stack locally. No lint step exists yet.
- Do not commit unless asked.
