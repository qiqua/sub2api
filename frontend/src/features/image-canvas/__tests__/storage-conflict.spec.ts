// @vitest-environment node
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { once } from 'node:events'
import { createServer } from 'vite'
import { expect, it } from 'vitest'

// jsdom has no IndexedDB. Use an isolated browser profile for native transaction locks and Blob cloning.
const chrome = [
  process.env.CHROME_BIN,
  process.env.ProgramFiles && join(process.env.ProgramFiles, 'Google/Chrome/Application/chrome.exe'),
  process.env['ProgramFiles(x86)'] && join(process.env['ProgramFiles(x86)'], 'Microsoft/Edge/Application/msedge.exe'),
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', '/usr/bin/chromium', '/usr/bin/google-chrome', '/usr/bin/chromium-browser',
].find(path => path && existsSync(path))

it.skipIf(!chrome)('enforces canvas CAS, restores pending state and isolates forked assets in native IndexedDB (set CHROME_BIN to select Chromium)', async () => {
  const profile = await mkdtemp(join(tmpdir(), 'canvas-storage-test-'))
  let finish: (result: string) => void
  const result = new Promise<string>(resolve => { finish = resolve })
  const server = await createServer({
    configFile: false, appType: 'custom', logLevel: 'error',
    optimizeDeps: { noDiscovery: true },
    server: { host: '127.0.0.1', port: 0, hmr: false },
    plugins: [{
      name: 'storage-conflict-test',
      configureServer(vite) {
        vite.middlewares.use('/__storage_result', (req, res) => {
          let message = ''
          req.setEncoding('utf8').on('data', chunk => { message += chunk })
          req.on('end', () => { res.end(); finish(message) })
        })
        vite.middlewares.use('/__storage_test', (_req, res) => {
          res.setHeader('Content-Type', 'text/html')
          res.end('<script type="module" src="/src/features/image-canvas/__tests__/storage-conflict.browser.ts"></script>')
        })
      },
    }],
  })
  try {
    await server.listen()
    const address = server.httpServer!.address()
    if (!address || typeof address === 'string') throw new Error('Expected a local HTTP port')
    const flags = ['--headless', '--disable-gpu', '--disable-dev-shm-usage', '--no-first-run', '--no-default-browser-check', `--user-data-dir=${profile}`]
    if (process.getuid?.() === 0) flags.push('--no-sandbox')
    const child = spawn(chrome!, [...flags, `http://127.0.0.1:${address.port}/__storage_test`], { stdio: 'ignore' })
    const closed = once(child, 'close')
    let timer: NodeJS.Timeout | undefined
    try {
      const timeout = new Promise<never>((_, reject) => { timer = setTimeout(() => reject(new Error('Browser storage test did not finish')), 40_000) })
      expect(await Promise.race([result, timeout, closed.then(([code]) => `Browser exited (${code})`)])).toBe('pass')
    } finally {
      clearTimeout(timer)
      child.kill()
      await closed
    }
  } finally {
    await server.close()
    await rm(profile, { recursive: true, force: true, maxRetries: 10, retryDelay: 150 })
  }
}, 55_000)
