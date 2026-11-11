import { expect, test } from '@playwright/test'

test('人才库通过 backend 桥接读取本地数据', async ({ page }) => {
  await page.goto('/#/talent')
  await expect(page.getByRole('heading', { name: '简历人才库' })).toBeVisible()
  await expect.poll(() => page.evaluate(() => typeof (window as any).go?.backend?.App?.GetDataDir)).toBe('function')

  const dataDir = await page.evaluate(() => (window as any).go.backend.App.GetDataDir())
  expect(dataDir.replaceAll('\\', '/')).toContain('/.e2e-data/run-')
  await expect(page.getByText('当前为浏览器界面预览', { exact: false })).toHaveCount(0)
})
