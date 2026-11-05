import { createRouter, createWebHashHistory } from 'vue-router'

// 路由配置
const routes = [
  {
    path: '/',
    name: 'RecruitmentDashboard',
    component: () => import('../views/RecruitmentDashboardView.vue'),
    meta: { title: '招聘概览' }
  },
  {
    path: '/jobs',
    name: 'Projects',
    component: () => import('../views/RecruitmentJobsView.vue'),
    meta: { title: '职位管理' }
  },
  {
    path: '/jobs/manage',
    name: 'ProjectsManage',
    component: () => import('../views/ProjectListView.vue'),
    meta: { title: '岗位配置' }
  },
  {
    path: '/talent',
    name: 'TalentPool',
    component: () => import('../views/TalentPoolView.vue'),
    meta: { title: '简历人才库' }
  },
  {
    path: '/interviews',
    name: 'RecruitmentInterviews',
    component: () => import('../views/RecruitmentInterviewsView.vue'),
    meta: { title: '面试安排' }
  },
  {
    path: '/pipeline',
    name: 'RecruitmentPipeline',
    component: () => import('../views/RecruitmentPipelineView.vue'),
    meta: { title: '候选人跟进' }
  },
  {
    path: '/project/:id',
    name: 'ProjectDetail',
    component: () => import('../views/HomeView.vue'),
    meta: { title: '项目详情' }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/SettingsView.vue'),
    meta: { title: '设置' }
  }
]

// 创建路由器
const router = createRouter({
  // Wails serves local assets without an SPA history fallback. Keep routes in
  // the fragment so refreshing a project or settings page still loads index.html.
  history: createWebHashHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  document.title = `${to.meta.title || 'Tellgen'} - Tellgen`
  next()
})

export default router
