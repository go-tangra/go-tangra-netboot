<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Page } from 'shell/vben/common-ui';
import { $t } from 'shell/locales';

import { useNetbootSessionStore } from '../../stores/netboot-session.state';
import type { ProvisioningEvent, ProvisioningSession, SessionState } from '../../types';

const store = useNetbootSessionStore();

const sessions = ref<ProvisioningSession[]>([]);
const total = ref(0);
const loading = ref(false);
const errorMessage = ref('');

const page = ref(1);
const pageSize = ref(20);
const stateFilter = ref('');

const timelineOpen = ref(false);
const timeline = ref<ProvisioningEvent[]>([]);
const evidence = ref('');

async function load() {
  loading.value = true;
  errorMessage.value = '';
  try {
    const result = await store.listSessions(
      { page: page.value, pageSize: pageSize.value },
      { state: stateFilter.value || undefined },
    );
    sessions.value = result.sessions ?? [];
    total.value = result.meta?.total ?? 0;
  } catch (error) {
    errorMessage.value = (error as Error).message || $t('netboot.common.loadFailed');
  } finally {
    loading.value = false;
  }
}

async function openTimeline(session: ProvisioningSession) {
  if (!session.id) return;
  const detail = await store.getSession(session.id);
  timeline.value = detail.timeline ?? [];
  evidence.value = detail.evidence ?? '';
  timelineOpen.value = true;
}

function stateLabel(state?: SessionState) {
  return $t(`netboot.enum.sessionState.${state ?? 'SESSION_STATE_UNSPECIFIED'}`);
}

function stateClass(state?: SessionState) {
  switch (state) {
    case 'SESSION_STATE_ACTIVE':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200';
    case 'SESSION_STATE_COMPLETED':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
    case 'SESSION_STATE_FAILED':
      return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200';
  }
}

function outcomeClass(outcome?: string) {
  if (outcome === 'EVENT_OUTCOME_ERROR') return 'text-red-600';
  if (outcome === 'EVENT_OUTCOME_DENIED') return 'text-amber-600';
  return 'text-green-600';
}

onMounted(load);
</script>

<template>
  <Page :title="$t('netboot.page.session.title')">
    <div class="space-y-4">
      <div v-if="errorMessage" class="rounded border border-red-300 bg-red-50 p-3 text-red-800">
        {{ errorMessage }}
      </div>

      <div class="flex items-center gap-2">
        <select v-model="stateFilter" class="rounded border px-3 py-1.5" @change="load">
          <option value="">{{ $t('netboot.page.session.state') }}</option>
          <option value="SESSION_STATE_ACTIVE">{{ stateLabel('SESSION_STATE_ACTIVE') }}</option>
          <option value="SESSION_STATE_COMPLETED">{{ stateLabel('SESSION_STATE_COMPLETED') }}</option>
          <option value="SESSION_STATE_FAILED">{{ stateLabel('SESSION_STATE_FAILED') }}</option>
          <option value="SESSION_STATE_STALE">{{ stateLabel('SESSION_STATE_STALE') }}</option>
        </select>
        <button class="rounded border px-3 py-1.5" :disabled="loading" @click="load">
          {{ $t('netboot.common.refresh') }}
        </button>
      </div>

      <div class="overflow-x-auto rounded border">
        <table class="w-full text-left text-sm">
          <thead class="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th class="px-3 py-2">{{ $t('netboot.page.session.machine') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.session.profile') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.session.state') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.session.startedAt') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.session.endedAt') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.page.session.failurePhase') }}</th>
              <th class="px-3 py-2">{{ $t('netboot.common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="session in sessions" :key="session.id" class="border-t">
              <td class="px-3 py-2">
                <div class="font-medium">{{ session.machineName }}</div>
                <div class="font-mono text-xs text-gray-500">{{ session.machineMac }}</div>
              </td>
              <td class="px-3 py-2">{{ session.profileId }} (v{{ session.profileVersion }})</td>
              <td class="px-3 py-2">
                <span class="rounded px-2 py-0.5 text-xs" :class="stateClass(session.state)">
                  {{ stateLabel(session.state) }}
                </span>
              </td>
              <td class="px-3 py-2 text-xs">{{ session.startedAt }}</td>
              <td class="px-3 py-2 text-xs">{{ session.endedAt || '—' }}</td>
              <td class="px-3 py-2 text-xs">{{ session.failurePhase || '—' }}</td>
              <td class="px-3 py-2">
                <button class="text-primary" @click="openTimeline(session)">
                  {{ $t('netboot.page.session.viewTimeline') }}
                </button>
              </td>
            </tr>
            <tr v-if="!loading && sessions.length === 0">
              <td class="px-3 py-6 text-center text-gray-500" colspan="7">—</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex items-center justify-between text-sm">
        <span>{{ total }}</span>
        <div class="space-x-2">
          <button class="rounded border px-2 py-1" :disabled="page <= 1" @click="page -= 1; load()">
            ‹
          </button>
          <span>{{ page }}</span>
          <button
            class="rounded border px-2 py-1"
            :disabled="page * pageSize >= total"
            @click="page += 1; load()"
          >
            ›
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="timelineOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      @click.self="timelineOpen = false"
    >
      <div class="max-h-[80vh] w-[52rem] overflow-y-auto rounded bg-white p-6 dark:bg-gray-900">
        <h2 class="mb-4 text-lg font-semibold">{{ $t('netboot.page.session.timeline') }}</h2>

        <ol class="space-y-2">
          <li v-for="(event, index) in timeline" :key="index" class="rounded border p-2">
            <div class="flex items-center justify-between">
              <span class="font-medium">{{ event.phase }}</span>
              <span class="text-xs" :class="outcomeClass(event.outcome)">
                {{ $t(`netboot.enum.eventOutcome.${event.outcome ?? 'EVENT_OUTCOME_UNSPECIFIED'}`) }}
              </span>
            </div>
            <div class="text-xs text-gray-500">{{ event.time }}</div>
            <pre v-if="event.detail" class="mt-1 overflow-x-auto text-xs">{{ event.detail }}</pre>
          </li>
        </ol>

        <div v-if="evidence" class="mt-4">
          <div class="mb-1 text-sm font-medium">{{ $t('netboot.page.session.evidence') }}</div>
          <pre class="overflow-x-auto rounded bg-gray-100 p-3 text-xs dark:bg-gray-800">{{ evidence }}</pre>
        </div>

        <div class="mt-4 flex justify-end">
          <button class="rounded border px-4 py-1.5" @click="timelineOpen = false">
            {{ $t('netboot.common.close') }}
          </button>
        </div>
      </div>
    </div>
  </Page>
</template>
