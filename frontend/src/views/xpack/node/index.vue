<template>
    <div>
        <LayoutContent :title="$t('xpack.node.nodeManagement')" v-loading="loading">
            <template #toolbar>
                <el-button type="primary" icon="Plus" @click="openCreate">{{ $t('xpack.node.addNode') }}</el-button>
                <el-button icon="Refresh" :disabled="selectedIDs.length === 0" @click="syncSelected">
                    {{ $t('commons.button.sync') }}
                </el-button>
                <el-button icon="Upload" :disabled="selectedIDs.length === 0" @click="upgradeSelected">
                    {{ $t('commons.button.upgrade') }}
                </el-button>
            </template>
            <div class="node-stats">
                <div class="node-stat">
                    <span>{{ $t('commons.table.total') }}</span>
                    <strong>{{ dashboard.total }}</strong>
                </div>
                <div class="node-stat">
                    <span>{{ $t('commons.status.normal') }}</span>
                    <strong>{{ dashboard.healthy }}</strong>
                </div>
                <div class="node-stat">
                    <span>{{ $t('xpack.node.nodeUnhealthy') }}</span>
                    <strong>{{ dashboard.unhealthy + dashboard.offline }}</strong>
                </div>
                <div class="node-stat">
                    <span>{{ $t('commons.button.sync') }}</span>
                    <strong>{{ dashboard.syncing }}</strong>
                </div>
            </div>
            <ComplexTable :data="nodes" @selection-change="selectionChange">
                <el-table-column type="selection" width="45" :selectable="(row) => row.name !== 'local'" />
                <el-table-column :label="$t('commons.table.name')" min-width="160">
                    <template #default="{ row }">
                        <el-link type="primary" @click="switchNode(row)">{{ displayNode(row) }}</el-link>
                        <el-tag v-if="row.name === 'local'" class="ml-2" size="small">Master</el-tag>
                    </template>
                </el-table-column>
                <el-table-column prop="addr" :label="$t('home.ip')" min-width="150" />
                <el-table-column prop="groupBelong" :label="$t('commons.table.group')" min-width="120" />
                <el-table-column :label="$t('commons.table.status')" width="130">
                    <template #default="{ row }">
                        <el-tag :type="row.status === 'Healthy' ? 'success' : 'warning'">{{ row.status }}</el-tag>
                    </template>
                </el-table-column>
                <el-table-column prop="version" :label="$t('setting.version')" min-width="120" />
                <el-table-column :label="'CPU / Memory'" min-width="160">
                    <template #default="{ row }">
                        {{ formatPercent(row.cpuUsedPercent) }} / {{ formatPercent(row.memoryUsedPercent) }}
                    </template>
                </el-table-column>
                <fu-table-operations
                    width="260"
                    :buttons="buttons"
                    :ellipsis="10"
                    :label="$t('commons.table.operate')"
                    fixed="right"
                />
            </ComplexTable>
        </LayoutContent>

        <el-drawer v-model="drawerVisible" :title="form.id ? $t('commons.button.edit') : $t('xpack.node.addNode')" size="520px">
            <el-form ref="formRef" :model="form" label-position="top">
                <el-form-item :label="$t('commons.table.name')" required>
                    <el-input v-model="form.name" :disabled="form.name === 'local'" />
                </el-form-item>
                <el-form-item :label="$t('home.ip')" required>
                    <el-input v-model="form.addr" />
                </el-form-item>
                <el-form-item :label="$t('xpack.node.nodePort')">
                    <el-input-number v-model="form.agentPort" :min="1" :max="65535" class="w-full" />
                </el-form-item>
                <el-form-item label="SSH">
                    <div class="node-inline">
                        <el-input v-model="form.sshUser" placeholder="root" />
                        <el-input-number v-model="form.sshPort" :min="1" :max="65535" />
                    </div>
                </el-form-item>
                <el-form-item :label="$t('commons.login.loginPassword')">
                    <el-input v-model="form.password" type="password" show-password />
                </el-form-item>
                <el-form-item :label="$t('commons.table.description')">
                    <el-input v-model="form.description" type="textarea" />
                </el-form-item>
                <el-form-item>
                    <el-checkbox v-model="form.configure">{{ $t('xpack.node.checkConnInfo') }}</el-checkbox>
                    <el-checkbox v-model="form.isAutoUpgrade">{{ $t('xpack.node.nodeUpgrade') }}</el-checkbox>
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="drawerVisible = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button @click="checkCurrent">{{ $t('xpack.node.nodeCheck') }}</el-button>
                <el-button type="primary" :loading="submitLoading" @click="submit">{{ $t('commons.button.confirm') }}</el-button>
            </template>
        </el-drawer>
    </div>
</template>

