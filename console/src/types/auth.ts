import type { Tenant } from './tenant';

export type UserRole = 'owner' | 'admin' | 'operator' | 'viewer';

export interface UserProfile {
  member_id: string;
  email: string;
  role: UserRole;
  status: 'active' | 'invited' | 'disabled';
  tenant_id: string;
  name?: string;
  created_at?: number;
  updated_at?: number;
}

export interface AuthState {
  token: string | null;
  user: UserProfile | null;
  tenant: Tenant | null;
  permissions: string[];
  isAuthenticated: boolean;
}

export interface LoginRequest {
  email: string;
  password?: string;
  role?: UserRole;
}

export interface LoginResponse {
  token: string;
  user: UserProfile;
  tenant?: Tenant;
}

export interface RegisterRequest {
  tenant_name: string;
  email: string;
  password: string;
}

export interface RegisterResponse {
  token: string;
  user: UserProfile;
  tenant: Tenant;
}

export interface MeResponse {
  user: UserProfile;
  tenant: Tenant;
  permissions: string[];
}
