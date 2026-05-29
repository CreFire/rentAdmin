/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
*/

import { ExcelImportSummary, TenantRecord } from './types';

export const generateId = () => Date.now().toString(36) + Math.random().toString(36).substring(2);

// Fetch all tenants
export async function fetchAllTenants(): Promise<TenantRecord[]> {
  const response = await fetch('/api/tenants');
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  const data = await response.json();
  return Array.isArray(data) ? data : [];
}

// Create or update tenant
export async function createOrUpdateTenant(tenant: TenantRecord): Promise<{ id: string }> {
  const response = await fetch('/api/tenants', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(tenant),
  });
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

// Update tenant
export async function updateTenant(id: string, tenant: TenantRecord): Promise<{ id: string, message: string }> {
  const response = await fetch(`/api/tenants/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(tenant),
  });
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

// Fetch tenants by room number
export async function fetchTenantsByRoom(roomNumber: string): Promise<TenantRecord[]> {
  const response = await fetch(`/api/tenants/room/${roomNumber}`);
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  const data = await response.json();
  return Array.isArray(data) ? data : [];
}

export async function deleteTenantByRoom(roomNumber: string): Promise<{ message: string; deletedRows: number; roomNumber: string }> {
  const response = await fetch(`/api/tenants/room/${encodeURIComponent(roomNumber)}`, {
    method: 'DELETE',
  });
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

export async function deleteTenantById(id: string): Promise<{ message: string; deletedRows: number; id: string }> {
  const response = await fetch(`/api/tenants/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

// Fetch income summary
export async function fetchIncomeSummary(date?: string): Promise<{
  totalReceivable: number;
  totalReceived: number;
  totalUtilityIncome: number;
  outstandingBalance: number;
  dateFilter: string;
}> {
  let url = '/api/income-summary';
  if (date) {
    url += `?date=${encodeURIComponent(date)}`;
  }
  
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

export async function importExcelTenants(): Promise<ExcelImportSummary> {
  const response = await fetch('/api/excel/import', {
    method: 'POST',
  });
  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || `HTTP error! status: ${response.status}`);
  }
  return response.json();
}
