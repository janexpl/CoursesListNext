<script setup lang="ts">
import type { JournalSession } from '~/composables/useApi'

defineProps<{
  sessions: JournalSession[]
  sessionsPending: boolean
  hasError: boolean
  generateSessionsError: string
  generateSessionsSuccess: string
  sessionUpdateSuccess: string
  generatingSessions: boolean
  sessionDrafts: Record<number, { sessionDate: string, trainerName: string }>
  sessionSaveErrors: Record<number, string>
  savingSessionId: number | null
  isClosed: boolean
  journalDateStart: string
  hasSessionChanges: (session: JournalSession) => boolean
}>()

const emit = defineEmits<{
  refresh: []
  generateSessions: []
  updateSessionDraft: [payload: { sessionId: number, sessionDate?: string, trainerName?: string }]
  saveSession: [session: JournalSession]
}>()

function formatSessionHours(value: string) {
  return `${value.replace(/\.0+$/, '').replace(/(\.\d*[1-9])0+$/, '$1')} h`
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-slate-900">Program szkolenia</h2>
        <p class="text-sm text-slate-500">
          Snapshot tematów i godzin skopiowanych z programu kursu do konkretnego dziennika.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <button
          v-if="!sessionsPending && sessions.length === 0 && !isClosed"
          type="button"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="generatingSessions"
          @click="emit('generateSessions')"
        >
          {{ generatingSessions ? 'Uzupełnianie...' : 'Uzupełnij z programu szkolenia' }}
        </button>

        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="sessionsPending"
          @click="emit('refresh')"
        >
          Odśwież
        </UButton>
      </div>
    </div>

    <p
      class="mt-4 rounded-lg border border-slate-200 bg-slate-50/80 px-4 py-3 text-xs leading-5 text-slate-500"
    >
      Na tym etapie edytujesz tylko datę realizacji i prowadzącego. Kolumnę czasu zegarowego
      ukryłem, bo obecnie dziennik operuje na dacie i liczbie godzin, bez dokładnych godzin od-do.
    </p>

    <div
      v-if="generateSessionsError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ generateSessionsError }}
    </div>

    <div
      v-if="generateSessionsSuccess"
      class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ generateSessionsSuccess }}
    </div>

    <div
      v-if="sessionUpdateSuccess"
      class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ sessionUpdateSuccess }}
    </div>

    <div
      v-if="hasError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      Nie udało się pobrać listy pozycji programu.
    </div>

    <div
      v-else-if="sessionsPending"
      class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-8 text-sm text-slate-500"
    >
      Ładowanie programu szkolenia...
    </div>

    <div
      v-else-if="sessions.length === 0"
      class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-500"
    >
      <p>Ten dziennik nie ma jeszcze skopiowanych pozycji programu.</p>
      <p class="mt-2">
        Dla starszych dzienników możesz uzupełnić je jednym kliknięciem na podstawie programu kursu.
      </p>
    </div>

    <div v-else class="mt-5 grid gap-4">
      <article
        v-for="session in sessions"
        :key="session.id"
        class="rounded-lg border border-slate-200 bg-slate-50/70 p-5"
      >
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="space-y-3 lg:max-w-[28rem]">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="inline-flex items-center justify-center rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600"
              >
                Lp. {{ session.sortOrder }}
              </span>
              <span
                class="inline-flex items-center justify-center rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600"
              >
                {{ formatSessionHours(session.hours) }}
              </span>
            </div>

            <div>
              <h3 class="text-base font-semibold leading-6 text-slate-900">
                {{ session.topic }}
              </h3>
              <p class="mt-1 text-sm text-slate-500">
                Domyślnie każda pozycja startuje z datą pierwszego dnia szkolenia.
              </p>
            </div>
          </div>

          <div class="grid gap-4 lg:min-w-[24rem] lg:max-w-[26rem]">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Data realizacji</span>
              <template v-if="isClosed">
                <div class="rounded-md border border-slate-200 bg-white px-3 py-2 text-slate-700">
                  {{ sessionDrafts[session.id]?.sessionDate || session.sessionDate || journalDateStart }}
                </div>
              </template>
              <input
                v-else
                :value="sessionDrafts[session.id]?.sessionDate"
                type="date"
                class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                @input="
                  emit('updateSessionDraft', {
                    sessionId: session.id,
                    sessionDate: ($event.target as HTMLInputElement).value
                  })
                "
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Prowadzący</span>
              <template v-if="isClosed">
                <div class="rounded-md border border-slate-200 bg-white px-3 py-2 text-slate-700">
                  {{ session.trainerName }}
                </div>
              </template>
              <input
                v-else
                :value="sessionDrafts[session.id]?.trainerName"
                type="text"
                class="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                @input="
                  emit('updateSessionDraft', {
                    sessionId: session.id,
                    trainerName: ($event.target as HTMLInputElement).value
                  })
                "
              >
            </label>

            <p v-if="sessionSaveErrors[session.id]" class="text-xs text-red-600">
              {{ sessionSaveErrors[session.id] }}
            </p>

            <div class="flex justify-end">
              <span v-if="isClosed" class="text-xs text-slate-400"> Dziennik zamknięty </span>
              <button
                v-else
                type="button"
                class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="savingSessionId === session.id || !hasSessionChanges(session)"
                @click="emit('saveSession', session)"
              >
                {{ savingSessionId === session.id ? 'Zapisywanie...' : 'Zapisz' }}
              </button>
            </div>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
