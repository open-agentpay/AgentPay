---
title: Deployment Guide
description: Containerized deployment instructions, Docker Compose setups, and cloud environment requirements.
---

# Deployment Guide

::: warning Experimental Notice
Version 0.1.x is an experimental release. Complete thorough testnet drills before deploying to staging or production.
:::

## Docker Compose Setup

The repository root includes a ready-to-run `docker-compose.yml`:

```bash
docker compose up -d
```

Included services:
1. **PostgreSQL 16**: Pre-seeded with the multitenant schema from `infra/postgres/schema.sql`.
2. **AgentPay Gateway**: The Go gateway container exposing port 8080 (proxy) and 8081 (admin API).
