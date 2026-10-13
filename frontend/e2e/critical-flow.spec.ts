import { expect, test } from '@playwright/test'
import process from 'node:process'

// The isolated smoke stack supplies credentials; no production account is used.
test('login, goroutines, progress, note and bookmark survive reload', async ({ page }) => {
  if (!process.env.E2E_LOGIN || !process.env.E2E_PASSWORD) throw new Error('Set E2E_LOGIN and E2E_PASSWORD for an isolated stack')
  const browserErrors: string[] = []
  page.on('pageerror', (error) => browserErrors.push(error.message))
  await page.goto('/login')
  await page.getByLabel('Логин').fill(process.env.E2E_LOGIN)
  await page.getByLabel('Пароль', { exact: true }).fill(process.env.E2E_PASSWORD)
  await page.getByRole('button', { name: 'Войти', exact: true }).click()
  await page.getByRole('link', { name: 'Карта знаний', exact: true }).click()
  await page.getByLabel('Поиск по темам').fill('goroutines')
  await page.getByLabel('Поиск по темам').press('Enter')
  await page.locator('a[href="/topics/goroutines"]').first().click()
  await expect(page).toHaveURL(/\/topics\/goroutines$/)
  await page.getByRole('combobox', { name: 'Статус', exact: true }).selectOption('mastered')
  await page.getByRole('button', { name: 'Сохранить прогресс' }).click()
  await expect(page.getByRole('button', { name: 'Сохранить прогресс' })).toBeEnabled()
  const bookmark = page.getByRole('button', { name: /в закладки|из закладок/ })
  if (await bookmark.getAttribute('aria-pressed') === 'false') await bookmark.click()
  await expect(bookmark).toHaveAttribute('aria-pressed', 'true')
  const note = 'E2E: cancellation and ownership survive reload'
  await page.getByLabel('Новая заметка').fill(note)
  await page.getByRole('button', { name: 'Добавить заметку' }).click()
  await expect(page.getByText(note, { exact: true })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Статус', exact: true })).toHaveValue('mastered')
  await expect(bookmark).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByText(note, { exact: true })).toBeVisible()
  expect(browserErrors).toEqual([])
})
