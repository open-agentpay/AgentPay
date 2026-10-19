---
title: 生产部署说明
description: AgentPay 网关容器化部署与基础设施配置。
---

# 生产部署说明

::: warning 生产提示
当前版本（v0.1.x）属于实验性版本，生产部署前请确保完成完备的测试网演练并配合风控预警。
:::

## Docker Compose 部署

仓库根目录提供了完整的 `docker-compose.yml`：

```bash
docker compose up -d
```

包含组件：
1. **PostgreSQL 16**：已预装 `infra/postgres/schema.sql` 多租户表结构。
2. **AgentPay Gateway**：Go 网关容器，映射 8080（代理）与 8081（管理端）。
