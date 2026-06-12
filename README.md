# PRD-028 Restore Points

Go + Vue + PostgreSQL implementation for restore points list sorted by latest backup time, showing age from now, and highlighting records beyond RPO.

## Features

- Restore point CRUD
- Fields fully implemented:
  - ID
  - Name
  - Company Name
  - Owner
  - Lifecycle Stage
  - Status
  - Latest Backup Time
  - RPO Minutes
  - Created At
  - Updated At
- List sorting by latest backup time desc by default
- Age from now display
- Highlight rows beyond RPO
- Filtering
- Sorting
- Editing
- Deleting
- PostgreSQL persistence via pgx

## Project Structure

- `backend` Go API server
- `frontend` Vue 3 + Vite app
- `db/schema.sql` PostgreSQL schema and seed

## Requirements

- Go 1.22+
- Node.js 20+
- PostgreSQL 14+

## Database

Create a database and set environment variable:

```bash
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/restore_points?sslmode=disable
```

or

```bash
export LF_PG_DSN=postgres://postgres:postgres@localhost:5432/restore_points?sslmode=disable
```

Initialize schema:

```bash
psql "$DATABASE_URL" -f db/schema.sql
```

## Run Backend

```bash
cd backend
go mod tidy
go run .
```

Backend listens on `http://localhost:8080`.

## Run Frontend

```bash
cd frontend
npm install
npm run dev
```

Frontend runs on `http://localhost:5173` and proxies `/api` to backend.

## API

- `GET /api/restore-points`
- `POST /api/restore-points`
- `GET /api/restore-points/:id`
- `PUT /api/restore-points/:id`
- `DELETE /api/restore-points/:id`
- `GET /api/options`

## Filters

Supported query params on list endpoint:

- `q`
- `company_name`
- `owner`
- `lifecycle_stage`
- `status`
- `rpo_breached=true|false`
- `sort_by=id|name|company_name|owner|lifecycle_stage|status|latest_backup_time|rpo_minutes|created_at|updated_at|age_minutes`
- `sort_order=asc|desc`
- `page`
- `page_size`

Default sort:

- `sort_by=latest_backup_time`
- `sort_order=desc`
