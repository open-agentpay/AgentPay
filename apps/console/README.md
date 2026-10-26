# AgentPay Console (Vue 3 + Element Plus)

AgentPay 管理控制台是面向自主 AI Agent 支付基础设施与财务防火墙的 Web 端可视化操作界面。

## 1. 对应规划与里程碑

本目录完整实现了 **[T4.1 建立控制台基础框架](../docs/development-tasks-v1.1.md#T4.1-建立控制台基础框架)**，并为后续 T4.2 ~ T4.7 提供了完整的基础架构与挂载点：

- **Vue 3 + Vite + Element Plus + Pinia + Vue Router** 现代化技术栈。
- **登录与会话处理 (Session & Auth)**：支持 JWT Bearer Token 规范、状态持久化与自动过期拦截。
- **租户与项目多级切换 (Multi-Tenant & Project Switcher)**：
  - 严格以 `Tenant` 为数据隔离边界。
  - 项目选择器随所选租户联动级联过滤。
  - 请求自动注入 `X-Tenant-Id` 和 `X-Project-Id` 头部。
- **细粒度角色权限控制 (RBAC)**：
  - 角色模型：`owner`、`admin`、`operator`、`viewer`（严格对齐 [`specs/multitenant-data-model.md`](../specs/multitenant-data-model.md)）。
  - 路由级守卫拦截（无权访问时阻断并重定向至 `/403`）。
  - 组件级细粒度权限控制 (`<PermissionGate :roles="['owner', 'admin']">`)。
  - 动态侧边栏菜单（根据当前登录角色自动过滤可见功能）。
  - 开发/联调环境内置 **一键快速切换角色** 功能。

---

## 2. 目录结构

```
console/
├── index.html                   # HTML 模板入口
├── package.json                 # 依赖配置与脚本
├── vite.config.ts               # Vite 构建与代理配置
├── tsconfig.json                # TypeScript 严格类型配置
├── src/
│   ├── main.ts                  # Vue 应用主入口
│   ├── App.vue                  # 根视图组件
│   ├── api/
│   │   ├── client.ts            # Axios 客户端与拦截器 (注入 Token & 租户头)
│   │   └── mockData.ts          # 符合多租户规范的 Mock 数据
│   ├── components/
│   │   └── PermissionGate.vue   # RBAC 细粒度权限控制组件
│   ├── layout/
│   │   └── index.vue            # 框架布局 (侧边栏、租户/项目切换、角色标识、用户下拉)
│   ├── router/
│   │   └── index.ts             # 路由配置与全局鉴权/RBAC导航守卫
│   ├── stores/
│   │   ├── auth.ts              # 会话认证与角色权限 Pinia Store
│   │   └── tenant.ts            # 租户与项目切换联动 Pinia Store
│   ├── types/
│   │   ├── auth.ts              # 用户身份、角色 (UserRole)、会话类型
│   │   └── tenant.ts            # 租户 (Tenant)、项目 (Project) 类型定义
│   └── views/
│       ├── login/               # 登录视图 (支持 4 类角色一键体验)
│       ├── dashboard/           # 工作台总览与当前角色 RBAC 权限矩阵
│       ├── routes/              # 路由与上游管理 (T4.2 占位与预览)
│       ├── versions/            # 发布、快照与回滚 (T4.4，限制 Owner/Admin)
│       ├── wallet/              # 虚拟钱包与流水对账 (T4.6)
│       ├── agents/              # Agent 管理与熔断时间线 (T4.7)
│       ├── audit/               # 审计日志 (T5，限制 Owner/Admin)
│       ├── 403.vue              # 403 权限不足视图
│       └── 404.vue              # 404 未找到视图
└── tests/
    └── auth_and_rbac.spec.ts    # 单元测试 (会话持久化、RBAC 矩阵、租户切换)
```

---

## 3. 本地运行与调试

```bash
# 进入 console 目录安装依赖
cd console
npm install

# 启动本地开发服务 (默认端口 3000)
npm run dev

# 运行自动化单元测试
npm run test

# 生产环境类型检查与构建打包
npm run build
```

在项目根目录下也可以通过 Makefile 快捷启动：
```bash
make console-dev
make console-test
make console-build
```
