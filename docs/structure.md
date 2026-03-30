# ParkOps — Submission Folder Structure

Task ID: 17
Project Type: fullstack
Stack: Go (Gin) + Templ + PostgreSQL

---

## ZIP Root Layout

```
17/
├── docs/
│   ├── design.md
│   ├── api-spec.md
│   ├── questions.md
│   ├── action-plan.md
│   ├── features.md
│   ├── requirements.md
│   ├── testing-plan.md
│   └── AI-self-test.md
├── repo/                             # project code lives directly here
├── sessions/
│   ├── develop-1.json                # primary development session
│   └── bugfix-1.json                 # remediation session (if needed)
├── metadata.json
├── prompt.md
└── questions.md
```

### metadata.json

```json
{
  "prompt": "...",
  "project_type": "fullstack",
  "frontend_language": "go",
  "backend_language": "go",
  "frontend_framework": "templ",
  "backend_framework": "gin",
  "database": "postgresql"
}
```

---

## repo/ — Full Project Structure

```
repo/
├── cmd/
│   └── web/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── config.go
│   │   └── router.go
│   │
│   ├── auth/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   ├── password.go
│   │   ├── lockout.go
│   │   └── session.go
│   │
│   ├── rbac/
│   │   ├── middleware.go
│   │   ├── service.go
│   │   └── model.go
│   │
│   ├── users/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── facilities/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── zones/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── rates/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── members/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── vehicles/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── drivers/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── reservations/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   ├── hold_engine.go
│   │   └── calendar.go
│   │
│   ├── capacity/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   └── reconciliation.go
│   │
│   ├── exceptions/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── devices/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   ├── ingest.go
│   │   └── dedupe.go
│   │
│   ├── tracking/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   ├── smoother.go
│   │   └── stop_detector.go
│   │
│   ├── notifications/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   ├── dispatcher.go
│   │   └── rules.go
│   │
│   ├── campaigns/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── tasks/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── tags/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── segments/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── model.go
│   │   └── runner.go
│   │
│   ├── analytics/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── queries.go
│   │
│   ├── exports/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── csv.go
│   │   ├── excel.go
│   │   └── pdf.go
│   │
│   ├── audit/
│   │   ├── service.go
│   │   ├── repo.go
│   │   └── model.go
│   │
│   ├── jobs/
│   │   ├── worker.go
│   │   ├── scheduler.go
│   │   └── registry.go
│   │
│   ├── db/
│   │   ├── postgres.go
│   │   ├── tx.go
│   │   └── migrate.go
│   │
│   ├── web/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   ├── templates/
│   │   │   ├── layouts/
│   │   │   │   ├── base.templ
│   │   │   │   └── auth.templ
│   │   │   ├── pages/
│   │   │   │   ├── login.templ
│   │   │   │   ├── dashboard.templ
│   │   │   │   ├── reservations.templ
│   │   │   │   ├── capacity.templ
│   │   │   │   ├── notifications.templ
│   │   │   │   ├── campaigns.templ
│   │   │   │   ├── segments.templ
│   │   │   │   ├── analytics.templ
│   │   │   │   ├── devices.templ
│   │   │   │   ├── audit.templ
│   │   │   │   └── admin/
│   │   │   │       ├── users.templ
│   │   │   │       └── content-rules.templ
│   │   │   ├── partials/
│   │   │   │   ├── activity-feed.templ
│   │   │   │   ├── conflict-warning.templ
│   │   │   │   ├── zone-card.templ
│   │   │   │   └── exception-list.templ
│   │   │   └── components/
│   │   │       ├── button.templ
│   │   │       ├── modal.templ
│   │   │       ├── table.templ
│   │   │       └── alert.templ
│   │   └── static/
│   │       ├── css/
│   │       │   └── app.css
│   │       ├── js/
│   │       │   └── poll.js
│   │       └── img/
│   │
│   └── platform/
│       ├── logger/
│       ├── clock/
│       ├── security/
│       ├── pagination/
│       └── validator/
│
├── migrations/
│   ├── 0001_init.sql
│   ├── 0002_auth.sql
│   ├── 0003_master_data.sql
│   ├── 0004_reservations.sql
│   ├── 0005_notifications.sql
│   ├── 0006_devices.sql
│   ├── 0007_tags_segments.sql
│   └── 0008_analytics.sql
│
├── unit_tests/
│   ├── auth_test.go
│   ├── capacity_test.go
│   ├── device_test.go
│   ├── notification_test.go
│   ├── segment_test.go
│   ├── tracking_test.go
│   ├── reconciliation_test.go
│   └── export_test.go
│
├── API_tests/
│   ├── testdb_test.go
│   ├── auth_api_test.go
│   ├── reservations_api_test.go
│   ├── capacity_api_test.go
│   ├── devices_api_test.go
│   ├── notifications_api_test.go
│   ├── segments_api_test.go
│   ├── analytics_api_test.go
│   └── rbac_api_test.go
│
├── scripts/
│   ├── seed.sh
│   └── gen.sh
│
├── run_tests.sh
├── docker-compose.yml
├── Dockerfile
├── .env.example
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## What Must NOT Be in the ZIP

- no `vendor/` directory
- no compiled binaries
- no `.env` with real credentials (only `.env.example`)
- no temp or scratch files

---

## Sessions Naming Rules

- primary development session → `sessions/develop-1.json`
- remediation session → `sessions/bugfix-1.json`
- additional sessions → `develop-2.json`, `bugfix-2.json`, etc.

---

## Submission Checklist

- [ ] `docker compose up` completes without errors
- [ ] Cold start tested in clean environment
- [ ] README URLs, ports, and credentials match running app
- [ ] `docs/design.md` and `docs/api-spec.md` present
- [ ] `docs/questions.md` has question + assumption + solution for each item
- [ ] `unit_tests/` and `API_tests/` exist in `repo/`, `run_tests.sh` passes
- [ ] No `vendor/`, cache, or compiled output in ZIP
- [ ] No real credentials in any config file
- [ ] All prompt requirements implemented — no silent substitutions
- [ ] `sessions/develop-1.json` trajectory file present
- [ ] `metadata.json` at root with all required fields
- [ ] `prompt.md` at root, unmodified
- [ ] Running application screenshots captured
- [ ] Self-test report generated and attached
