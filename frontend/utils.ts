/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
*/

import { ExcelImportSummary, GenerateBillsResult, RoomPaymentResult, TenantRecord } from './types';

export const generateId = () => Date.now().toString(36) + Math.random().toString(36).substring(2);

const ADMIN_TOKEN_KEY = 'rentadmin_admin_token';
let tokenPrompt: Promise<string | null> | null = null;
let tokenPromptDeclined = false;

function readAdminToken(): string {
  try {
    return localStorage.getItem(ADMIN_TOKEN_KEY) || '';
  } catch {
    return '';
  }
}

function saveAdminToken(token: string) {
  try {
    localStorage.setItem(ADMIN_TOKEN_KEY, token);
  } catch {
    // Storage unavailable (private mode): the token only lives for this request.
  }
}

// Ask once at a time, so concurrent 401s (polling + user action) share one prompt.
function promptAdminToken(): Promise<string | null> {
  if (tokenPromptDeclined) return Promise.resolve(null);
  if (!tokenPrompt) {
    tokenPrompt = Promise.resolve().then(() => {
      const input = window.prompt('请输入管理员口令（后端启动时控制台打印的 Admin token，或环境变量 ADMIN_TOKEN 的值）');
      tokenPrompt = null;
      if (!input?.trim()) {
        tokenPromptDeclined = true;
        return null;
      }
      saveAdminToken(input.trim());
      return input.trim();
    });
  }
  return tokenPrompt;
}

async function apiFetch(path: string, init: RequestInit = {}, allowPrompt = true): Promise<Response> {
  const headers = new Headers(init.headers);
  const token = readAdminToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  const response = await fetch(path, { ...init, headers });
  if (response.status === 401 && allowPrompt && (await promptAdminToken())) {
    return apiFetch(path, init, false);
  }
  return response;
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await apiFetch(path, init);
  if (!response.ok) {
    let message = '';
    try {
      message = (await response.json())?.error || '';
    } catch {
      // Non-JSON error body.
    }
    throw new Error(message || `HTTP error! status: ${response.status}`);
  }
  return response.json();
}

// Fetch all tenants
export async function fetchAllTenants(): Promise<TenantRecord[]> {
  const data = await requestJSON<unknown>('/api/tenants');
  return Array.isArray(data) ? data : [];
}

// Create or update the bill identified by (roomNumber, date)
export function createOrUpdateTenant(tenant: Partial<TenantRecord>): Promise<{ id: string }> {
  return requestJSON('/api/tenants', { method: 'POST', body: JSON.stringify(tenant) });
}

// Update a bill; omitted fields keep their stored values
export function updateTenant(id: string, tenant: Partial<TenantRecord>): Promise<{ id: string, message: string }> {
  return requestJSON(`/api/tenants/${encodeURIComponent(id)}`, { method: 'PUT', body: JSON.stringify(tenant) });
}

// Add a received payment to a bill (server caps it at the bill total)
export function registerPayment(id: string, amount: number): Promise<TenantRecord> {
  return requestJSON(`/api/tenants/${encodeURIComponent(id)}/payments`, { method: 'POST', body: JSON.stringify({ amount }) });
}

// One collection for a room: applied to its unpaid bills from the oldest due month on
export function registerRoomPayment(roomNumber: string, amount: number): Promise<RoomPaymentResult> {
  return requestJSON(`/api/rooms/${encodeURIComponent(roomNumber)}/payments`, { method: 'POST', body: JSON.stringify({ amount }) });
}

// Create the month's bill for every room that does not have one yet
export function generateMonthlyBills(month: string): Promise<GenerateBillsResult> {
  return requestJSON('/api/bills/generate', { method: 'POST', body: JSON.stringify({ month }) });
}

// Fetch tenants by room number
export async function fetchTenantsByRoom(roomNumber: string): Promise<TenantRecord[]> {
  const data = await requestJSON<unknown>(`/api/tenants/room/${encodeURIComponent(roomNumber)}`);
  return Array.isArray(data) ? data : [];
}

export function deleteTenantByRoom(roomNumber: string): Promise<{ message: string; deletedRows: number; roomNumber: string }> {
  return requestJSON(`/api/tenants/room/${encodeURIComponent(roomNumber)}`, { method: 'DELETE' });
}

export function deleteTenantById(id: string): Promise<{ message: string; deletedRows: number; id: string }> {
  return requestJSON(`/api/tenants/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

// Fetch income summary
export function fetchIncomeSummary(date?: string): Promise<{
  totalReceivable: number;
  totalReceived: number;
  totalUtilityIncome: number;
  outstandingBalance: number;
  dateFilter: string;
}> {
  const url = date ? `/api/income-summary?date=${encodeURIComponent(date)}` : '/api/income-summary';
  return requestJSON(url);
}

export function importExcelTenants(): Promise<ExcelImportSummary> {
  return requestJSON('/api/excel/import', { method: 'POST' });
}
