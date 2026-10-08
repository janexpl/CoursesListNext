<script setup lang="ts">
// Wybór podstawy prawnej kursu. Jej treść trafia na zaświadczenie w miejsce znacznika
// {{ podstawa_prawna }}; nową podstawę może dodać każdy, kto edytuje kursy, a poprawić
// treść istniejącej - tylko administrator (zmiana dotyka wszystkich kursów, które jej
// używają), stąd tylko odnośnik do zakładki administratora.
withDefaults(defineProps<{
  // Czy szablon (albo któreś tłumaczenie) zawiera znacznik - do ostrzeżeń.
  templateUsesTag: boolean
  disabled?: boolean
}>(), {
  disabled: false
})

const legalBasisId = defineModel<number | null>({ required: true })

const api = useApi()
const auth = useAuth()
const isAdmin = computed(() => auth.user.value?.role === 1)
const { legalBases, pending, error, refresh, findLegalBasis } = useLegalBases()

const selected = computed(() => findLegalBasis(legalBasisId.value))
const tag = '{{ podstawa_prawna }}'

const showCreate = ref(false)
const createPending = ref(false)
const createError = ref('')
const createForm = reactive({ name: '', content: '' })

function openCreate() {
  showCreate.value = !showCreate.value
  createError.value = ''
  createForm.name = ''
  createForm.content = ''
}

async function onCreate() {
  if (!createForm.name.trim() || !createForm.content.trim()) {
    createError.value = 'Podaj nazwę i treść podstawy prawnej.'
    return
  }
  createPending.value = true
  createError.value = ''
  try {
    const response = await api.createLegalBasis({
      name: createForm.name.trim(),
      content: createForm.content.trim()
    })
    await refresh()
    // Nowa podstawa jest od razu wybrana - po to ją dodano.
    legalBasisId.value = response.data.id
    showCreate.value = false
  } catch (apiError) {
    createError.value = getApiErrorMessage(apiError, 'Nie udało się dodać podstawy prawnej.')
  } finally {
    createPending.value = false
  }
}

function onSelect(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  legalBasisId.value = value ? Number.parseInt(value, 10) : null
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="flex items-center gap-2">
      <h2 class="min-w-0 text-lg font-semibold text-slate-900">
        Podstawa prawna
      </h2>
      <HelpHint text="Treść wstawiana na zaświadczenie w miejsce znacznika {{ podstawa_prawna }} z szablonu. Zaświadczenie zapamiętuje ją w chwili wystawienia, więc późniejsza zmiana w bibliotece nie dotyczy już wydanych dokumentów." />
    </div>

    <div class="mt-5 space-y-4">
      <label class="block space-y-2">
        <span class="text-sm font-medium text-slate-700">Podstawa prawna kursu</span>
        <select
          :value="legalBasisId ?? ''"
          :disabled="disabled || pending"
          class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100 disabled:cursor-not-allowed disabled:bg-slate-100"
          @change="onSelect"
        >
          <option value="">
            — brak podstawy prawnej —
          </option>
          <option
            v-for="basis in legalBases"
            :key="basis.id"
            :value="basis.id"
          >
            {{ basis.name }}
          </option>
        </select>
      </label>

      <p
        v-if="error"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
      >
        Nie udało się pobrać listy podstaw prawnych.
      </p>

      <blockquote
        v-if="selected"
        class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm leading-6 text-slate-700"
      >
        {{ selected.content }}
      </blockquote>

      <p
        v-if="templateUsesTag && !selected"
        class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-800"
      >
        Szablon zawiera znacznik <code class="font-mono text-xs">{{ tag }}</code>, a kurs nie ma
        wybranej podstawy — na zaświadczeniu będzie w tym miejscu pusto.
      </p>
      <p
        v-else-if="selected && !templateUsesTag"
        class="rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm leading-6 text-slate-600"
      >
        Szablon nie zawiera znacznika <code class="font-mono text-xs">{{ tag }}</code>, więc wybrana
        podstawa nie pojawi się na zaświadczeniu. Wstaw znacznik w zakładce z szablonem.
      </p>

      <div class="flex flex-wrap items-center gap-3">
        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="disabled"
          @click="openCreate"
        >
          Dodaj nową podstawę
        </button>
        <NuxtLink
          v-if="isAdmin"
          to="/admin/legal-bases"
          class="text-sm font-medium text-sky-700 underline decoration-sky-200 underline-offset-2 hover:decoration-sky-500"
        >
          Zarządzaj podstawami prawnymi
        </NuxtLink>
      </div>

      <div
        v-if="showCreate"
        class="space-y-4 rounded-lg border border-sky-200 bg-sky-50/60 p-4"
      >
        <label class="block space-y-2">
          <span class="flex items-center gap-2 text-sm font-medium text-slate-700">Nazwa <HelpHint text="Krótka nazwa do wyboru z listy, np. „Szkolenia BHP — rozporządzenie z 27.07.2004”. Nie trafia na zaświadczenie." /></span>
          <input
            v-model="createForm.name"
            type="text"
            maxlength="200"
            class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
          >
        </label>
        <label class="block space-y-2">
          <span class="flex items-center gap-2 text-sm font-medium text-slate-700">Treść <HelpHint text="Tekst wstawiany w szablon, zwykle od „§ …” do nawiasu z Dz. U. Zwrot „na podstawie” zostaje w szablonie." /></span>
          <textarea
            v-model="createForm.content"
            rows="4"
            class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
          />
        </label>
        <p
          v-if="createError"
          class="rounded-lg border border-red-200 bg-white px-4 py-3 text-sm text-red-700"
        >
          {{ createError }}
        </p>
        <div class="flex flex-wrap gap-3">
          <button
            type="button"
            class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-sky-300"
            :disabled="createPending"
            @click="onCreate"
          >
            {{ createPending ? 'Zapisywanie...' : 'Dodaj i wybierz' }}
          </button>
          <button
            type="button"
            class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400"
            :disabled="createPending"
            @click="showCreate = false"
          >
            Anuluj
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
