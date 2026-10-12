export interface JobBrowserActionResult { status: string; message?: string }

interface BrowserBridge {
  Show51JobPage: () => Promise<JobBrowserActionResult>
  Check51JobLogin: () => Promise<JobBrowserActionResult>
  Minimize51JobBrowser: () => Promise<JobBrowserActionResult>
}

interface RecoveryOptions {
  onWaiting?: (message: string) => void
  isActive?: () => boolean
}

export async function runWith51JobBrowser<T extends JobBrowserActionResult>(
  app: BrowserBridge,
  action: () => Promise<T>,
  options: RecoveryOptions = {}
): Promise<T> {
  const first = await action()
  // needs_login 只能由脚本在连接/页面检查阶段返回，尚未点击发布或发送按钮。
  // 对 uncertain/error 一律不重放，避免重复发布、扣点或发送消息。
  if (first.status !== 'needs_login') return first

  let lastMessage = ''
  const report = (message: string) => {
    if (message !== lastMessage) options.onWaiting?.(message)
    lastMessage = message
  }
  report('51job 浏览器未就绪，正在打开登录窗口…')
  const shown = await app.Show51JobPage()
  if (shown.status !== 'ready') throw new Error(shown.message || '无法打开 51job 企业浏览器')

  const deadline = Date.now() + 180_000
  while (Date.now() < deadline) {
    if (options.isActive && !options.isActive()) throw new Error('操作已取消，未继续访问 51job')
    const login = await app.Check51JobLogin()
    if (login.status === 'ready') {
      const minimized = await app.Minimize51JobBrowser()
      if (minimized.status !== 'ready') report(minimized.message || '51job 已登录，浏览器暂未最小化')
      report('51job 已登录，正在继续刚才的操作…')
      return action()
    }
    if (login.status === 'error') throw new Error(login.message || '无法确认 51job 登录状态')
    report('请在已打开的 51job 浏览器中登录；成功后会自动继续…')
    await new Promise(resolve => setTimeout(resolve, 3000))
  }
  throw new Error('等待 51job 登录超时，请登录后重新点击')
}
