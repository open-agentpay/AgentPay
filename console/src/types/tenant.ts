export interface Tenant {
  tenant_id: string;
  name: string;
  status: 'active' | 'suspended' | 'terminated';
  created_at?: string;
}

export interface Project {
  project_id: string;
  tenant_id: string;
  name: string;
  budget_cap_base_units: number;
  status: 'active' | 'archived';
  created_at?: string;
}
