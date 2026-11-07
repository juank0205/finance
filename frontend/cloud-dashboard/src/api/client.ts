import type { Api, Category, NetWorth, Recurring, Summary, Transaction } from './types.ts'

const baseUrl = import.meta.env.VITE_API_URL

// Without VITE_API_URL the dashboard runs on generated demo data, and says so in the header.
export const usingMockData = !baseUrl

export const api: Promise<Api> = baseUrl
  ? Promise.resolve(httpApi(baseUrl))
  : import('./mock.ts').then((m) => m.mockApi)

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

function httpApi(base: string): Api {
  // TODO(phase 5): send the Cognito access token as a Bearer header.
  async function call<T>(method: Method, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
    const res = await fetch(new URL(path, base), {
      method,
      signal,
      headers: { Accept: 'application/json', ...(body ? { 'Content-Type': 'application/json' } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    })
    if (!res.ok) {
      // cloud-api answers errors as {"error": "..."}; show that message when there is one.
      const detail = await res
        .json()
        .then((j: { error?: string }) => j.error)
        .catch(() => undefined)
      throw new Error(detail ?? `${method} ${path}: HTTP ${res.status}`)
    }
    return res.status === 204 ? (undefined as T) : ((await res.json()) as T)
  }

  const q = (month: string) => `?month=${encodeURIComponent(month)}`
  const tx = (id: string) => `/v1/transactions/${encodeURIComponent(id)}`
  const rule = (id: string) => `/v1/recurring/${encodeURIComponent(id)}`

  return {
    summary: (month, signal) => call<Summary>('GET', `/v1/summary${q(month)}`, undefined, signal),
    transactions: (month, signal) =>
      call<{ items: Transaction[] }>('GET', `/v1/transactions${q(month)}`, undefined, signal).then((r) => r.items),
    createTransaction: (input) => call<Transaction>('POST', '/v1/transactions', input),
    updateTransaction: (id, input) => call<Transaction>('PUT', tx(id), input),
    deleteTransaction: (id) => call<void>('DELETE', tx(id)),
    recurring: (signal) =>
      call<{ items: Recurring[] }>('GET', '/v1/recurring', undefined, signal).then((r) => r.items),
    createRecurring: (input) => call<Recurring>('POST', '/v1/recurring', input),
    updateRecurring: (id, input) => call<Recurring>('PUT', rule(id), input),
    deleteRecurring: (id) => call<void>('DELETE', rule(id)),
    netWorth: (signal) => call<NetWorth>('GET', '/v1/net-worth', undefined, signal),
    categories: (signal) =>
      call<{ items: Category[] }>('GET', '/v1/categories', undefined, signal).then((r) => r.items),
    saveCategories: (items) => call<{ items: Category[] }>('PUT', '/v1/categories', { items }).then((r) => r.items),
  }
}
