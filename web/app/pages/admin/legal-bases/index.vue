<script setup lang="ts">
import type { LegalBasisDetails } from '~/composables/useApi'

definePageMeta({
  middleware: 'admin'
})

useSeoMeta({
  title: 'Podstawy prawne'
})

const api = useApi()
const { legalBases, pending, error, refresh } = useLegalBases()

// Edycja jednej podstawy naraz. Przed zapisem pokazujemy, których kursów dotyczy zmiana:
// treść trafi na wszystkie ich przyszłe zaświadczenia.
const editing = ref<LegalBasisDetails | null>(null)
const editLoading = ref(false)
const editPending = ref(false)
const editError = ref('')
const editForm = reactive({ name: '', content: '' })

const deletePendingId = ref<number | null>(null)
const listError = ref('')
const successMessage = ref('')

const editChanged = computed(() => !!editing.value
  && (editForm.name.trim() !== editing.value.name || editForm.content.trim() !== editing.value.content))

async function startEdit(id: number) {
  successMessage.value = ''
  editError.value = ''
  editLoading.value = true
  try {
    const response = await api.legalBasis(id)
    editing.value = response.data
    editForm.name = response.data.name
    editForm.content = response.data.content
  } catch (apiError) {
    listError.value = getApiErrorMessage(apiError, 'Nie udało się pobrać podstawy prawnej.')
  } finally {
    editLoading.value = false
  }
}

function cancelEdit() {
  editing.value = null
  editError.value = ''
}

async function onSave() {
  if (!editing.value) {
    return
  }
  if (!editForm.name.trim() || !editForm.content.trim()) {
    editError.value = 'Podaj nazwę i treść podstawy prawnej.'
    return
  }
  editPending.value = true
  editError.value = ''
  try {
    await api.updateLegalBasis(editing.value.id, {
      name: editForm.name.trim(),
      content: editForm.content.trim()
    })
    successMessage.value = `Zapisano „${editForm.name.trim()}”.`
    editing.value = null
    await refresh()
  } catch (apiError) {
    editError.value = getApiErrorMessage(apiError, 'Nie udało się zapisać podstawy prawnej.')
  } finally {
    editPending.value = false
  }
}

async function onDelete(id: number) {
  successMessage.value = ''
  listError.value = ''
  deletePendingId.value = id
  try {
    await api.deleteLegalBasis(id)
    if (editing.value?.id === id) {
      editing.value = null
    }
    await refresh()
  } catch (apiError) {
    listError.value = getApiErrorMessage(apiError, 'Nie udało się usunąć podstawy prawnej.')
  } finally {
    deletePendingId.value = null
  }
}

function formatCourseCount(count: number) {
  if (count === 0) {
    return 'nieużywana'
  }
  if (count === 1) {
    return '1 kurs'
  }
  const lastDigit = count % 10
  const lastTwo = count % 100
  return lastDigit >= 2 && lastDigit <= 4 && (lastTwo < 12 || lastTwo > 14) ? `${count} kursy` : `${count} kursów`
}
</script>

<template>
  <section class="space-y-6">
    <AdminTabs />

    <div class="space-y-2">
      <div class="flex items-center gap-2">
        <h1 class="min-w-0 text-2xl font-semibold tracking-tight text-slate-900 [overflow-wrap:anywhere]">
          Podstawy prawne
        </h1>
        <HelpHint text="Biblioteka podstaw prawnych cytowanych na zaświadczeniach. Kurs wskazuje jedną z nich w swoich ustawieniach, a szablon wstawia ją znacznikiem {{ podstawa_prawna }}. Zmiana treści dotyczy wyłącznie zaświadczeń wystawionych później - wydane zachowują treść z dnia wystawienia." />
      </div>
    </div>

    <p
      v-if="successMessage"
      class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
    >
      {{ successMessage }}
    </p>
    <p
      v-if="listError"
      class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ listError }}
    </p>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać listy podstaw prawnych.
    </div>

    <div
      v-else-if="pending && !legalBases.length"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie...
    </div>

    <div
      v-else-if="!legalBases.length"
      class="rounded-xl border border-dashed border-slate-300 bg-white/90 px-6 py-10 text-sm text-slate-500"
    >
      Biblioteka jest pusta. Podstawę prawną dodaje się w ustawieniach kursu.
    </div>

    <ul
      v-else
      class="space-y-4"
    >
      <li
        v-for="basis in legalBases"
        :key="basis.id"
        class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm"
      >
        <template v-if="editing?.id === basis.id">
          <div class="space-y-4">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Nazwa</span>
              <input
                v-model="editForm.name"
                type="text"
                maxlength="200"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Treść</span>
              <textarea
                v-model="editForm.content"
                rows="4"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              />
            </label>

            <!-- Zakres zmiany na wierzchu, nie w podpowiedzi: trzeba go zobaczyć przed zapisem. -->
            <div
              v-if="editing.courses.length"
              class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900"
            >
              <p class="font-medium">
                Zmiana dotyczy przyszłych zaświadczeń {{ editing.courses.length === 1 ? 'kursu' : 'kursów' }}:
              </p>
              <p class="mt-1 [overflow-wrap:anywhere]">
                {{ editing.courses.map(course => course.symbol).join(', ') }}
              </p>
              <p class="mt-2 text-amber-800">
                Wydane zaświadczenia zachowują treść z dnia wystawienia.
              </p>
            </div>

            <p
              v-if="editError"
              class="rounded-lg border border-red-200 bg-white px-4 py-3 text-sm text-red-700"
            >
              {{ editError }}
            </p>

            <div class="flex flex-wrap gap-3">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-sky-300"
                :disabled="editPending || !editChanged"
                @click="onSave"
              >
                {{ editPending ? 'Zapisywanie...' : 'Zapisz' }}
              </button>
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400"
                :disabled="editPending"
                @click="cancelEdit"
              >
                Anuluj
              </button>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0 space-y-2">
              <p class="font-medium text-slate-900 [overflow-wrap:anywhere]">
                {{ basis.name }}
              </p>
              <p class="text-sm leading-6 text-slate-600 [overflow-wrap:anywhere]">
                {{ basis.content }}
              </p>
              <UBadge
                :color="basis.courseCount ? 'primary' : 'neutral'"
                variant="subtle"
              >
                {{ formatCourseCount(basis.courseCount) }}
              </UBadge>
            </div>

            <div class="flex shrink-0 flex-wrap gap-2">
              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:opacity-60"
                :disabled="editLoading"
                @click="startEdit(basis.id)"
              >
                Edytuj
              </button>
              <!-- Usunąć da się tylko nieużywaną - API i tak odmówi (409), ale nie kusimy. -->
              <button
                v-if="!basis.courseCount"
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:opacity-60"
                :disabled="deletePendingId === basis.id"
                @click="onDelete(basis.id)"
              >
                {{ deletePendingId === basis.id ? 'Usuwanie...' : 'Usuń' }}
              </button>
            </div>
          </div>
        </template>
      </li>
    </ul>
  </section>
</template>
