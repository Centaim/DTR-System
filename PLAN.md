# Centaim DTR System: Bid and System Plan

A general-purpose Daily Time Record (DTR) system for Philippine government agencies, prepared for bidding through PhilGEPS. Employees clock in and out with a live face photo as proof of attendance, using an office kiosk or a mobile app.

The target agency is not yet known, so agency-specific rules are settings rather than hard-coded behavior.

## 1. Scope decisions

| Topic | Decision |
|---|---|
| Attendance proof | **Live face photo as proof.** Each clock-in and clock-out stores a photo for human review. |
| Face recognition | **Not included.** The system does not create face templates or automatically match faces. |
| Design approach | **Mobile-first.** Screens, APIs, and delivery start from the phone experience, then scale up to tablet and desktop. |
| Clock-in channels | Mobile app as the primary experience, with an office kiosk for staff without a suitable phone |
| Mobile platforms | Android and iOS, built with Tauri 2 |
| Web | Responsive employee, supervisor, HR and admin portal |
| Tech stack | React TypeScript, Go Fiber, PostgreSQL |
| Deployment | Docker Compose on an agency server or government cloud |

### How photo proof works

1. The employee identifies themselves with an employee ID and PIN, a QR code, or a signed-in mobile app.
2. The camera opens. Only a live capture is accepted, and gallery uploads are not available.
3. The device checks that a face is in frame. This check does not identify the person.
4. The photo, device ID, and location (mobile only) are sent to the server.
5. The server records the official time and stores the raw log.
6. The attendance engine updates the employee's daily record.
7. The employee sees a confirmation with the recorded time and log type.

Because no automatic matching takes place, buddy punching is controlled through PINs, registered devices, exception flags, and HR photo audits.

## 2. System modules

### Core modules

| # | Module | Features |
|---|---|---|
| 1 | **Kiosk clock-in** | ID, PIN or QR entry; live photo capture; face-in-frame check; server timestamp; duplicate-tap blocking; registered devices only |
| 2 | **Mobile app** | Selfie clock-in, geofencing, DTR viewing, requests, approvals and reminders (see section 3) |
| 3 | **Offline mode** | Encrypted local queue, automatic sync, clock-tampering detection, and flags on offline logs for HR review |
| 4 | **Employee records** | Profiles, reference photo, office and division hierarchy, position, supervisor, allowed clock-in channels, and employment status (plantilla, casual, contract of service, job order) |
| 5 | **Schedules** | Fixed, flexitime, compressed workweek, work shifting, skeleton workforce, work from home, overnight shifts, holidays and work suspensions |
| 6 | **Attendance engine** | Pairs logs into AM/PM arrival and departure; computes tardiness, undertime, absences and overtime; applies approved requests; reprocesses when rules change |
| 7 | **Corrections and approvals** | Missed-log corrections, pass slips, official business, travel orders and leave tagging; configurable approval levels; original logs are never overwritten |
| 8 | **DTR generation** | CSC Form No. 48 PDF, single and bulk printing, employee certification, supervisor verification, and period locking |
| 9 | **Photo audit** | Side-by-side view of log photo and reference photo, random sampling, exception queue, and reviewer notes |
| 10 | **Reports** | Tardiness, undertime, absences, habitual tardiness flags, attendance summaries, offline and out-of-area logs; PDF, Excel and CSV export |
| 11 | **Dashboard** | Live present, late, absent and on-leave counts by office, pending approvals, and kiosk status |
| 12 | **Administration** | Role-based access, agency branding, attendance rules, cutoffs, geofences, and device registration |
| 13 | **Security and privacy** | Append-only audit trail, encrypted photos, retention rules, privacy notice, and data subject request handling |
| 14 | **Event attendance** | Flag ceremonies, trainings, and meetings; photo or QR check-in; expected-attendee lists; attendance reports |
| 15 | **Accomplishment reports** | Daily or period reports for work-from-home, contract of service, and job order staff; attachments; supervisor review; link to the DTR period |
| 16 | **Anomaly detection** | Rule-based flags sent to HR for human review (see below) |
| 17 | **Announcements** | Memos and advisories on the kiosk, portal, and app; targeting by office; read acknowledgement tracking |
| 18 | **Notification center** | In-app and email alerts for missed logs, pending approvals, DTR deadlines, and request results |
| 19 | **Onboarding and offboarding** | Excel import, setup wizard, QR ID card printing, and account and device revocation for separated employees with records retained |
| 20 | **System monitoring** | Kiosk heartbeat and sync status, remote kiosk lock, background job status, backup status, and storage usage |

