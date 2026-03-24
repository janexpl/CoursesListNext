<script setup lang="ts">
import type { JournalDetails } from '~/composables/useApi'

defineProps<{
  journal: JournalDetails
  attendeeCount: number
  sessionCount: number
  formattedTotalHours: string
  printJournalPending: boolean
  closeJournalPending: boolean
  isClosed: boolean
  deleteJournalPending: boolean
  journalPdfDownloadUrl: string
  editJournalLink: string
}>()

const emit = defineEmits<{
  printJournal: []
  printAttendanceList: []
  closeJournal: []
  deleteJournal: []
}>()

function statusLabel(value: string) {
  return value === 'closed' ? 'Zamknięty' : 'Roboczy'
}

function statusBadgeClass(value: string) {
  return value === 'closed'
    ? 'border-slate-300 bg-slate-100 text-slate-700'
    : 'border-sky-200 bg-sky-50 text-sky-700'
}
</script>

<template>
  <div
    class="flex flex-col gap-4 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur lg:flex-row lg:items-start lg:justify-between"
  >
    <div class="space-y-3">
      <div class="flex flex-wrap items-center gap-2">
        <NuxtLink
          to="/journals"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Wróć do listy
        </NuxtLink>

        <span
          class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
          :class="statusBadgeClass(journal.status)"
        >
          {{ statusLabel(journal.status) }}
        </span>

        <span class="text-xs uppercase tracking-[0.16em] text-slate-400">
          {{ journal.courseSymbol }}
        </span>
      </div>

      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Dziennik</p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          {{ journal.title }}
        </h1>
      </div>
    </div>

    <div class="grid gap-4 text-sm lg:min-w-[34rem] lg:grid-cols-2 lg:items-start">
      <div class="grid gap-3 rounded-lg border border-slate-200 bg-slate-50/80 p-4">
        <div class="grid grid-cols-2 gap-3">
          <div class="rounded-lg border border-slate-200 bg-white px-2 py-3">
            <p class="text-xs uppercase tracking-[0.16em] text-slate-400">Uczestnicy</p>
            <p class="mt-1 text-lg font-semibold text-slate-900">
              {{ attendeeCount }}
            </p>
          </div>

          <div class="rounded-lg border border-slate-200 bg-white px-2 py-3">
            <p class="text-xs uppercase tracking-[0.16em] text-slate-400">Zajęcia</p>
            <p class="mt-1 text-lg font-semibold text-slate-900">
              {{ sessionCount }}
            </p>
          </div>
        </div>

        <div>
          <p class="text-xs uppercase tracking-[0.16em] text-slate-400">Termin</p>
          <p class="mt-1 font-medium text-slate-700">
            {{ journal.dateStart }} - {{ journal.dateEnd }}
          </p>
        </div>

        <div>
          <p class="text-xs uppercase tracking-[0.16em] text-slate-400">Liczba godzin</p>
          <p class="mt-1 font-medium text-slate-700">
            {{ formattedTotalHours }}
          </p>
        </div>
      </div>

      <div class="grid gap-3 rounded-lg border border-slate-200 bg-slate-50/80 p-4">
        <p class="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Akcje</p>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="printJournalPending"
          @click="emit('printJournal')"
        >
          {{ printJournalPending ? 'Przygotowywanie wydruku...' : 'Drukuj dziennik' }}
        </button>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="printJournalPending"
          @click="emit('printAttendanceList')"
        >
          {{ printJournalPending ? 'Przygotowywanie wydruku...' : 'Drukuj listę obecności' }}
        </button>

        <a
          :href="journalPdfDownloadUrl"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Pobierz PDF
        </a>

        <NuxtLink
          :to="editJournalLink"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Edytuj nagłówek
        </NuxtLink>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-emerald-300 bg-white px-4 py-2 text-sm font-medium text-emerald-700 transition hover:border-emerald-400 hover:text-emerald-900 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="closeJournalPending || isClosed"
          @click="emit('closeJournal')"
        >
          {{
            isClosed
              ? 'Dziennik zamknięty'
              : closeJournalPending
                ? 'Zamykanie...'
                : 'Zamknij dziennik'
          }}
        </button>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-red-300 bg-white px-4 py-2 text-sm font-medium text-red-700 transition hover:border-red-400 hover:text-red-900 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="deleteJournalPending"
          @click="emit('deleteJournal')"
        >
          {{ deleteJournalPending ? 'Usuwanie...' : 'Usuń dziennik' }}
        </button>
      </div>
    </div>
  </div>
</template>
