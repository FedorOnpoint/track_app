import { defineConfig } from '@playwright/test'
import process from 'node:process'

export default defineConfig({
  testDir: './e2e',
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://127.0.0.1:4180',
    channel: process.env.PLAYWRIGHT_CHANNEL,
    trace: 'retain-on-failure',
  },
})
