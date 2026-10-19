import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import type { UserRole } from '@/types/auth';

declare module 'vue-router' {
  interface RouteMeta {
    title: string;
    requiresAuth?: boolean;
    roles?: UserRole[];
    icon?: string;
    hideInMenu?: boolean;
  }
}

export const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录', requiresAuth: false, hideInMenu: true }
  },
  {
    path: '/',
    component: () => import('@/layout/index.vue'),
    redirect: '/dashboard',
    meta: { title: '工作台', requiresAuth: true },
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: {
          title: '总览看板',
          icon: 'Odometer',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'projects',
        name: 'Projects',
        component: () => import('@/views/projects/index.vue'),
        meta: {
          title: '业务项目管理',
          icon: 'Folder',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'members',
        name: 'Members',
        component: () => import('@/views/members/index.vue'),
        meta: {
          title: '组织团队与成员 (T4.1)',
          icon: 'User',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'routes',
        name: 'Routes',
        component: () => import('@/views/routes/index.vue'),
        meta: {
          title: '路由与上游 (T4.2)',
          icon: 'Connection',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'versions',
        name: 'Versions',
        component: () => import('@/views/versions/index.vue'),
        meta: {
          title: '发布与回滚 (T4.4)',
          icon: 'Files',
          requiresAuth: true,
          roles: ['owner', 'admin'] // Operator & Viewer cannot access releases
        }
      },
      {
        path: 'wallet',
        name: 'Wallet',
        component: () => import('@/views/wallet/index.vue'),
        meta: {
          title: '虚拟钱包与账本 (T4.6)',
          icon: 'Wallet',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'agents',
        name: 'Agents',
        component: () => import('@/views/agents/index.vue'),
        meta: {
          title: 'Agent 管理与审计 (T4.7)',
          icon: 'Cpu',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'payments',
        name: 'Payments',
        component: () => import('@/views/payments/index.vue'),
        meta: {
          title: '消费与履约审计 (T5.3)',
          icon: 'Tickets',
          requiresAuth: true,
          roles: ['owner', 'admin', 'operator', 'viewer']
        }
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('@/views/audit/index.vue'),
        meta: {
          title: '管理操作审计 (T5.4)',
          icon: 'DocumentChecked',
          requiresAuth: true,
          roles: ['owner', 'admin']
        }
      }
    ]
  },
  {
    path: '/403',
    name: 'Forbidden',
    component: () => import('@/views/403.vue'),
    meta: { title: '403 无权访问', requiresAuth: true, hideInMenu: true }
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/404.vue'),
    meta: { title: '404 页面不存在', hideInMenu: true }
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes
});

// Navigation Guards: Authentication & RBAC Authorization
router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore();

  // Document title
  if (to.meta.title) {
    document.title = `${to.meta.title} - AgentPay Console`;
  }

  // Handle public login page
  if (to.path === '/login') {
    if (authStore.isAuthenticated) {
      return next('/');
    }
    return next();
  }

  // Requires authentication check
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return next({
      path: '/login',
      query: { redirect: to.fullPath }
    });
  }

  // RBAC Permission check
  if (to.meta.roles && to.meta.roles.length > 0) {
    if (!authStore.hasRole(to.meta.roles)) {
      return next('/403');
    }
  }

  next();
});

export default router;
