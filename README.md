# SupVault

A Kubernetes-native data protection UI built on top of [Velero](https://velero.io), inspired by [Kasten K10](https://docs.kasten.io).

## Overview

SupVault provides a clean, intuitive web interface for managing Kubernetes backup and restore operations powered by Velero. It aims to bring the ease-of-use of commercial solutions like Kasten K10 to the open-source Velero ecosystem.

## Architecture

```
supvault/
├── supvault-backend/    # Go + Gin REST API (wraps Velero CRDs)
├── supvault-frontend/   # Vue 3 + Element Plus UI
└── supvault-helm/       # Helm Chart for K8S deployment
```

## Features (MVP)

- 📊 Dashboard — cluster overview, backup status summary
- 🗂 Applications — namespace list with protection status
- 💾 Backups — create, list, view, delete backups
- ♻️ Restores — restore from backup, cross-namespace support
- ⏰ Policies — scheduled backup with cron + retention rules
- 🗄 Storage Locations — S3/MinIO/Azure/GCS configuration
- ⚙️ Settings — system info, Velero connection status

## Prerequisites

- Kubernetes cluster (Docker Desktop, kind, or production)
- Velero installed in the cluster
- MinIO or S3-compatible storage

## Quick Start (Local Development)

### Backend
```bash
cd supvault-backend
go mod tidy
go run main.go
```

### Frontend
```bash
cd supvault-frontend
npm install
npm run dev
```

## Tech Stack

- **Backend**: Go, Gin, client-go, controller-runtime
- **Frontend**: Vue 3, TypeScript, Element Plus
- **Deployment**: Helm Chart on Kubernetes
- **Storage**: Velero + S3/MinIO

## License

Apache 2.0
