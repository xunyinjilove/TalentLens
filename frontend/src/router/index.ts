import { createRouter, createWebHashHistory } from 'vue-router'

// 路由配置
const routes = [
  {
    path: '/',
    name: 'Projects',
    component: () => import('../views/ProjectListView.vue'),
    meta: { title: '招聘项目' }
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
