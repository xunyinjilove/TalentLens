/**
 * 51job [职位管理] 岗位与城市信息抓取适配器
 * 连接 Edge (port 9503)，读取 https://ehire.51job.com/Revision/job-manage 页面中
 * 所有已发布 / 维护中的职位名称、发布城市、详细地点、薪资和要求。
 */
const puppeteer = require('puppeteer-core');

async function scrape51JobManagementJobs() {
  let browser;
  try {
    browser = await puppeteer.connect({ browserURL: 'http://127.0.0.1:9503', defaultViewport: null, protocolTimeout: 15000 });
  } catch (err) {
    return {
      status: 'needs_login',
      message: '51job 企业浏览器未就绪，请先在浏览器中登录 51job 企业端',
      jobs: []
    };
  }

  try {
    const pages = await browser.pages();
    let page = pages.find(p => p.url().includes('job-manage'));

    if (!page) return {
      status: 'needs_action',
      message: '没有已打开的 51job 职位管理页面；请先在专用浏览器中打开该页面，再手动同步',
      jobs: []
    };

    // Wait a brief moment for dynamic elements if needed
    await page.waitForSelector('body', { timeout: 5000 }).catch(() => {});

    const jobs = await page.evaluate(() => {
      const results = [];
      const allElements = Array.from(document.querySelectorAll('*'));
      const cityLabels = allElements.filter(el => 
        el.children.length === 0 && (el.textContent || '').trim() === '发布城市：'
      );

      for (const labelEl of cityLabels) {
        const parent = labelEl.parentElement;
        const fullCityText = parent ? (parent.textContent || '').trim() : '';
        const cityMatch = fullCityText.match(/发布城市：\s*([^\s\r\n]+)/);
        const city = cityMatch ? cityMatch[1].trim() : '';

        // Find enclosing container (look up up to 12 levels)
        let container = labelEl;
        for (let i = 0; i < 12; i++) {
          if (!container.parentElement) break;
          const p = container.parentElement;
          if (p.tagName === 'BODY') break;
          container = p;
          if (container.classList && (
            container.classList.contains('job-list-item') || 
            container.classList.contains('el-table__row') || 
            container.classList.contains('job-item') ||
            container.classList.contains('card')
          )) {
            break;
          }
        }

        const text = container.textContent || '';
        
        // Find title
        let title = '';
        const titleLink = container.querySelector('a.job-name, a.title, .job-title, [class*="jobName"], [class*="job-name"], h3, h4');
        if (titleLink && titleLink.textContent.trim()) {
          title = titleLink.textContent.trim();
        } else {
          const lines = text.split('\n').map(s => s.trim()).filter(Boolean);
          for (const line of lines) {
            if (line.includes('经理') || line.includes('工程师') || line.includes('专员') || 
                line.includes('主管') || line.includes('总监') || line.includes('助理') || 
                line.includes('QC') || line.includes('代表') || line.includes('开发') ||
                line.includes('销售') || line.includes('顾问') || line.includes('研究员')) {
              title = line.replace(/^[0-9]+星\s*“[^”]+”\s*去提升\s*/, '').replace(/\s*已暂停.*$/, '').replace(/\s*到期下线.*$/, '').trim();
              break;
            }
          }
        }

        // Location / Edu / Exp / Salary row
        // Example: "上海-浦东新区  |  硕士 | 3年及以上 | 1.4-2万/月"
        let location = '';
        let education = '';
        let experience = '';
        let salary = '';
        
        const metaMatch = text.match(/([^\s|]+(?:\s*-\s*[^\s|]+)?)\s*\|\s*([^\s|]+)\s*\|\s*([^\s|]+)\s*\|\s*([^\s|]+(?:\/[月年]|万|千)?)/);
        if (metaMatch) {
          location = metaMatch[1].trim();
          education = metaMatch[2].trim();
          experience = metaMatch[3].trim();
          salary = metaMatch[4].trim();
        }

        let status = '招聘中';
        if (text.includes('已暂停')) status = '已暂停';
        else if (text.includes('草稿')) status = '草稿';

        if (title || city) {
          results.push({
            title: title || '未命名职位',
            city: city || location.split(/[-—]/)[0].trim() || '上海',
            location: location || city,
            education: education || '不限',
            experience: experience || '经验不限',
            salary: salary || '面议',
            status
          });
        }
      }

      return results;
    });

    return {
      status: 'success',
      count: jobs.length,
      jobs
    };
  } catch (err) {
    return {
      status: 'error',
      message: '抓取 51job [职位管理] 失败: ' + (err.message || String(err)),
      jobs: []
    };
  }
}

if (require.main === module) {
  scrape51JobManagementJobs().then(res => {
    process.stdout.write(JSON.stringify(res) + '\n');
    process.exit(res.status === 'success' ? 0 : 1);
  });
}

module.exports = { scrape51JobManagementJobs };
