import { test, expect, Page } from '@playwright/test';
import { createServer, Server } from 'node:http';

let server: Server;
let baseURL: string;
let mode: 'success' | 'error';
let requests: any[];

test.beforeEach(async () => {
  mode = 'success';
  requests = [];
  server = createServer(async (req, res) => {
    let body = '';
    for await (const chunk of req) body += chunk;
    requests.push({ url: req.url, body: JSON.parse(body) });
    res.setHeader('Content-Type', 'application/json');
    if (mode === 'error') {
      res.writeHead(503);
      res.end(JSON.stringify({ error: { message: 'E2E simulated outage' } }));
      return;
    }
    res.end(JSON.stringify({ choices: [{ message: { content: JSON.stringify({
      candidate_name: 'E2E虚构候选人', core_match: 75, skill_match: 75,
      experience_match: 75, education_match: 80, overall_score: 75,
      summary: '自动化测试固定响应', recommendation: 'recommend',
      strengths: ['主导测试设计方案'], weaknesses: [], risks: [],
      core_criterion_checks: [{ criterion: '测试设计', level: 3, evidence: '主导测试设计方案' }],
      red_line_checks: [{ criterion: '具备测试项目经验', status: 'met', evidence: '主导测试设计方案' }],
    }) } }] }));
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  baseURL = `http://127.0.0.1:${(server.address() as any).port}/v1`;
});

test.afterEach(async () => {
  server.closeAllConnections();
  await new Promise<void>(resolve => server.close(() => resolve()));
});

async function seedProject(page: Page, configured = true) {
  await page.goto('/');
  await expect(page.getByRole('button', { name: /以(此|当前)岗位新建项目/ })).toBeVisible();
  const project = await page.evaluate(async ({ baseURL, configured }) => {
    const api = (window as any).go.main.App;
    const dir = await api.GetDataDir();
    if (!dir.replaceAll('\\', '/').includes('/.e2e-data/run-')) throw new Error('Not isolated');
    localStorage.setItem('goresume_settings', JSON.stringify({
      ai: { provider: 'custom', baseURL, apiKey: configured ? 'e2e-fake-key' : '', model: 'e2e-mock' },
      analysis: { autoStart: false },
    }));
    const project = await api.CreateProject(`E2E分析-${Date.now()}`, '测试部', 1, 'E2E', '', {
      title: '测试工程师', requirements: [], required_skills: ['测试设计'],
      experience_years: 0, education_level: '', core_criteria: ['测试设计'],
      red_lines: ['具备测试项目经验'], bonus_points: [],
    });
    const imported = await api.ImportResumesToProject(project.id, [dir + '/e2e-resume.docx']);
    if (imported !== 1) throw new Error(`Unexpected import count: ${imported}`);
    return project;
  }, { baseURL, configured });
  await page.goto(`/#/project/${project.id}`);
  await expect(page.locator('.resume-card')).toHaveCount(1);
  return project.id;
}

test('真实DOCX导入 → 模拟AI分析 → 界面完成 → 刷新持久化；红线仍待HR核实', async ({ page }) => {
  const id = await seedProject(page);
  await page.getByRole('button', { name: '开始分析', exact: true }).click();
  await expect(page.locator('.resume-card')).toHaveClass(/is-done/);
  await expect(page.locator('.resume-card')).toContainText('红线待核实');
  const resumes = await page.evaluate(id => (window as any).go.main.App.GetProjectResumes(id), id);
  expect(resumes[0].content).toContain('E2E虚构候选人');
  expect(resumes[0].score).toBe(75);
  expect(resumes[0].analysis.red_line_status).toBe('pending');
  expect(requests).toHaveLength(1);
  expect(requests[0].url).toBe('/v1/chat/completions');
  expect(JSON.stringify(requests[0].body)).toContain('主导测试设计方案');
  await page.reload();
  await expect(page.locator('.resume-card')).toHaveClass(/is-done/);
  await expect(page.locator('.resume-card .score-number')).toHaveText('75');
});

test('AI服务失败后展示错误，并允许对该简历重试', async ({ page }) => {
  test.setTimeout(45_000);
  await seedProject(page);
  mode = 'error';
  await page.getByRole('button', { name: '开始分析', exact: true }).click();
  await expect(page.locator('.resume-card')).toHaveClass(/is-error/, { timeout: 20_000 });
  mode = 'success';
  await page.locator('.resume-card .reanalyze-btn').click();
  await expect(page.locator('.resume-card')).toHaveClass(/is-done/);
});

test('未配置AI时显示配置引导，且不发送分析请求', async ({ page }) => {
  await seedProject(page, false);
  await page.getByRole('button', { name: '开始分析', exact: true }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await expect(page.locator('.resume-card')).toHaveClass(/is-pending/);
  expect(requests).toHaveLength(0);
});