### Enhancements to core modules

| Module | Added features |
|---|---|
| **Kiosk clock-in** | Blur and darkness checks, time and device stamp on photos, suggested next log type, and voice and on-screen confirmation |
| **Employee records** | Temporary detail to another office, and dated history of position, office, and supervisor changes |
| **Schedules** | Calendar view, reusable templates, bulk assignment, Excel import, and suspension declarations by office and time range that update affected records |
| **Attendance engine** | Rule versions with effective dates, so past DTRs keep the rules in force at that time, plus a preview of the impact before a rule change is applied |
| **Corrections and approvals** | Attachments, bulk approval, approver delegation during absence, escalation of overdue requests, and a block on approving one's own request |
| **Manual fallback** | Encoding of paper logbook entries during outages, with the scanned logbook attached and each entry marked as manual |
| **DTR generation** | QR code to verify a printed DTR, version history for amended DTRs, and a tracker of submitted, verified, and missing DTRs by office |
| **Reports** | Audit pack export containing DTRs, raw logs, photos, approvals, and change history for a chosen period; leave without pay and perfect attendance reports |
| **Dashboard** | Role-specific views and 30-day attendance trends |
| **Security and privacy** | Legal hold on records under investigation, periodic access review report, settings change history, and full data export at contract end |
| **User interface** | Mobile-first responsive layouts, English and Filipino language options, and WCAG 2.1 AA accessibility |

### Anomaly detection rules

Flags never apply a penalty automatically. HR reviews each one and records the outcome.

| Flag | Trigger |
|---|---|
| **Rapid device use** | Several employees log on the same device within a short, configurable period |
| **Reused photo** | A new photo is nearly identical to an earlier one, using image comparison that does not identify the person |
| **Poor photo** | The photo is too dark, blurred, or has no face in frame |
| **Location issue** | The log is out of area, uses spoofed GPS, or implies impossible travel between two logs |
| **Unusual timing** | The log falls outside the schedule, or an offline log syncs unusually late |
| **Device change** | An employee requests repeated device changes |
| **Frequent corrections** | An employee files more missed-log corrections than the configured limit |

### Priced add-ons

These stay outside the base bid unless the TOR requires them. Size is a rough estimate: **S** is up to 1 week, **M** is 2–3 weeks, and **L** is 4 or more weeks.

| # | Add-on | Features | Size |
|---|---|---|---|
| 21 | **Leave credits and CTO** | Leave balances, accruals, compensatory overtime credits, compensatory time off, tardiness and undertime deductions, leave card, and CSC Form No. 6 | L |
| 22 | **Payroll and HRIS integration** | Payroll export or API, HRIS sync, and legacy biometric log import | M |
| 23 | **Digital signatures** | PNPKI-based signing of DTRs and approvals | M |
| 24 | **Contract of service and job order billing** | Days-worked computation and billing summaries linked to accomplishment reports | M |
| 25 | **Roster planner** | 24/7 rosters, shift swaps, and coverage views for hospitals, security, and similar units | L |
| 26 | **Assisted crew clock-in** | A team leader captures individual photos for field crews without phones | M |
| 27 | **Kiosk QR presence** | The mobile app scans a rotating QR code on an office screen to prove on-site presence | S |
| 28 | **SMS alerts** | Missed-log, deadline, and approval alerts through an SMS gateway | S |
| 29 | **Single sign-on** | LDAP, Active Directory, Google, or Microsoft sign-in | M |
| 30 | **Advanced analytics** | Trend analysis, office heatmaps, scheduled reports, and a custom report builder | M |
| 31 | **Attendance case management** | Notice drafts for habitual tardiness, employee explanations, and case tracking | M |
| 32 | **Sector presets** | Ready-made rules and reports for LGUs, hospitals, schools, and national agencies | M |
| 33 | **Multi-agency platform** | Several agencies on one deployment with separated data and settings | L |
| 34 | **Public API and webhooks** | Documented API access and event notifications for other agency systems | M |
| 35 | **Helpdesk** | In-app issue reporting, ticket tracking, and a knowledge base | S |

