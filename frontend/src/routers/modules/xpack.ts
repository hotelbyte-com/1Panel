import { Layout } from '@/routers/constant';

const xpackRouter = {
    sort: 11,
    path: '/xpack',
    name: 'Xpack-Menu',
    component: Layout,
    redirect: '/xpack/node/dashboard',
    meta: {
        title: 'xpack.menu',
        icon: 'p-briefcase',
        adminOnly: true,
    },
    children: [
        {
            path: '/xpack/node/dashboard',
            name: 'NodeDashboard',
            component: () => import('@/views/xpack/node/index.vue'),
            meta: {
                title: 'xpack.node.nodeManagement',
                activeMenu: '/xpack/node/dashboard',
                adminOnly: true,
            },
        },
        {
            path: '/xpack/node',
            name: 'Node',
            component: () => import('@/views/xpack/node/index.vue'),
            hidden: true,
            meta: {
                title: 'xpack.node.nodeItem',
                activeMenu: '/xpack/node/dashboard',
                adminOnly: true,
            },
        },
    ],
};

export default xpackRouter;
