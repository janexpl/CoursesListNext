<script setup lang="ts">
import type {
  JournalAttendee,
  StudentCertificateSummary
} from '~/composables/useApi'

defineProps<{
  attendees: JournalAttendee[]
  attendeesPending: boolean
  hasError: boolean
  deleteAttendeeError: string
  deleteAttendeeSuccess: string
  attendeeCertificateError: string
  attendeeCertificateSuccess: string
  generateAttendeeCertificateError: string
  generateAttendeeCertificateSuccess: string
  editingCertificateAttendeeId: number | null
  loadingAttendeeCertificatesId: number | null
  savingAttendeeCertificateId: number | null
  generatingAttendeeCertificateId: number | null
  attendeeCertificateDrafts: Record<number, string>
  attendeeCertificateOptions: Record<number, StudentCertificateSummary[]>
  deletingAttendeeId: number | null
  isClosed: boolean
}>()

const emit = defineEmits<{
  refresh: []
  startCertificateEdit: [attendee: JournalAttendee]
  cancelCertificateEdit: []
  updateCertificateDraft: [payload: { attendeeId: number, value: string }]
  saveCertificate: [attendee: JournalAttendee]
  generateCertificate: [attendee: JournalAttendee]
  detachCertificate: [attendee: JournalAttendee]
  deleteAttendee: [payload: { attendeeId: number, fullName: string }]
}>()

