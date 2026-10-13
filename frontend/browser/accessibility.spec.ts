import AxeBuilder from '@axe-core/playwright'
import { test, expect } from './fixtures'

const screens = [
  ['/login', 'С возвращением'],
  ['/', 'Обзор подготовки'],
  ['/topics', 'Карта знаний'],
  ['/topics/goroutines', 'Горутины и планировщик'],
  ['/track', 'Трек подготовки'],
  ['/bookmarks', 'Закладки'],
  ['/review', 'Повторение'],
  ['/settings', 'Настройки'],
]

for (const [path, heading] of screens) {
  test(`${path} fits the viewport and passes the accessibility audit`, async ({ page }) => {
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto(path)
    await expect(page.getByRole('heading', { name: heading, exact: true })).toBeVisible()
    if (path === '/topics/goroutines') await expect(page.getByText('Проверить отмену через context.')).toBeVisible()
    if (path === '/topics') await expect(page.getByText('Горутины и планировщик', { exact: true })).toBeVisible()
    await expect(page.getByRole('main')).toHaveCount(1)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    const results = await new AxeBuilder({ page }).analyze()
    expect(results.violations.map(issue => ({ id: issue.id, targets: issue.nodes.map(node => node.target), details: issue.nodes.map(node => node.failureSummary) }))).toEqual([])
    expect(errors).toEqual([])
  })
}

test('skip link moves keyboard focus to content and navigation focuses the new page', async ({ page }) => {
  await page.goto('/track')
  await expect(page.getByRole('heading', { name: 'Трек подготовки' })).toBeVisible()
  await page.getByRole('link', { name: 'Перейти к содержимому' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.locator('#page-content')).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(page.getByRole('textbox', { name: 'Поиск по темам' })).toBeFocused()
  expect(await page.getByRole('textbox', { name: 'Поиск по темам' }).evaluate(el => getComputedStyle(el).outlineStyle)).not.toBe('none')
  await page.getByRole('link', { name: 'Настройки', exact: true }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('heading', { name: 'Настройки', exact: true })).toBeVisible()
  await expect(page.locator('#page-content')).toBeFocused()
})

test('canceling note deletion returns focus to the delete button', async ({ page }) => {
  await page.goto('/topics/goroutines')
  const trigger = page.getByRole('button', { name: 'Удалить', exact: true })
  await trigger.click()
  const dialog = page.getByRole('alertdialog', { name: 'Удалить заметку?' })
  await expect(dialog.getByRole('button', { name: 'Да, удалить' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(trigger).toBeFocused()
})

test('reduced motion disables smooth scrolling', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/track')
  await expect(page.getByRole('heading', { name: 'Трек подготовки' })).toBeVisible()
  expect(await page.evaluate(() => getComputedStyle(document.documentElement).scrollBehavior)).toBe('auto')
})

test('top navigation does not scroll the page when moving focus to content', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/track')
  await expect(page.getByRole('heading', { name: 'Трек подготовки' })).toBeVisible()
  await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'instant' }))
  await page.getByRole('link', { name: 'Настройки', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Настройки', exact: true })).toBeVisible()
  await expect(page.locator('#page-content')).toBeFocused()
  expect(await page.evaluate(() => window.scrollY)).toBe(0)
})

test('status filter works with the keyboard and Escape preserves the selection', async ({ page }) => {
  await page.goto('/track')
  const status = page.getByRole('combobox', { name: 'Статус', exact: true })
  await status.focus()
  await page.keyboard.press('Space')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  await expect(status).toHaveValue('not_started')
  await expect(page).toHaveURL(/status=not_started/)
  await status.focus()
  await page.keyboard.press('Space')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Escape')
  await expect(status).toHaveValue('not_started')
  await expect(status).toBeFocused()
})

test('mobile catalog filters keep category and difficulty when used together', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 800 })
  await page.goto('/topics')
  const category = page.getByRole('combobox', { name: 'Раздел', exact: true })
  await expect(category).toBeVisible()
  await category.selectOption('concurrency')
  await page.getByRole('combobox', { name: 'Сложность', exact: true }).selectOption('advanced')
  await expect(category).toHaveValue('concurrency')
  await expect(page).toHaveURL(/category=concurrency/)
  await expect(page).toHaveURL(/difficulty=advanced/)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
