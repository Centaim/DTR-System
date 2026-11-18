# DTR System

Daily Time Record system for Philippine government agencies. Employees clock in and out with a live photo as proof of attendance. See `PLAN.md` for scope and `CLAUDE.md` for project rules.

**Status:** Phase 0 (bid demo). Working: PIN sign-in, live-photo clock-in, four daily slots, offline queue with sync, English and Filipino, and a printable monthly DTR in the CSC Form No. 48 layout with computed undertime, roles, HR admin for employees, dated schedules, and requests (missed-log corrections, leave, official business, travel orders) with supervisor approval that feeds the DTR, photo audit with rule-based anomaly flags, a holiday calendar, an HR dashboard, a monthly attendance summary with CSV export, spreadsheet import of employees (.xlsx or .csv, with a preview step), printable QR ID cards, and an activity log. Not built yet: kiosk, the Tauri mobile shell. Admin, requests, approvals and audit screens are English only for now.

## Run locally

Requires Go, Node with pnpm, and Docker.

```sh
# 1. Database (Postgres on localhost:5433)
docker compose -f deploy/docker-compose.yml up -d db

# 2. API (migrates the database and seeds one synthetic employee)
cd server
JWT_SECRET=local-dev-secret-local-dev-secret-123 SEED_DEMO=true go run ./cmd/api

# 3. Web app, in another terminal
cd apps/web
pnpm install
pnpm dev
```

Open http://localhost:5173 and sign in with any of these synthetic demo accounts (PIN `123456` for all):

| Employee number | Role |
|---|---|
| `DEMO-0001` | Employee |
| `HR-0001` | HR admin: dashboard, employees, schedules and holidays, photo audit, reports |
| `ADMIN-0001` | Super admin: also HR and super admin accounts |

Accounts HR creates get a temporary PIN and must choose their own at first sign-in.

Camera access needs `localhost` or HTTPS. Testing from a phone on the local network needs an HTTPS tunnel or a local certificate.

## API settings

| Variable | Default | Purpose |
|---|---|---|
| `JWT_SECRET` | required, 32+ characters | Signs access tokens |
| `DATABASE_URL` | `postgres://dtr:dtr@localhost:5433/dtr?sslmode=disable` | Postgres connection |
| `ADDR` | `:8080` | Listen address |
| `PHOTO_DIR` | `./data/photos` | Local photo storage until MinIO is added |
| `MAX_PHOTO_MB` | `5` | Largest accepted photo |
| `MIN_GAP_SECONDS` | `60` | Blocks accidental double taps (set to 0 in demos) |
| `CORS_ORIGINS` | `http://localhost:5173` | Allowed web origins |
| `SEED_DEMO` | `false` | Insert the synthetic demo accounts |
| `REQUEST_WINDOW_DAYS` | `30` | How far back an employee may date a request |
| `HABITUAL_LATE_DAYS` | `10` | Late days in a month that flag habitual tardiness. Confirm the rule with HR |

## Checks

```sh
cd server && go vet ./... && go test ./...
cd apps/web && pnpm typecheck && pnpm build
```