### User roles

| Role | Access |
|---|---|
| Super Admin | System settings, roles, and agency configuration |
| HR Admin | Employees, schedules, DTRs, reports, and photo audits |
| Timekeeper | Office-level attendance monitoring, manual logbook encoding, event attendance, and DTR submission tracking |
| Supervisor / Approver | Team attendance, approvals, and DTR verification |
| Employee | Own clock-ins, DTR, and requests |
| Auditor | Read-only access to records and audit logs |
| Kiosk Device | Clock-in functions only |

### Configurable attendance rules

- Two logs per day (in and out) or four logs per day (AM in, AM out, PM in, PM out)
- Official hours, flexitime window, core hours, and required weekly hours
- Grace period, rounding, and lunch break handling
- Habitual tardiness thresholds
- DTR cutoff and submission deadlines
- Approval levels by request type
- Photo retention period
- Whether out-of-area mobile logs are flagged or rejected
- Anomaly detection limits, and which flags are active
- Approval escalation period and delegation rules
- Required events, such as flag ceremonies, and which employees must attend
- Accomplishment report frequency by employment status

## 3. Mobile-first approach and app (Tauri 2)

The mobile app and web portal share one mobile-first React TypeScript interface and API client. A thin Tauri layer provides access to device features.

### Mobile-first principles

| Area | Approach |
|---|---|
| **Layout** | Design each screen at 360 px wide first, then add tablet and desktop layouts using min-width breakpoints |
| **Navigation** | Bottom tabs on phones for Home, Attendance, Requests, Alerts, and More; a sidebar on desktop |
| **Home screen** | A large clock-in button, today's logs, and the next expected log appear first |
| **Touch** | Touch targets of at least 48 px, main actions within thumb reach, and no hover-only controls |
| **Data display** | Tables become cards on phones, with filters in bottom sheets and key actions in a fixed bottom bar |
| **Forms** | Short steps, suitable mobile keyboards, native date and time pickers, and automatically saved drafts |
| **Offline** | Recent data stays available, actions queue on the device, and sync status is always visible |
| **Low data use** | Photos are compressed on the device, lists are paginated, and thumbnails load only when needed |
| **Readability** | Supports system text scaling and high contrast for outdoor use |

### Screen priority by role

| Role | Phone | Tablet or desktop |
|---|---|---|
| **Employee** | All features | All features |
| **Supervisor / Approver** | All features | All features |
| **Timekeeper** | Monitoring, event attendance, and submission tracking | Manual logbook encoding |
| **HR Admin** | Dashboard, approvals, anomaly review, and photo audit | Bulk import, schedule setup, bulk DTR printing, and detailed reports |
| **Super Admin** | System status and alerts | Agency configuration, roles, and attendance rules |
| **Auditor** | Record lookup | Audit pack review and export |
| **Kiosk Device** | Not applicable | Dedicated tablet layout |

### Performance targets

These are proposed targets to confirm during the bid demo phase.

| Measure | Target |
|---|---|
| Taps to clock in after opening the app | 3 or fewer |
| Clock-in confirmation on a typical 4G connection | Within 5 seconds |
| Screen ready on a mid-range phone over 4G | Within 3 seconds |
| JavaScript per initial route | Up to 200 KB compressed |
| Compressed clock-in photo | About 150 KB |

