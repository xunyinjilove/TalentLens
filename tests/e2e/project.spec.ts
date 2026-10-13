import { test, expect, Page } from '@playwright/test';

async function openApp(page: Page) {
  await page.goto('/');
  await expect(page.getByRole('button', { name: /以(此|当前)岗位新建项目/ })).toBeVisible();
  const dataDir = await page.evaluate(() => (window as any).go.main.App.GetDataDir());
  expect(dataDir.replaceAll('\\', '/')).toContain('/.e2e-data/run-');
}

test('真实后端：新建项目继承岗位，刷新后仍存在，删除后不再出现', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => {
    // 过滤 Wails 2.15 官方 devserver WebSocket 调试层特有的 Svelte overlay 异常（生产环境打包版不注入此调试层）
    if (error.message.includes("reading 'nodes'") || error.stack?.includes('/wails/ipc.js')) {
      return;
    }
    errors.push(error.message);
  });
  await openApp(page);
  await page.getByRole('button', { name: /微调要求|查看\/编辑岗位要求/ }).click();
  const job = page.getByRole('dialog', { name: '岗位要求', exact: true });
  await job.getByPlaceholder('岗位名称', { exact: true }).fill('E2E 测试工程师');
  await job.getByPlaceholder('岗位名称', { exact: true }).press('Tab');
  await job.getByRole('button', { name: '关闭岗位要求' }).click();
  await page.getByRole('button', { name: /以(此|当前)岗位新建项目/ }).click();
  const dialog = page.getByRole('dialog', { name: '新建招聘项目' });
  const name = `E2E招聘项目-${Date.now()}`;
  await dialog.getByPlaceholder('如：Go高级开发 - 2月批次').fill(name);
  await dialog.getByRole('button', { name: '确认', exact: true }).click();
  await page.locator('.project-card').filter({ hasText: name }).click();
  await expect(page).toHaveURL(/\/project\//);
  const id = page.url().split('/').pop()!;
  const project = await page.evaluate(id => (window as any).go.main.App.GetProject(id), id);
  expect(project.name).toBe(name);
  expect(project.job_config.title).toBe('E2E 测试工程师');
  await page.reload();
  await expect(page.locator('.title-text')).toContainText(name);
  await page.goto('/');
  const card = page.locator('.project-card').filter({ hasText: name });
  await expect(card).toBeVisible();
  await card.getByTitle('删除项目').click();
  await page.getByRole('button', { name: '确定', exact: true }).click();
  await expect(card).toHaveCount(0);
  await page.reload();
  await expect(page.locator('.project-card').filter({ hasText: name })).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('岗位弹窗可关闭，空白项目名不能提交', async ({ page }) => {
  await openApp(page);
  await page.getByRole('button', { name: /微调要求|查看\/编辑岗位要求/ }).click();
  await expect(page.getByRole('dialog', { name: '岗位要求', exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: '岗位要求', exact: true })).toBeHidden();
  await page.getByRole('button', { name: /以(此|当前)岗位新建项目/ }).click();
  const dialog = page.getByRole('dialog', { name: '新建招聘项目' });
  await dialog.getByPlaceholder('如：Go高级开发 - 2月批次').fill('   ');
  await expect(dialog.getByRole('button', { name: '确认', exact: true })).toBeDisabled();
});
