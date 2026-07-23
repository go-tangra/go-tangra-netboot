import type { RouteRecordRaw } from 'vue-router';

// The route tree mirrors cmd/server/assets/menus.yaml, which the admin
// service uses to build the navigation; keeping the two in step is what makes
// a menu entry actually resolve to a component.
const routes: RouteRecordRaw[] = [
  {
    path: '/netboot',
    name: 'Netboot',
    component: () => import('shell/app-layout'),
    redirect: '/netboot/machine',
    meta: {
      order: 2020,
      icon: 'lucide:server-cog',
      title: 'netboot.menu.netboot',
      keepAlive: true,
      authority: ['platform:admin', 'tenant:manager'],
    },
    children: [
      {
        path: 'machine',
        name: 'NetbootMachines',
        meta: {
          icon: 'lucide:hard-drive',
          title: 'netboot.menu.machines',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/machine/index.vue'),
      },
      {
        path: 'profile',
        name: 'NetbootProfiles',
        meta: {
          icon: 'lucide:file-cog',
          title: 'netboot.menu.profiles',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/profile/index.vue'),
      },
      {
        path: 'session',
        name: 'NetbootSessions',
        meta: {
          icon: 'lucide:activity',
          title: 'netboot.menu.sessions',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/session/index.vue'),
      },
      {
        path: 'dhcp',
        name: 'NetbootDhcp',
        meta: {
          icon: 'lucide:network',
          title: 'netboot.menu.dhcp',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/dhcp/index.vue'),
      },
      {
        path: 'artifact',
        name: 'NetbootArtifacts',
        meta: {
          icon: 'lucide:package',
          title: 'netboot.menu.artifacts',
          authority: ['platform:admin', 'tenant:manager'],
        },
        component: () => import('./views/artifact/index.vue'),
      },
    ],
  },
];

export default routes;