function formatCertificateNumber(certificate: {
  registryNumber: number
  courseSymbol: string
  registryYear: number
}) {
  return `${certificate.registryNumber}/${certificate.courseSymbol}/${certificate.registryYear}`
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold text-slate-900">Uczestnicy dziennika</h2>
        <p class="text-sm text-slate-500">
          Lista snapshotów kursantów przypisanych do tego szkolenia.
        </p>
      </div>

      <UButton
        icon="i-lucide-refresh-cw"
        color="neutral"
        variant="outline"
        :loading="attendeesPending"
        @click="emit('refresh')"
      >
        Odśwież
      </UButton>
    </div>

    <div
      v-if="deleteAttendeeError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ deleteAttendeeError }}
    </div>

    <div
      v-if="deleteAttendeeSuccess"
      class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ deleteAttendeeSuccess }}
    </div>

    <div
      v-if="attendeeCertificateError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ attendeeCertificateError }}
    </div>

    <div
      v-if="attendeeCertificateSuccess"
      class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ attendeeCertificateSuccess }}
    </div>

    <div
      v-if="generateAttendeeCertificateError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ generateAttendeeCertificateError }}
    </div>

    <div
      v-if="generateAttendeeCertificateSuccess"
      class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ generateAttendeeCertificateSuccess }}
    </div>

    <div
      v-if="hasError"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      Nie udało się pobrać listy uczestników.
    </div>

    <div
      v-else-if="attendeesPending"
      class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-8 text-sm text-slate-500"
    >
      Ładowanie uczestników...
    </div>

    <div
      v-else-if="attendees.length === 0"
      class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-sm text-slate-500"
    >
      Ten dziennik nie ma jeszcze żadnych uczestników.
    </div>

    <div v-else class="mt-5 overflow-hidden rounded-lg border border-slate-200">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-slate-200 text-sm">
          <thead class="bg-slate-50 text-left text-xs uppercase tracking-[0.16em] text-slate-500">
            <tr>
              <th class="px-4 py-3 font-medium">Lp.</th>
              <th class="px-4 py-3 font-medium">Uczestnik</th>
              <th class="px-4 py-3 font-medium">Data urodzenia</th>
              <th class="px-4 py-3 font-medium">Firma</th>
              <th class="px-4 py-3 font-medium">Zaświadczenie</th>
              <th class="px-4 py-3 font-medium">Akcje</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-200 bg-white text-slate-700">
            <tr v-for="attendee in attendees" :key="attendee.id">
              <td class="px-4 py-3">
                {{ attendee.sortOrder }}
              </td>
              <td class="px-4 py-3 font-medium text-slate-900">
                {{ attendee.fullNameSnapshot }}
              </td>
              <td class="px-4 py-3">
                {{ attendee.birthdateSnapshot }}
              </td>
              <td class="px-4 py-3">
                {{ attendee.companyNameSnapshot || 'Brak firmy' }}
              </td>
              <td class="min-w-80 px-4 py-3">
                <div
                  v-if="editingCertificateAttendeeId === attendee.id"
                  class="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-3"
                >
                  <div
                    v-if="loadingAttendeeCertificatesId === attendee.id"
                    class="text-sm text-slate-500"
                  >
                    Ładowanie dostępnych zaświadczeń...
                  </div>
                  <template v-else>
                    <label class="block space-y-2">
                      <span class="text-xs uppercase tracking-[0.14em] text-slate-500">
                        Wystawione zaświadczenie
                      </span>
                      <select
                        :value="attendeeCertificateDrafts[attendee.id]"
                        class="h-[42px] w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                        @change="
                          emit('updateCertificateDraft', {
                            attendeeId: attendee.id,
                            value: ($event.target as HTMLSelectElement).value
                          })
                        "
                      >
                        <option value="">Bez powiązanego zaświadczenia</option>
                        <option
                          v-for="certificate in attendeeCertificateOptions[attendee.id] || []"
                          :key="certificate.id"
                          :value="String(certificate.id)"
                        >
                          {{ formatCertificateNumber(certificate) }} • {{ certificate.date }}
                        </option>
                      </select>
                    </label>

                    <p
                      v-if="(attendeeCertificateOptions[attendee.id] || []).length === 0"
                      class="text-xs leading-5 text-slate-500"
                    >
                      Nie znaleziono wcześniej wystawionych zaświadczeń tego kursanta dla tego
                      kursu.
                    </p>

                    <div class="flex flex-wrap items-center gap-2">
                      <button
                        type="button"
                        class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                        :disabled="savingAttendeeCertificateId === attendee.id"
                        @click="emit('saveCertificate', attendee)"
                      >
                        {{
                          savingAttendeeCertificateId === attendee.id
                            ? 'Zapisywanie...'
                            : 'Zapisz powiązanie'
                        }}
                      </button>

                      <button
                        type="button"
                        class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                        :disabled="savingAttendeeCertificateId === attendee.id"
                        @click="emit('cancelCertificateEdit')"
                      >
                        Anuluj
                      </button>
                    </div>
                  </template>
                </div>

                <div v-else class="space-y-2">
                  <template v-if="attendee.certificate">
                    <NuxtLink
                      :to="`/certificates/${attendee.certificate.id}`"
                      class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:text-sky-900"
                    >
                      {{ formatCertificateNumber(attendee.certificate) }}
                    </NuxtLink>
                    <p class="text-xs text-slate-500">
                      Wystawiono: {{ attendee.certificate.date }}
                    </p>
                  </template>
                  <p v-else class="text-sm text-slate-500">Brak przypiętego zaświadczenia.</p>
                </div>
              </td>
              <td class="px-4 py-3 whitespace-nowrap">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    v-if="!attendee.certificate"
                    type="button"
                    class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="isClosed || generatingAttendeeCertificateId === attendee.id"
                    @click="emit('generateCertificate', attendee)"
                  >
                    {{
                      generatingAttendeeCertificateId === attendee.id
                        ? 'Wystawianie...'
                        : 'Wystaw zaświadczenie'
                    }}
                  </button>

                  <button
                    type="button"
                    class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="
                      isClosed
                        || loadingAttendeeCertificatesId === attendee.id
                        || savingAttendeeCertificateId === attendee.id
                        || generatingAttendeeCertificateId === attendee.id
                    "
                    @click="emit('startCertificateEdit', attendee)"
                  >
                    {{
                      loadingAttendeeCertificatesId === attendee.id
                        ? 'Ładowanie...'
                        : attendee.certificate
                          ? 'Zmień zaświadczenie'
                          : 'Podepnij zaświadczenie'
                    }}
                  </button>

                  <button
                    v-if="attendee.certificate"
                    type="button"
                    class="inline-flex items-center justify-center rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-700 transition hover:border-amber-300 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="
                      isClosed
                        || savingAttendeeCertificateId === attendee.id
                        || generatingAttendeeCertificateId === attendee.id
                    "
                    @click="emit('detachCertificate', attendee)"
                  >
                    {{
                      savingAttendeeCertificateId === attendee.id ? 'Zapisywanie...' : 'Odepnij'
                    }}
                  </button>

                  <span v-if="isClosed" class="text-xs text-slate-400">
                    Usuwanie uczestnika zablokowane po zamknięciu
                  </span>
                  <button
                    v-else
                    type="button"
                    class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="deletingAttendeeId === attendee.id"
                    @click="
                      emit('deleteAttendee', {
                        attendeeId: attendee.id,
                        fullName: attendee.fullNameSnapshot
                      })
                    "
                  >
                    {{ deletingAttendeeId === attendee.id ? 'Usuwanie...' : 'Usuń uczestnika' }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>
