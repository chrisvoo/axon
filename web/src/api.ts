export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, apiKey: string, init: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      Authorization: `Bearer ${apiKey}`,
    },
  })
  if (!res.ok) {
    const t = await res.text()
    throw new ApiError(t || res.statusText, res.status)
  }
  return (await res.json()) as T
}

export function apiPostJson<T>(path: string, apiKey: string, body: Record<string, unknown>): Promise<T> {
  return request<T>(path, apiKey, { method: 'POST', body: JSON.stringify(body) })
}

export function apiGetJson<T>(path: string, apiKey: string): Promise<T> {
  return request<T>(path, apiKey, { method: 'GET' })
}
