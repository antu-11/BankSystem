// ── API Client ─────────────────────────────────────────────────
// All requests include credentials: 'include' so the HttpOnly
// JWT cookie is automatically sent to the Go backend.

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  });

  const data = await res.json().catch(() => null);

  if (!res.ok) {
    const message = data?.error || `Request failed (${res.status})`;
    const err = new Error(message);
    err.status = res.status;
    throw err;
  }

  return data;
}

// ── Auth ──────────────────────────────────────────────────────────
export const authAPI = {
  register: (body) =>
    request('/api/v1/auth/register', { method: 'POST', body: JSON.stringify(body) }),

  login: (body) =>
    request('/api/v1/auth/login', { method: 'POST', body: JSON.stringify(body) }),

  logout: () =>
    request('/api/v1/auth/logout', { method: 'POST' }),
};

// ── Accounts ─────────────────────────────────────────────────────
export const accountsAPI = {
  getAll: () =>
    request('/api/v1/accounts'),

  getBalance: (accountId) =>
    request(`/api/v1/accounts/${accountId}/balance`),

  getHistory: (accountId, page = 1, perPage = 20) =>
    request(`/api/v1/accounts/${accountId}/history?page=${page}&per_page=${perPage}`),
};

// ── Transfers ────────────────────────────────────────────────────
export const transfersAPI = {
  send: (body) =>
    request('/api/v1/transfers', { method: 'POST', body: JSON.stringify(body) }),
};

// ── System Funding ───────────────────────────────────────────────
export const fundingAPI = {
  fund: (body) =>
    request('/api/v1/transactions/system/fund', { method: 'POST', body: JSON.stringify(body) }),
};