### Device testing

- Entry-level Android phone with a 360 px wide screen
- Mid-range Android phone
- Small and current iPhone models
- 10-inch tablet for the kiosk
- Desktop browser for HR and admin work

Minimum Android and iOS versions will be set after the mobile camera test.

### UI design workflow (Impeccable)

All interface work uses the Impeccable design skill in Claude Code, so screens follow one design system instead of being styled screen by screen.

| Stage | Work |
|---|---|
| **Phase 0** | Shape the clock-in, home, and DTR screens at phone width, and set the design tokens: color, type, spacing, and light and dark themes |
| **Build** | Design each new screen with Impeccable before coding it, starting with the phone layout |
| **Review** | Run a critique and audit on each finished screen for hierarchy, accessibility, responsive behavior, and error and empty states |
| **Polish** | Before each release, run a polish and hardening pass, then check the layout on the test devices |
| **Record** | Record the shipped design system in `DESIGN.md`, so later work stays consistent |

Government users are not designers, so the aim is a plain, trustworthy interface with clear status and error messages. Visual flourish is not a goal.

### Features

| Area | Features |
|---|---|
| **Account** | Sign-in, device pairing, optional fingerprint or face unlock using the phone's own security |
| **Clock-in** | Live selfie, GPS capture, geofence result, and confirmation screen |
| **Offline** | Queues logs on the device and syncs them when the connection returns |
| **My attendance** | Today's status, log history with photos, and monthly DTR view and PDF download |
| **Requests** | Corrections, pass slips, official business, travel orders, and leave tagging |
| **Accomplishments** | Submit daily or period accomplishment reports with attachments |
| **Events** | View required events and check in by photo or QR code |
| **Announcements** | Read agency memos and acknowledge receipt |
| **Supervisors** | Approve or reject requests, review accomplishments, delegate approvals, and view team attendance |
| **Notifications** | Clock-in reminders, DTR deadlines, and request status updates |
| **Privacy** | Privacy notice acceptance and clear camera and location permission prompts |
| **Updates** | Minimum-version check that blocks outdated app builds |

### Mobile controls

- **Device binding:** each employee has one approved device. HR approves any device change.
- **Geofencing:** the server evaluates location against the office, an approved work-from-home address, or an official business location.
- **Trusted time:** online logs use server time. Offline logs use elapsed device time since the last server sync, and are flagged if the device restarted or its clock changed.
- **Secure storage:** tokens and queued photos are encrypted using Android Keystore and iOS Keychain.
- **Mock location detection:** flags spoofed GPS where the platform allows detection.
- **Channel control:** HR sets whether each employee clocks in by mobile, kiosk, or both.

### Tauri components

| Need | Approach |
|---|---|
| Camera | **Custom Kotlin and Swift plugin** for live capture |
| GPS | Official geolocation plugin |
| QR device pairing | Official barcode scanner plugin |
| Device unlock | Official biometric plugin |
| Local reminders | Official notification plugin |
| Offline queue | Official SQL plugin with SQLite |
| Secure tokens and mock location detection | Custom native code |

### Technical risks

| Risk | Mitigation |
|---|---|
| Android WebView camera access needs native permission handling | Build the custom camera plugin and test it during the bid demo phase |
| Camera behavior varies across low-cost Android phones | Test on a device matrix that includes entry-level models |
| Server-sent push notifications need extra FCM and APNs work | Start with local reminders and in-app status, then add push if the TOR requires it |
| iOS builds require a Mac with Xcode and an Apple Developer account | Budget for both, and agree who owns the accounts |
| App store review can delay release | Submit early, and use signed APKs for Android pilots |

### Distribution

- **Android:** signed APK for direct installation or managed devices, or an App Bundle through Google Play.
- **iOS:** App Store distribution through an Apple Developer Program account.

