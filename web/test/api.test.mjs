import assert from 'node:assert/strict'
import test from 'node:test'
import { ApiError, get, post } from '../src/lib/api.ts'

test('GET returns JSON and forwards the cookie policy and abort signal', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => new globalThis.Response('{"status":"ready"}'))
  const controller = new globalThis.AbortController()
  assert.deepEqual(await get('/status', controller.signal), { status: 'ready' })
  const [url, options] = fetch.mock.calls[0].arguments
  assert.equal(url, '/api/v1/status')
  assert.equal(options.method, 'GET')
  assert.equal(options.credentials, 'same-origin')
  assert.equal(options.signal, controller.signal)
})

test('POST sends JSON and accepts an empty 204 response', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => new globalThis.Response(null, { status: 204 }))
  assert.equal(await post('/example', { content: 'hello' }), undefined)
  const options = fetch.mock.calls[0].arguments[1]
  assert.equal(options.method, 'POST')
  assert.equal(options.headers['Content-Type'], 'application/json')
  assert.equal(options.body, '{"content":"hello"}')
})

test('API errors preserve the HTTP status and application message', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new globalThis.Response(
    '{"error":{"message":"Please sign in to continue."}}', { status: 401 },
  ))
  await assert.rejects(get('/auth/me'), (error) =>
    error instanceof ApiError && error.status === 401 && error.message === 'Please sign in to continue.',
  )
})

test('non-JSON errors have a readable fallback', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new globalThis.Response('Bad Gateway', { status: 502 }))
  await assert.rejects(get('/status'), (error) =>
    error instanceof ApiError && error.status === 502 && error.message === 'Unable to reach MediaShare.',
  )
})

test('network failures have a readable message', async (t) => {
  const cause = new TypeError('Failed to fetch')
  t.mock.method(globalThis, 'fetch', async () => { throw cause })
  await assert.rejects(get('/status'), (error) =>
    error.message === 'Unable to reach MediaShare. Please try again.' && error.cause === cause,
  )
})

test('cancellation remains distinguishable from a network failure', async (t) => {
  const controller = new globalThis.AbortController()
  controller.abort()
  t.mock.method(globalThis, 'fetch', async () => { throw controller.signal.reason })
  await assert.rejects(get('/status', controller.signal), (error) => error === controller.signal.reason)
})

test('multipart uploads preserve FormData and let the browser set its boundary', async (t) => {
  const form = new globalThis.FormData()
  form.append('file', new globalThis.Blob(['image data'], { type: 'image/png' }), 'photo.png')
  const fetch = t.mock.method(globalThis, 'fetch', async () => new globalThis.Response('{"id":"uploaded"}', { status: 201 }))
  assert.deepEqual(await post('/media', form), { id: 'uploaded' })
  const options = fetch.mock.calls[0].arguments[1]
  assert.equal(options.body, form)
  assert.equal(options.headers['Content-Type'], undefined)
  assert.equal(options.credentials, 'same-origin')
})
