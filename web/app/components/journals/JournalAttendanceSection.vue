<script setup lang="ts">
import type { JournalAttendee, JournalSession } from '~/composables/useApi'

const props = defineProps<{
  attendees: JournalAttendee[]
  sessions: JournalSession[]
  attendancePending: boolean
  hasError: boolean
  attendanceSaveError: string
  attendanceSaveSuccess: string
  isClosed: boolean
  journalDateStart: string
  savingAttendanceKey: string | null
  bulkSavingAttendeeId: number | null
  attendanceDrafts: Record<string, boolean>
}>()

const emit = defineEmits<{
  refresh: []
  setAttendanceForAttendee: [payload: { attendeeId: number, present: boolean }]
  toggleAttendance: [payload: { sessionId: number, attendeeId: number, present: boolean }]
}>()

function shortenAttendanceTopic(topic: string, maxLength = 40) {
  const normalized = topic.trim()

  if (normalized.length <= maxLength) {
    return normalized
  }

  const shortened = normalized.slice(0, maxLength).trimEnd()
  const lastSpace = shortened.lastIndexOf(' ')
  const safeCut
    = lastSpace > Math.floor(maxLength * 0.6) ? shortened.slice(0, lastSpace) : shortened

  return `${safeCut}...`
}

function attendanceKey(sessionId: number, attendeeId: number) {
  return `${sessionId}:${attendeeId}`
}

function attendanceValue(sessionId: number, attendeeId: number) {
  return props.attendanceDrafts[attendanceKey(sessionId, attendeeId)] ?? false
}

function allAttendanceSelected(attendeeId: number) {
  if (props.sessions.length === 0) {
    return false
  }

  return props.sessions.every(session => attendanceValue(session.id, attendeeId))
}

function onAttendanceChange(sessionId: number, attendeeId: number, event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked ?? false
  emit('toggleAttendance', {
    sessionId,
    attendeeId,
    present: checked
  })
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-slate-900">Obecność uczestników</h2>
        <p class="text-sm text-slate-500">
          Zaznacz obecność dla każdej pozycji programu. Zmiana zapisuje się od razu po kliknięciu.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <span
          class="hidden w-[18rem] truncate text-right text-xs font-medium transition-opacity sm:inline-block"
          :class="
            attendanceSaveError
              ? 'text-red-600 opacity-100'
              : attendanceSaveSuccess
                ? 'text-emerald-600 opacity-100'
                : attendancePending && attendees.length > 0 && sessions.length > 0
                  ? 'text-slate-400 opacity-100'
                  : 'opacity-0'
          "
          :title="
            attendanceSaveError
              || attendanceSaveSuccess
              || (attendancePending && attendees.length > 0 && sessions.length > 0 ? 'Odświeżanie...' : '')
          "
        >
          {{
            attendanceSaveError
              ? 'Nie zapisano'
              : attendanceSaveSuccess
                ? 'Zapisano'
                : attendancePending && attendees.length > 0 && sessions.length > 0
                  ? 'Odświeżanie...'
                  : 'Status sekcji'
          }}
        </span>

        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="attendancePending"
          @click="emit('refresh')"
        >
          Odśwież
        </UButton>
      </div>
    </div>

    <div
      v-if="hasError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      Nie udało się pobrać obecności.
    </div>

    <div
      v-else-if="attendancePending && (attendees.length === 0 || sessions.length === 0)"
      class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-8 text-sm text-slate-500"
    >
      Ładowanie obecności...
    </div>

    <div
      v-else-if="attendees.length === 0 || sessions.length === 0"
      class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-500"
    >
      Obecność będzie dostępna po dodaniu uczestników i pozycji programu.
    </div>

    <div v-else class="mt-5 overflow-hidden rounded-lg border border-slate-200">
      <div class="overflow-x-auto">
        <table class="min-w-max border-separate border-spacing-0 text-sm text-slate-700">
          <thead class="bg-slate-50 text-left">
            <tr>
              <th
                class="sticky left-0 z-20 min-w-60 border-b border-r border-slate-200 bg-slate-50 px-4 py-4 align-top"
              >
                <div class="space-y-1">
                  <p class="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">
                    Uczestnik
                  </p>
                </div>
              </th>
              <th
                v-for="session in sessions"
                :key="session.id"
                class="w-40 min-w-40 border-b border-r border-slate-200 px-4 py-4 align-top"
              >
                <div class="space-y-2">
                  <p class="text-[11px] font-semibold tracking-[0.14em] text-sky-700">
                    Pozycja {{ session.sortOrder }}
                  </p>
                  <p class="text-xs font-medium text-slate-500">
                    {{ session.sessionDate || journalDateStart }}
                  </p>
                  <p class="text-sm font-medium leading-5 text-slate-800" :title="session.topic">
                    {{ shortenAttendanceTopic(session.topic) }}
                  </p>
                </div>
              </th>
            </tr>
          </thead>
          <tbody class="bg-white">
            <tr v-for="attendee in attendees" :key="attendee.id">
              <td
                class="sticky left-0 z-10 border-b border-r border-slate-200 bg-white px-4 py-4 align-top"
              >
                <div class="space-y-3">
                  <div class="space-y-1">
                    <p class="font-medium text-slate-900">
                      {{ attendee.fullNameSnapshot }}
                    </p>
                    <p class="text-xs text-slate-500">
                      {{ attendee.companyNameSnapshot || 'Brak firmy' }}
                    </p>
                  </div>

                  <button
                    type="button"
                    class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="isClosed || bulkSavingAttendeeId === attendee.id"
                    @click="
                      emit('setAttendanceForAttendee', {
                        attendeeId: attendee.id,
                        present: !allAttendanceSelected(attendee.id)
                      })
                    "
                  >
                    {{
                      bulkSavingAttendeeId === attendee.id
                        ? 'Zapisywanie...'
                        : allAttendanceSelected(attendee.id)
                          ? 'Wyczyść wiersz'
                          : 'Zaznacz wszystko'
                    }}
                  </button>
                </div>
              </td>
              <td
                v-for="session in sessions"
                :key="`${attendee.id}-${session.id}`"
                class="border-b border-r border-slate-200 px-4 py-4 text-center align-middle"
              >
                <label class="inline-flex items-center justify-center">
                  <input
                    :checked="attendanceValue(session.id, attendee.id)"
                    type="checkbox"
                    class="h-4 w-4 rounded border-slate-300 text-sky-600 focus:ring-sky-500"
                    :disabled="
                      isClosed
                        || savingAttendanceKey === attendanceKey(session.id, attendee.id)
                        || bulkSavingAttendeeId === attendee.id
                    "
                    @change="onAttendanceChange(session.id, attendee.id, $event)"
                  >
                </label>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>