## 4. Technical architecture

```text
Office kiosk (PWA)  ─┐
Web portal (React)  ─┼─ HTTPS ─> Go Fiber API ─┬─> PostgreSQL
Mobile app (Tauri)  ─┘                         ├─> S3-compatible photo storage (MinIO)
                                               └─> Background worker
                                                   (attendance, PDFs, sync, retention)
```

### Repository structure

```text
CLAUDE.md         Project rules for Claude Code
DESIGN.md         Design system record (written after the first shipped screens)
PLAN.md           This plan
apps/
  web/            React portal and kiosk PWA
  mobile/         Tauri 2 Android and iOS app
packages/
  ui/             Shared mobile-first React components
  api-client/     Typed API client
  shared/         Shared types and validation
server/
  cmd/api/        Fiber HTTP server
  cmd/worker/     Background jobs
  internal/       One package per business module
  migrations/     Database migrations
deploy/           Docker Compose and environment templates
```

### Core data model

| Area | Tables |
|---|---|
| Organization | `agency_settings`, `offices`, `positions` |
| People and access | `employees`, `users`, `roles`, `user_roles` |
| Assignments | `employee_assignments`, `office_details` |
| Devices | `devices`, `device_bindings`, `device_heartbeats`, `geofences` |
| Scheduling | `schedules`, `schedule_assignments`, `holidays`, `work_suspensions` |
| Attendance | `attendance_logs`, `attendance_photos`, `daily_records`, `attendance_flags`, `manual_log_batches` |
| Rules | `attendance_rule_versions` |
| Requests | `requests`, `request_approvals`, `approval_delegations` |
| DTR | `dtr_periods`, `dtr_documents`, `dtr_submissions` |
| Events | `events`, `event_participants`, `event_attendance` |
| Accomplishments | `accomplishment_reports`, `accomplishment_attachments` |
| Communication | `announcements`, `announcement_acknowledgements`, `notifications` |
| System | `audit_logs`, `system_settings`, `legal_holds` |

Key design rules:

- `attendance_logs` is append-only and partitioned by month.
- `daily_records` is computed from raw logs, so it can always be regenerated.
- Photos live in object storage. PostgreSQL stores only the file reference and SHA-256 hash.
- Sync uses idempotency keys so a retried upload cannot create duplicate logs.

### Security baseline

- TLS for all traffic, and encryption at rest for photos and backups
- Argon2id password hashing, with multi-factor authentication for admin roles
- Short-lived access tokens with rotating refresh tokens
- Short-lived signed URLs for photo access
- Server-side permission checks on every endpoint
- Rate limiting and account lockout
- Automated encrypted backups with tested restore
- Security testing against OWASP ASVS (web and API) and OWASP MASVS (mobile)

### Data privacy (RA 10173)

- Complete a Privacy Impact Assessment before go-live.
- Collect only the photo, time, device, and location data needed for attendance.
- Capture location only at clock-in and clock-out, with no background tracking.
- Restrict photo access by role, and log every photo view.
- Delete photos automatically when the agency's retention period ends.
- Provide a privacy notice and a process for data subject requests.

### Storage sizing

Assuming four logs per day, 22 workdays per month, and 150 KB per compressed photo:

| Employees | Photos per month | Storage per month | Storage per year |
|---:|---:|---:|---:|
| 100 | 8,800 | 1.3 GB | 16 GB |
| 500 | 44,000 | 6.6 GB | 79 GB |
| 1,000 | 88,000 | 13.2 GB | 158 GB |

## 5. Delivery plan

The estimated duration for the base scope is **about 23 to 24 weeks** with phases run in sequence. Overlapping work can shorten this, depending on team size. Priced add-ons are not included.

Mobile work starts in the bid demo and continues through every phase. Each feature is built and accepted on a phone first, then extended to tablet and desktop. Android and iOS test builds are released from Phase 2 onward.

