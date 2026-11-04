export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      method,
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal,
    })
  } catch (error) {
    if (signal?.aborted) throw error
    throw new Error('Unable to reach MediaShare. Please try again.', { cause: error })
  }

  if (!response.ok) {
    const error = await response.json().catch(() => null)
    throw new ApiError(response.status, error?.error?.message ?? 'Unable to reach MediaShare.')
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  return request<T>('GET', path, undefined, signal)
}

export function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('POST', path, body)
}
