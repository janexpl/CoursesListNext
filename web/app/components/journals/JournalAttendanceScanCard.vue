<script setup lang="ts">
import type { JournalScan } from '~/composables/useApi'

const props = withDefaults(defineProps<{
  title: string
  description: string
  loadErrorMessage: string
  emptyStateMessage: string
  embedded?: boolean
  scan: JournalScan | null
  pending: boolean
  hasError: boolean
  actionError: string
  actionSuccess: string
  uploadPending: boolean
  deletePending: boolean
  selectedFileName: string
  downloadUrl: string
}>(), {
  embedded: false
})

const emit = defineEmits<{
  chooseFile: []
  upload: []
  clearSelection: []
  deleteScan: []
}>()

function formatDateTime(value: string | null) {
  if (!value) {
    return 'Brak'
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return parsed.toLocaleString('pl-PL')
}

function formatFileSize(value: number) {
  if (value >= 1024 * 1024) {
    return `${(value / (1024 * 1024)).toFixed(2).replace(/\.00$/, '')} MB`
  }

  if (value >= 1024) {
    return `${(value / 1024).toFixed(1).replace(/\.0$/, '')} KB`
  }

  return `${value} B`
}
</script>

<template>
  <component
    :is="props.embedded ? 'div' : 'section'"
    :class="props.embedded
      ? 'rounded-lg border border-slate-200 bg-slate-50/40 p-3'
      : 'rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm'"
  >
    <div class="space-y-1">
      <component
        :is="props.embedded ? 'h3' : 'h2'"
        :class="props.embedded ? 'text-base font-semibold text-slate-900' : 'text-lg font-semibold text-slate-900'"
      >
        {{ title }}
      </component>
      <p :class="props.embedded ? 'text-xs leading-5 text-slate-500' : 'text-sm text-slate-500'">
        {{ description }}
      </p>
    </div>

    <div
      v-if="actionError"
      :class="props.embedded
        ? 'mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700'
        : 'mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700'"
    >
      {{ actionError }}
    </div>

    <div
      v-if="actionSuccess"
      :class="props.embedded
        ? 'mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs leading-5 text-emerald-700'
        : 'mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700'"
    >
      {{ actionSuccess }}
    </div>

    <div
      v-if="hasError"
      :class="props.embedded
        ? 'mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700'
        : 'mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700'"
    >
      {{ loadErrorMessage }}
    </div>

    <div
      v-else-if="pending"
      :class="props.embedded
        ? 'mt-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500'
        : 'mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-500'"
    >
      Ładowanie informacji o skanie...
    </div>

    <template v-else>
      <template v-if="props.embedded">
        <div
          v-if="scan"
          class="mt-3 rounded-lg border border-slate-200 bg-slate-50/70 px-3 py-2.5"
        >
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-slate-900">
                {{ scan.fileName }}
              </p>
              <p class="mt-1 text-xs leading-5 text-slate-500">
                {{ formatFileSize(scan.fileSize) }} • {{ formatDateTime(scan.updatedAt) }}
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <a
                :href="downloadUrl"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              >
                Pobierz
              </a>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="deletePending || uploadPending"
                @click="emit('deleteScan')"
              >
                {{ deletePending ? 'Usuwanie...' : 'Usuń' }}
              </button>
            </div>
          </div>
        </div>

        <div
          v-else
          class="mt-3 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500"
        >
          {{ emptyStateMessage }}
        </div>

        <div class="mt-3 rounded-lg border border-slate-200 bg-white px-3 py-2.5">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <p class="truncate text-sm text-slate-700">
                {{ selectedFileName || 'Nie wybrano pliku' }}
              </p>
              <p class="mt-1 text-xs text-slate-400">
                PDF, JPG lub PNG do 16 MB
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                :disabled="uploadPending || deletePending"
                @click="emit('chooseFile')"
              >
                {{ scan ? 'Zmień plik' : 'Wybierz plik' }}
              </button>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-3 py-2 text-xs font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="uploadPending || deletePending || !selectedFileName"
                @click="emit('upload')"
              >
                {{
                  uploadPending
                    ? 'Wysyłanie...'
                    : scan
                      ? 'Podmień'
                      : 'Załącz'
                }}
              </button>

              <button
                v-if="selectedFileName"
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                :disabled="uploadPending"
                @click="emit('clearSelection')"
              >
                Wyczyść
              </button>
            </div>
          </div>
        </div>
      </template>

      <template v-else>
        <div v-if="scan" class="mt-4 rounded-lg border border-slate-200 bg-slate-50/70 p-3">
          <div class="grid gap-3 text-sm text-slate-700">
            <div>
              <p class="text-xs uppercase tracking-[0.14em] text-slate-400">Plik</p>
              <p class="mt-1 break-words font-medium text-slate-900">
                {{ scan.fileName }}
              </p>
            </div>

            <div class="grid gap-3 sm:grid-cols-2">
              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-400">Typ</p>
                <p class="mt-1">{{ scan.contentType }}</p>
              </div>

              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-400">Rozmiar</p>
                <p class="mt-1">{{ formatFileSize(scan.fileSize) }}</p>
              </div>

              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-400">Załączono</p>
                <p class="mt-1">{{ formatDateTime(scan.createdAt) }}</p>
              </div>

              <div>
                <p class="text-xs uppercase tracking-[0.14em] text-slate-400">Ostatnia podmiana</p>
                <p class="mt-1">{{ formatDateTime(scan.updatedAt) }}</p>
              </div>
            </div>
          </div>

          <div class="mt-3 flex flex-wrap items-center gap-2">
            <a
              :href="downloadUrl"
              class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
            >
              Pobierz skan
            </a>

            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-60"
              :disabled="deletePending || uploadPending"
              @click="emit('deleteScan')"
            >
              {{ deletePending ? 'Usuwanie...' : 'Usuń skan' }}
            </button>
          </div>
        </div>

        <div
          v-else
          class="mt-4 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-4 text-sm text-slate-500"
        >
          {{ emptyStateMessage }}
        </div>

        <div class="mt-4 space-y-3">
          <p class="text-xs leading-5 text-slate-500">
            Obsługiwane formaty: PDF, JPG, PNG. Maksymalny rozmiar pliku: 16 MB.
          </p>

          <div
            class="flex flex-col gap-2 rounded-lg border border-slate-200 bg-white px-3 py-3 sm:flex-row sm:items-center sm:justify-between"
          >
            <div class="min-w-0">
              <p class="text-xs uppercase tracking-[0.14em] text-slate-400">
                {{ scan ? 'Nowy plik' : 'Plik do załączenia' }}
              </p>
              <p class="mt-1 truncate text-sm text-slate-700">
                {{ selectedFileName || 'Nie wybrano pliku' }}
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                :disabled="uploadPending || deletePending"
                @click="emit('chooseFile')"
              >
                {{ scan ? 'Zmień plik' : 'Wybierz plik' }}
              </button>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-3 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="uploadPending || deletePending || !selectedFileName"
                @click="emit('upload')"
              >
                {{
                  uploadPending
                    ? 'Wysyłanie...'
                    : scan
                      ? 'Podmień'
                      : 'Załącz'
                }}
              </button>

              <button
                v-if="selectedFileName"
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                :disabled="uploadPending"
                @click="emit('clearSelection')"
              >
                Wyczyść
              </button>
            </div>
          </div>
        </div>
      </template>
    </template>
  </component>
</template>
