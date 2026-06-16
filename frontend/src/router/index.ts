import { createRouter, createWebHistory } from 'vue-router'
import { authGuard } from './guards'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/users/sign_in',
      name: 'sign-in',
      component: () => import('@/pages/SignInPage.vue'),
      meta: { public: true, layout: 'auth' },
    },
    {
      path: '/users/sign_out',
      name: 'sign-out',
      component: () => import('@/pages/SignOutPage.vue'),
      meta: { public: true, layout: 'auth' },
    },
    {
      path: '/admin',
      component: () => import('@/layouts/AdminLayout.vue'),
      meta: { admin: true },
      children: [
        { path: '', name: 'admin', component: () => import('@/pages/AdminDashboard.vue') },
        { path: 'users', name: 'admin-users', component: () => import('@/pages/AdminUsersPage.vue') },
        { path: 'users/new', name: 'admin-user-new', component: () => import('@/pages/AdminUserFormPage.vue') },
        { path: 'users/:id', name: 'admin-user-edit', component: () => import('@/pages/AdminUserFormPage.vue') },
        {
          path: 'system',
          name: 'admin-system',
          component: () => import('@/pages/AdminSystemSettingsPage.vue'),
          meta: { root: true },
        },
      ],
    },
    {
      path: '/profile',
      name: 'profile',
      component: () => import('@/pages/ProfilePage.vue'),
    },
    {
      path: '/groups',
      name: 'groups',
      component: () => import('@/pages/GroupsPage.vue'),
    },
    {
      path: '/groups/:id/-/members',
      name: 'group-members',
      component: () => import('@/pages/GroupMembersPage.vue'),
    },
    { path: '/', redirect: '/projects' },
    {
      path: '/projects',
      name: 'projects',
      component: () => import('@/pages/ProjectsPage.vue'),
    },
    {
      path: '/projects/new',
      name: 'project-new',
      component: () => import('@/pages/ProjectNewPage.vue'),
    },
    {
      path: '/projects/:id',
      component: () => import('@/layouts/ProjectLayout.vue'),
      children: [
        { path: '', name: 'project-overview', component: () => import('@/pages/project/OverviewPage.vue') },
        { path: 'chat', name: 'project-chat', component: () => import('@/pages/project/ChatPage.vue') },
        { path: 'runs', name: 'project-runs', component: () => import('@/pages/project/RunsPage.vue') },
        { path: 'runs/:runId', name: 'project-run-detail', component: () => import('@/pages/project/RunDetailPage.vue') },
        { path: 'repository', name: 'project-repository', component: () => import('@/pages/project/RepositoryPage.vue') },
        { path: '-/settings/general', name: 'project-settings', component: () => import('@/pages/project/SettingsGeneralPage.vue') },
        { path: '-/settings/repositories', name: 'project-repositories-settings', component: () => import('@/pages/project/SettingsRepositoriesPage.vue') },
        { path: '-/settings/members', name: 'project-members', component: () => import('@/pages/project/SettingsMembersPage.vue') },
        { path: '-/settings/integrations', name: 'project-integrations', component: () => import('@/pages/project/SettingsIntegrationsPage.vue') },
      ],
    },
    { path: '/403', name: 'forbidden', component: () => import('@/pages/ForbiddenPage.vue'), meta: { public: true } },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/pages/NotFoundPage.vue') },
  ],
})

router.beforeEach(authGuard)

export default router
