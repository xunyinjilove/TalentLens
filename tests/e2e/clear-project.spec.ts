import { test, expect } from '@playwright/test';

test('清空当前项目需确认，且不影响其他项目', async ({ page }) => {
  await page.goto('/');
  const ids = await page.evaluate(async () => {
    const app = (window as any).go.main.App;
    const config = { title: '测试工程师', requirements: [], required_skills: [], experience_years: 0, education_level: '', job_description: '' };
    const first = await app.CreateProject('待清空项目', '', 1, '', '', config);
    const second = await app.CreateProject('保留项目', '', 1, '', '', config);
    const suffix = Date.now();
    await app.RegisterResumeToProject(first.id, `clear_a_${suffix}`, '候选人甲.txt', '', '.txt', 1);
    await app.RegisterResumeToProject(second.id, `clear_b_${suffix}`, '候选人乙.txt', '', '.txt', 1);
    return { first: first.id, second: second.id };
  });

  await page.reload();
  await page.locator('.project-card').filter({ hasText: '待清空项目' }).click();
  await expect(page).toHaveURL(new RegExp(`/project/${ids.first}$`));
  const toolbar = page.locator('.resume-toolbar');
  await expect(toolbar).toContainText('简历列表 1');
  await expect(toolbar.getByRole('button', { name: '分析 1 份' })).toBeVisible();
  await toolbar.getByRole('button', { name: '更多' }).click();
  await page.getByRole('menuitem', { name: '清空当前项目' }).click();
  await expect(page.getByText('将清空「待清空项目」中的 1 份候选人记录', { exact: false })).toBeVisible();
  await page.getByRole('button', { name: '取消' }).click();
  await expect(toolbar).toContainText('简历列表 1');

  await toolbar.getByRole('button', { name: '更多' }).click();
  await page.getByRole('menuitem', { name: '清空当前项目' }).click();
  await page.getByRole('button', { name: '确认清空' }).click();
  await expect(toolbar).toContainText('简历列表 0');
  await expect(toolbar.getByRole('button', { name: '已全部分析' })).toBeDisabled();

  const remaining = await page.evaluate(async id => {
    const app = (window as any).go.main.App;
    const project = await app.GetProject(id);
    return { count: project.resume_ids.length, resumes: (await app.GetProjectResumes(id) || []).length };
  }, ids.second);
  expect(remaining).toEqual({ count: 1, resumes: 1 });
});
