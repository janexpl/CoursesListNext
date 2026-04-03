<script setup lang="ts">
import type { JournalDetails, StudentSummary } from '~/composables/useApi'

defineProps<{
  journal: JournalDetails
  isClosed: boolean
  studentSearch: string
  studentsPending: boolean
  studentSearchError: string
  addAttendeeError: string
  addAttendeeSuccess: string
  showNoStudentResults: boolean
  availableStudentOptions: StudentSummary[]
  addingStudentId: number | null
}>()

const emit = defineEmits<{
  'update:studentSearch': [value: string]
  'addAttendee': [student: StudentSummary]
}>()

const searchInputRef = ref<HTMLInputElement | null>(null)

function focusSearchInput() {
  searchInputRef.value?.focus()
}

defineExpose({
  focusSearchInput
})
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-slate-900">Dodaj uczestnika</h2>
      <p class="text-sm leading-6 text-slate-500">
        Wyszukaj kursanta i dodawaj kolejne osoby bez opuszczania tej sekcji.
      </p>
    </div>

    <div
      v-if="isClosed"
      class="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700"
    >
      Dziennik jest zamknięty. Nie można już dodawać kolejnych uczestników.
    </div>

    <div v-else class="mt-4 space-y-3">
      <label class="block space-y-2">
        <div class="flex items-center justify-between gap-3">
          <span class="text-sm font-medium text-slate-700">Szukaj kursanta</span>
          <span
            v-if="availableStudentOptions.length"
            class="text-xs font-medium text-slate-400"
          >
            {{ availableStudentOptions.length }} wyników
          </span>
        </div>
        <input
          ref="searchInputRef"
          :value="studentSearch"
          type="text"
          placeholder="Nazwisko, imię, PESEL"
          class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-sm text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
          @input="emit('update:studentSearch', ($event.target as HTMLInputElement).value)"
        >
      </label>

      <p
        v-if="journal.companyName"
        class="rounded-lg border border-slate-200 bg-slate-50/80 px-4 py-2.5 text-xs leading-5 text-slate-500"
      >
        Wyszukiwanie zawęża wyniki do firmy {{ journal.companyName }}.
      </p>

      <div
        v-if="addAttendeeError"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-2.5 text-sm text-red-700"
      >
        {{ addAttendeeError }}
      </div>

      <div
        v-if="addAttendeeSuccess"
        class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-sm text-emerald-700"
      >
        {{ addAttendeeSuccess }}
      </div>

      <div
        v-if="studentSearchError"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-2.5 text-sm text-red-700"
      >
        {{ studentSearchError }}
      </div>

      <div
        v-else-if="studentsPending"
        class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-5 text-sm text-slate-500"
      >
        Wyszukiwanie kursantów...
      </div>

      <div
        v-else-if="showNoStudentResults"
        class="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-5 text-sm text-slate-500"
      >
        Brak kursantów pasujących do podanej frazy lub wszyscy znalezieni kursanci są już w
        dzienniku.
      </div>

      <div
        v-else-if="availableStudentOptions.length"
        class="max-h-[24rem] overflow-y-auto pr-1"
      >
        <div class="grid gap-2 xl:grid-cols-2">
          <article
            v-for="student in availableStudentOptions"
            :key="student.id"
            class="rounded-lg border border-slate-200 bg-slate-50/70 p-3"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0 space-y-1">
                <h3 class="truncate text-sm font-semibold text-slate-900">
                  {{ student.lastName }} {{ student.firstName }}
                </h3>
                <p class="text-xs leading-5 text-slate-500">
                  <span>{{ student.birthDate }}</span>
                  <span class="mx-1 text-slate-300">•</span>
                  <span>{{ student.company?.name || 'Brak firmy' }}</span>
                </p>
              </div>

              <button
                type="button"
                class="inline-flex shrink-0 items-center justify-center rounded-lg bg-sky-600 px-3 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="addingStudentId === student.id"
                @click="emit('addAttendee', student)"
              >
                {{ addingStudentId === student.id ? 'Dodawanie...' : 'Dodaj' }}
              </button>
            </div>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>