<script setup lang="ts">
import { Setting } from '@/api/interface/setting';
import {
    checkNode,
    createNode,
    deleteNode,
    listNodeOptions,
    syncNodes,
    updateNode,
    updateNodeFavorite,
    upgradeNodes,
} from '@/api/modules/setting';
import { useGlobalStore } from '@/composables/useGlobalStore';
import i18n from '@/lang';
import { MsgSuccess } from '@/utils/message';
import { onMounted, reactive, ref } from 'vue';

const { globalStore, currentNode, currentNodeAddr } = useGlobalStore();
const loading = ref(false);
const submitLoading = ref(false);
const drawerVisible = ref(false);
const nodes = ref<Setting.NodeItem[]>([]);
const selectedIDs = ref<number[]>([]);
const dashboard = reactive<Setting.NodeDashboard>({
    total: 0,
    healthy: 0,
    offline: 0,
    unhealthy: 0,
    upgrading: 0,
    syncing: 0,
    nodes: [],
});

const defaultForm = (): Setting.NodeUpdate => ({
    id: 0,
    name: '',
    addr: '',
    agentPort: 9999,
    sshPort: 22,
    sshUser: 'root',
    authMode: 'password',
    password: '',
    privateKey: '',
    passPhrase: '',
    description: '',
    configure: true,
    isAutoUpgrade: false,
});
const form = reactive<Setting.NodeUpdate>(defaultForm());

const buttons = [
    {
        label: i18n.global.t('commons.button.edit'),
        click: (row: Setting.NodeItem) => openEdit(row),
        disabled: (row: Setting.NodeItem) => row.name === 'local',
    },
    {
        label: i18n.global.t('website.favorite'),
        click: async (row: Setting.NodeItem) => {
            await updateNodeFavorite(row.id, !row.isFavorite);
            await search();
        },
        disabled: (row: Setting.NodeItem) => row.name === 'local',
    },
    {
        label: i18n.global.t('commons.button.delete'),
        click: async (row: Setting.NodeItem) => {
            await deleteNode(row.id);
            MsgSuccess(i18n.global.t('commons.msg.deleteSuccess'));
            await search();
        },
        disabled: (row: Setting.NodeItem) => row.name === 'local',
    },
];

const search = async () => {
    loading.value = true;
    try {
        const res = await listNodeOptions('all');
        const list = res.data || [];
        nodes.value = list;
        Object.assign(dashboard, {
            total: list.length,
            healthy: list.filter((item) => item.status === 'Healthy').length,
            offline: list.filter((item) => item.status === 'Offline').length,
            unhealthy: list.filter((item) => !['Healthy', 'Offline', 'Upgrading', 'Syncing'].includes(item.status)).length,
            upgrading: list.filter((item) => item.status === 'Upgrading').length,
            syncing: list.filter((item) => item.status === 'Syncing').length,
            nodes: list,
        });
    } finally {
        loading.value = false;
    }
};

const openCreate = () => {
    Object.assign(form, defaultForm());
    drawerVisible.value = true;
};

const openEdit = (row: Setting.NodeItem) => {
    Object.assign(form, defaultForm(), {
        id: row.id,
        name: row.name,
        addr: row.addr,
        description: row.description || '',
        isAutoUpgrade: row.isAutoUpgrade || false,
    });
    drawerVisible.value = true;
};

const submit = async () => {
    submitLoading.value = true;
    try {
        if (form.id) {
            await updateNode(form);
        } else {
            await createNode(form);
        }
        MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
        drawerVisible.value = false;
        await search();
    } finally {
        submitLoading.value = false;
    }
};

const checkCurrent = async () => {
    await checkNode(form);
    MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
};

const selectionChange = (selection: Setting.NodeItem[]) => {
    selectedIDs.value = selection.map((item) => item.id).filter(Boolean);
};

const syncSelected = async () => {
    await syncNodes(selectedIDs.value);
    await search();
};

const upgradeSelected = async () => {
    await upgradeNodes(selectedIDs.value);
    await search();
};

const switchNode = (row: Setting.NodeItem) => {
    currentNode.value = row.name;
    currentNodeAddr.value = row.addr;
};

const displayNode = (row: Setting.NodeItem) => (row.name === 'local' ? globalStore.getMasterAlias() : row.name);
const formatPercent = (value?: number) => (value == undefined ? '-' : `${value.toFixed(1)}%`);

onMounted(search);
</script>

<style scoped lang="scss">
.node-stats {
    display: grid;
    grid-template-columns: repeat(4, minmax(120px, 1fr));
    gap: 12px;
    margin-bottom: 12px;
}

.node-stat {
    border: 1px solid var(--el-border-color-light);
    border-radius: 6px;
    padding: 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
}

.node-inline {
    display: grid;
    grid-template-columns: 1fr 150px;
    gap: 8px;
    width: 100%;
}
</style>