| Phase | Scope | Duration |
|---|---|---|
| 0. Bid demo | Mobile clock-in flow, camera test, phone-size design system (shaped with Impeccable), basic attendance logs, and sample Form 48 | 2–3 weeks |
| 1. Foundation | API, authentication, administration, employee records, onboarding, schedules, security, language support, and mobile app shell | 4 weeks |
| 2. Mobile attendance | Mobile clock-in, device binding, geofencing, offline sync, attendance engine, and employee attendance views | 6 weeks |
| 3. Mobile self-service | Requests, approvals, accomplishments, events, announcements, and notifications | 4 weeks |
| 4. Kiosk and HR tools | Kiosk, DTR generation, photo audit, reports, dashboard, manual fallback, anomaly detection, and system monitoring | 4 weeks |
| 5. Rollout | Device testing, store submission, security testing, user acceptance, training, and go-live | 3 weeks |

### Deliverables

- Web portal, kiosk application, and Android and iOS apps
- Source code and deployment scripts, subject to the TOR
- System, API, and database documentation
- User, administrator, and deployment manuals
- Test results and security assessment report
- Training and post-deployment support

## 6. Items to confirm before bidding

### From the Terms of Reference

- Approved Budget for the Contract (ABC) and delivery deadline
- Number of employees, offices, and kiosks
- Who supplies kiosks, tablets, servers, and network equipment
- Hosting location and data residency requirements
- **Whether automatic face recognition is required.** This plan excludes it, so a TOR requiring it changes the scope and price.
- Whether iOS is required, and who owns the Apple and Google developer accounts
- Whether employees will use personal phones or agency-issued devices, and what share of staff have a suitable phone
- Data migration, integrations, and payroll requirements
- Which priced add-ons the agency requires, especially leave credits, payroll integration, and digital signatures
- Warranty, support period, and service levels
- Source code ownership and licensing
- Required bidder eligibility, experience, and documents under RA 12009 and its IRR

### Rules needing official verification

These were checked mainly against secondary sources, or could not be confirmed from an official source:

- National Privacy Commission requirements for employee face photos and location data
- Current CSC rules on habitual tardiness, undertime, and penalties
- Current official CSC Form No. 48 layout
- Attendance recording requirements for employees under flexible work arrangements
- Current CSC Form No. 6 layout and leave computation rules, if the leave add-on is offered
- PNPKI technical and enrollment requirements, if the digital signature add-on is offered
- COA documentation requirements for the audit pack and for contract of service and job order billing

## 7. References

- [RA 12009 Implementing Rules and Regulations](https://www.dbm.gov.ph/wp-content/uploads/Issuances/2025/GPPB-Resolution/IRR-RA-12009-Resolution-No-02-2025.pdf)
- [CSC Memorandum Circular No. 06, s. 2022: Flexible Work Arrangements](https://legal.uplb.edu.ph/wp-content/uploads/2023/07/2022-CSC-MC-06-Policies-on-Flexible-Working-Arrangements-in-the-Government.pdf)
- [CSC Advisory on Flexible Work Arrangements](https://csc.gov.ph/phocadownload/userupload/advisories/CSC-Advisory-on-Adoption-of-FWA-to-Improve-Traffic-in-Manila.pdf)
- [COA-DBM Joint Circular No. 1, s. 2022](https://hrdo.upm.edu.ph/sites/default/files/2022-10/COA-DBM-JOINT-CIRCULAR-NO-1-s2022_compressed.pdf)
- [Tauri 2 plugin directory](https://v2.tauri.app/plugin/)
- [Tauri mobile plugin development](https://v2.tauri.app/develop/plugins/develop-mobile/)
- [Tauri discussion on Android camera permissions](https://github.com/orgs/tauri-apps/discussions/12732)
- [Tauri Google Play distribution](https://v2.tauri.app/distribute/google-play/)
- [Tauri App Store distribution](https://v2.tauri.app/distribute/app-store/)
