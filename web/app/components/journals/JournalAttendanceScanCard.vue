<script setup lang="ts">
import type { JournalAttendanceScan } from '~/composables/useApi'

defineProps<{
  scan: JournalAttendanceScan | null
  pending: boolean
  hasError: boolean
  actionError: string
  actionSuccess: string
  uploadPending: boolean
  deletePending: boolean
  inputKey: number
  selectedFileName: string
  downloadUrl: string
}>()

const emit = defineEmits<{
  fileSelected: [file: File | null]
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

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement | null
  emit('fileSelected', input?.files?.[0] ?? null)
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-slate-900">Podpisana lista obecności</h2>
      <p class="text-sm text-slate-500">
        Załącz podpisany skan PDF lub zdjęcie listy obecności i przechowuj go razem z dziennikiem.
      </p>
    </div>

    <div
      v-if="actionError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ actionError }}
    </div>

    <div
      v-if="actionSuccess"
      class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ actionSuccess }}
    </div>

    <div
      v-if="hasError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      Nie udało się pobrać informacji o skanie listy obecności.
    </div>

    <div
      v-else-if="pending"
      class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-500"
    >
      Ładowanie informacji o skanie...
    </div>

    <template v-else>
      <div v-if="scan" class="mt-5 rounded-lg border border-slate-200 bg-slate-50/70 p-4">
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

        <div class="mt-4 flex flex-wrap items-center gap-2">
          <a
            :href="downloadUrl"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Pobierz skan
          </a>

          <button
            type="button"
            class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="deletePending || uploadPending"
            @click="emit('deleteScan')"
          >
            {{ deletePending ? 'Usuwanie...' : 'Usuń skan' }}
          </button>
        </div>
      </div>

      <div
        v-else
        class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
      >
        Nie załączono jeszcze skanu podpisanej listy obecności.
      </div>

      <div class="mt-5 space-y-4">
        <label class="block space-y-2">
          <span class="text-sm font-medium text-slate-700">
            {{ scan ? 'Podmień skan' : 'Załącz skan' }}
          </span>
          <input
            :key="inputKey"
            type="file"
            accept=".pdf,image/png,image/jpeg,application/pdf"
            class="block w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 file:mr-4 file:rounded-md file:border-0 file:bg-slate-100 file:px-3 file:py-2 file:font-medium file:text-slate-700 hover:file:bg-slate-200"
            @change="onFileChange"
          >
        </label>

        <p class="text-xs leading-5 text-slate-500">
          Obsługiwane formaty: PDF, JPG, PNG. Maksymalny rozmiar pliku: 16 MB.
        </p>

        <p v-if="selectedFileName" class="text-xs text-slate-500">
          Wybrano plik:
          <span class="font-medium text-slate-700">{{ selectedFileName }}</span>
        </p>

        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="uploadPending || deletePending || !selectedFileName"
            @click="emit('upload')"
          >
            {{
              uploadPending
                ? 'Wysyłanie...'
                : scan
                  ? 'Podmień skan'
                  : 'Załącz skan'
            }}
          </button>

          <button
            v-if="selectedFileName"
            type="button"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
            :disabled="uploadPending"
            @click="emit('clearSelection')"
          >
            Wyczyść wybór
          </button>
        </div>
      </div>
    </template>
  </section>
</template>
