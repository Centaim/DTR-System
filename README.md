# DTR System

Daily Time Record system for Philippine government agencies. Employees clock in and out with a live photo as proof of attendance. See `PLAN.md` for scope and `CLAUDE.md` for project rules.

**Status:** Phase 0 (bid demo). Working: PIN sign-in, live-photo clock-in, four daily slots, offline queue with sync, English and Filipino. Not built yet: kiosk, DTR (Form 48) output, approvals, HR tools, the Tauri mobile shell.

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

Open http://localhost:5173 and sign in with employee number `DEMO-0001` and PIN `123456`. This is synthetic demo data.

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
| `SEED_DEMO` | `false` | Insert the synthetic demo employee |

## Checks

```sh
cd server && go vet ./... && go test ./...
cd apps/web && pnpm typecheck && pnpm build
```
