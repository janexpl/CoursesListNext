<script setup lang="ts">
import type { ApiKey } from '~/composables/useApi'

definePageMeta({
  middleware: 'admin'
})

useSeoMeta({
  title: 'Klucze API'
})

const api = useApi()

const search = ref('')
const submitPending = ref(false)
const revokePendingId = ref<number | null>(null)
const createError = ref('')
const revokeError = ref('')
const copied = ref(false)

// Surowy klucz istnieje tylko w tej zmiennej i tylko do czasu zamknięcia
// panelu — serwer trzyma wyłącznie jego skrót i nie pokaże go drugi raz.
const issuedKey = ref<{ token: string, name: string } | null>(null)

const form = reactive({
  name: '',
  userId: 0,
  scopes: [] as string[],
  expiresAt: ''
})

const { data, pending, error, refresh } = await useAsyncData('admin-api-keys', async () => {
  const [keys, scopes, users] = await Promise.all([
    api.apiKeys(),
    api.apiKeyScopes(),
    api.users()
  ])

  return { keys: keys.data, scopes: scopes.data, users: users.data }
})

const keys = computed(() => data.value?.keys ?? [])
const availableScopes = computed(() => data.value?.scopes ?? [])
const users = computed(() => data.value?.users ?? [])

const resourceLabels: Record<string, string> = {
  'students': 'Kursanci',
  'companies': 'Firmy',
  'courses': 'Kursy',
  'certificates': 'Zaświadczenia',
  'journals': 'Dzienniki',
  'registries': 'Numery rejestru',
  'dashboard': 'Pulpit',
  'users': 'Użytkownicy',
  'audit-log': 'Historia zmian'
}

// Serwer sortuje katalog po angielskich nazwach zakresów, co po polsku układa
// się przypadkowo. Trzymamy kolejność menu aplikacji, a uprawnienia dostępne
// wyłącznie administratorom zostawiamy na końcu.
const resourceOrder = [
  'students',
  'companies',
  'courses',
  'certificates',
  'journals',
  'dashboard',
  'registries',
  'users',
  'audit-log'
]

// Te trasy są dodatkowo za RequireAdmin — klucz na koncie zwykłego użytkownika
// dostanie 403 mimo nadanego uprawnienia.
const adminOnlyResources = new Set(['users', 'audit-log'])

function resourceOf(scope: string) {
  return scope.split(':')[0] ?? scope
}

function actionOf(scope: string) {
  return scope.split(':')[1] ?? ''
}

function resourceLabel(resource: string) {
  return resourceLabels[resource] ?? resource
}

function scopeLabel(scope: string) {
  const action = actionOf(scope) === 'write' ? 'zapis' : 'odczyt'
  return `${resourceLabel(resourceOf(scope))} — ${action}`
}

// Katalog uprawnień przychodzi z serwera jako płaska lista; grupujemy ją po
// zasobie, żeby formularz nie był ścianą kilkunastu checkboxów.
const scopeGroups = computed(() => {
  const groups = new Map<string, { read: string | null, write: string | null }>()

  for (const scope of availableScopes.value) {
    const resource = resourceOf(scope)
    const group = groups.get(resource) ?? { read: null, write: null }

    if (actionOf(scope) === 'write') {
      group.write = scope
    } else {
      group.read = scope
    }

    groups.set(resource, group)
  }

  return [...groups.entries()]
    .map(([resource, actions]) => ({
      resource,
      label: resourceLabel(resource),
      adminOnly: adminOnlyResources.has(resource),
      ...actions
    }))
    .sort((a, b) => {
      // Zakres spoza znanej kolejności (dodany po stronie serwera) ląduje na
      // końcu zamiast znikać z formularza.
      const aIndex = resourceOrder.indexOf(a.resource)
      const bIndex = resourceOrder.indexOf(b.resource)
      const aRank = aIndex === -1 ? resourceOrder.length : aIndex
      const bRank = bIndex === -1 ? resourceOrder.length : bIndex

      return aRank - bRank || a.label.localeCompare(b.label, 'pl')
    })
})

const normalizedSearch = computed(() => search.value.trim().toLowerCase())

const filteredKeys = computed(() => {
  if (!normalizedSearch.value) {
    return keys.value
  }

  return keys.value.filter((key) => {
    const haystack = [key.name, key.prefix, key.userEmail, key.userName, ...key.scopes]
      .join(' ')
      .toLowerCase()

    return haystack.includes(normalizedSearch.value)
  })
})

function isExpired(key: ApiKey) {
  if (!key.expiresAt) {
    return false
  }

  const parsed = new Date(key.expiresAt)
  return !Number.isNaN(parsed.getTime()) && parsed.getTime() < Date.now()
}

function keyStatus(key: ApiKey) {
  if (key.revokedAt) {
    return { label: 'Unieważniony', class: 'border-red-200 bg-red-50 text-red-700' }
  }

  if (isExpired(key)) {
    return { label: 'Wygasły', class: 'border-amber-200 bg-amber-50 text-amber-700' }
  }

  return { label: 'Aktywny', class: 'border-emerald-200 bg-emerald-50 text-emerald-700' }
}

const activeKeysCount = computed(() => keys.value.filter(key => !key.revokedAt && !isExpired(key)).length)
const expiredKeysCount = computed(() => keys.value.filter(key => !key.revokedAt && isExpired(key)).length)
const revokedKeysCount = computed(() => keys.value.filter(key => !!key.revokedAt).length)

const selectedUser = computed(() => users.value.find(user => user.id === form.userId) ?? null)
const grantsAdminAccess = computed(() => selectedUser.value?.role === 1)

const requiredFormComplete = computed(() => {
  return !!(form.name.trim() && form.userId > 0 && form.scopes.length > 0)
})

// Zapis obejmuje odczyt (tak samo liczy to serwer), więc zaznaczenie zapisu
// blokuje i domyślnie zaznacza odczyt zamiast wysyłać oba uprawnienia.
function isReadImplied(group: { write: string | null }) {
  return !!group.write && form.scopes.includes(group.write)
}

function isScopeChecked(scope: string | null, group: { write: string | null }) {
  if (!scope) {
    return false
  }

  return form.scopes.includes(scope) || (actionOf(scope) === 'read' && isReadImplied(group))
}

function toggleScope(scope: string | null) {
  if (!scope) {
    return
  }

  const index = form.scopes.indexOf(scope)
  if (index === -1) {
    form.scopes.push(scope)
    return
  }

  form.scopes.splice(index, 1)
}

function onToggleWrite(group: { read: string | null, write: string | null }) {
  toggleScope(group.write)

  // Odczyt staje się nadmiarowy w chwili nadania zapisu.
  if (group.read && group.write && form.scopes.includes(group.write)) {
    const index = form.scopes.indexOf(group.read)
    if (index !== -1) {
      form.scopes.splice(index, 1)
    }
  }
}

function formatDateTime(value: string | null) {
  if (!value) {
    return null
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat('pl-PL', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(parsed)
}

function resetForm() {
  form.name = ''
  form.userId = 0
  form.scopes = []
  form.expiresAt = ''
}

async function onCopyToken() {
  if (!issuedKey.value) {
    return
  }

  try {
    await navigator.clipboard.writeText(issuedKey.value.token)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2500)
  } catch {
    // Schowek bywa niedostępny (brak HTTPS, odmowa uprawnień) — klucz jest
    // widoczny w polu obok, więc wystarczy go zaznaczyć ręcznie.
    copied.value = false
  }
}

async function onCreateKey() {
  createError.value = ''

  if (!requiredFormComplete.value) {
    createError.value = 'Podaj nazwę, wybierz konto i zaznacz co najmniej jedno uprawnienie.'
    return
  }

  submitPending.value = true

  try {
    const response = await api.createApiKey({
      name: form.name.trim(),
      userId: form.userId,
      scopes: [...form.scopes],
      expiresAt: form.expiresAt
    })

    issuedKey.value = { token: response.token, name: response.data.name }
    resetForm()
    await refresh()
  } catch (apiError) {
    createError.value = getApiErrorMessage(apiError, 'Nie udało się utworzyć klucza.')
  } finally {
    submitPending.value = false
  }
}

async function onRevokeKey(key: ApiKey) {
  revokeError.value = ''

  if (!window.confirm(`Unieważnić klucz „${key.name}"? Integracja korzystająca z niego przestanie działać natychmiast.`)) {
    return
  }

  revokePendingId.value = key.id

  try {
    await api.revokeApiKey(key.id)
    await refresh()
  } catch (apiError) {
    revokeError.value = getApiErrorMessage(apiError, 'Nie udało się unieważnić klucza.')
  } finally {
    revokePendingId.value = null
  }
}
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Administracja
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Klucze API
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Klucze pozwalają zewnętrznym programom korzystać z API bez logowania. Każdy klucz działa
          w imieniu wybranego konta i tylko w zakresie nadanych uprawnień.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending"
          @click="refresh()"
        >
          Odśwież
        </UButton>
      </div>
    </div>

    <div
      v-if="issuedKey"
      class="rounded-xl border-2 border-amber-300 bg-amber-50 p-6 shadow-sm"
    >
      <div class="flex items-start gap-3">
        <UIcon
          name="i-lucide-key-round"
          class="mt-1 shrink-0 text-xl text-amber-600"
        />

        <div class="min-w-0 flex-1 space-y-3">
          <div>
            <h2 class="text-lg font-semibold text-amber-900">
              Klucz „{{ issuedKey.name }}" został utworzony
            </h2>
            <p class="mt-1 text-sm leading-6 text-amber-800">
              Skopiuj go teraz i przekaż bezpiecznym kanałem. W systemie zapisany jest wyłącznie
              jego skrót — po zamknięciu tego komunikatu nie da się go odczytać ponownie.
              Jeśli zaginie, wystarczy unieważnić klucz i wystawić nowy.
            </p>
          </div>

          <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
            <input
              :value="issuedKey.token"
              readonly
              class="w-full min-w-0 rounded-md border border-amber-300 bg-white px-4 py-3 font-mono text-sm text-slate-900 outline-none"
              @focus="($event.target as HTMLInputElement).select()"
            >

            <button
              type="button"
              class="inline-flex shrink-0 items-center justify-center gap-2 rounded-lg bg-amber-600 px-4 py-3 text-sm font-medium text-white shadow-sm transition hover:bg-amber-700"
              @click="onCopyToken"
            >
              <UIcon :name="copied ? 'i-lucide-check' : 'i-lucide-copy'" />
              {{ copied ? 'Skopiowano' : 'Kopiuj' }}
            </button>
          </div>

          <button
            type="button"
            class="text-sm font-medium text-amber-800 underline underline-offset-4 transition hover:text-amber-900"
            @click="issuedKey = null"
          >
            Mam zapisany klucz — zamknij
          </button>
        </div>
      </div>
    </div>

    <div class="grid gap-4 md:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Aktywne klucze</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ activeKeysCount }}
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Wygasłe</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ expiredKeysCount }}
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Unieważnione</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ revokedKeysCount }}
        </p>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_25rem]">
      <div class="space-y-5">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Szukaj klucza</span>
            <input
              v-model="search"
              type="text"
              placeholder="Np. nazwa integracji, clk_ab12 lub konto serwisowe"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
            >
          </label>
        </div>

        <div
          v-if="revokeError"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          {{ revokeError }}
        </div>

        <div
          v-if="error"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          Nie udało się pobrać listy kluczy API.
        </div>

        <div
          v-else-if="pending"
          class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
        >
          Ładowanie kluczy...
        </div>

        <div
          v-else-if="keys.length === 0"
          class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm leading-6 text-slate-500"
        >
          Nie wystawiono jeszcze żadnego klucza. Użyj formularza obok, aby udostępnić API
          zewnętrznemu programowi.
        </div>

        <div
          v-else-if="filteredKeys.length === 0"
          class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm text-slate-500"
        >
          Brak kluczy pasujących do podanej frazy.
        </div>

        <div
          v-else
          class="grid gap-4"
        >
          <article
            v-for="key in filteredKeys"
            :key="key.id"
            class="grid gap-4 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm md:grid-cols-[minmax(0,1fr)_13rem]"
            :class="key.revokedAt ? 'opacity-70' : ''"
          >
            <div class="space-y-3">
              <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-400">
                <span>ID {{ key.id }}</span>
                <span>•</span>
                <span class="font-mono normal-case tracking-normal">{{ key.prefix }}…</span>
              </div>

              <div>
                <h2 class="text-lg font-semibold text-slate-900">
                  {{ key.name }}
                </h2>
                <p class="mt-1 text-sm text-slate-500">
                  Działa jako {{ key.userName || 'konto' }}
                  <span v-if="key.userEmail">({{ key.userEmail }})</span>
                </p>
              </div>

              <div class="flex flex-wrap gap-2">
                <span
                  class="inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium"
                  :class="keyStatus(key).class"
                >
                  {{ keyStatus(key).label }}
                </span>

                <span
                  v-for="scope in key.scopes"
                  :key="scope"
                  class="inline-flex items-center rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-600"
                >
                  {{ scopeLabel(scope) }}
                </span>
              </div>

              <dl class="grid gap-x-6 gap-y-1 text-xs text-slate-500 sm:grid-cols-3">
                <div>
                  <dt class="inline">Utworzono: </dt>
                  <dd class="inline">{{ formatDateTime(key.createdAt) ?? '—' }}</dd>
                </div>
                <div>
                  <dt class="inline">Wygasa: </dt>
                  <dd class="inline">{{ formatDateTime(key.expiresAt) ?? 'bezterminowo' }}</dd>
                </div>
                <div>
                  <dt class="inline">Ostatnie użycie: </dt>
                  <dd class="inline">{{ formatDateTime(key.lastUsedAt) ?? 'nigdy' }}</dd>
                </div>
              </dl>
            </div>

            <div class="flex flex-col items-start gap-3 md:items-end">
              <button
                v-if="!key.revokedAt"
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm font-medium text-red-700 transition hover:border-red-300 hover:bg-red-100 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="revokePendingId === key.id"
                @click="onRevokeKey(key)"
              >
                {{ revokePendingId === key.id ? 'Unieważnianie...' : 'Unieważnij klucz' }}
              </button>

              <p
                v-else
                class="text-right text-xs leading-5 text-slate-500"
              >
                Unieważniony {{ formatDateTime(key.revokedAt) }}
              </p>
            </div>
          </article>
        </div>
      </div>

      <aside class="space-y-4 lg:sticky lg:top-24 lg:self-start">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
              Nowy klucz
            </p>
            <h2 class="text-xl font-semibold tracking-tight text-slate-900">
              Wystaw klucz API
            </h2>
            <p class="text-sm leading-6 text-slate-600">
              Klucz zobaczysz tylko raz, zaraz po utworzeniu.
            </p>
          </div>

          <form
            class="mt-6 space-y-4"
            novalidate
            @submit.prevent="onCreateKey"
          >
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Nazwa</span>
              <input
                v-model="form.name"
                type="text"
                placeholder="Np. integracja kadrowa"
                required
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
              <span class="block text-xs leading-5 text-slate-500">
                Po nazwie rozpoznasz klucz na liście — wpisz, do czego służy.
              </span>
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Konto, w imieniu którego działa klucz</span>
              <div class="relative">
                <select
                  v-model="form.userId"
                  class="h-[50px] w-full appearance-none rounded-md border border-slate-300 bg-white px-4 py-3 pr-11 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
                  <option :value="0">
                    Wybierz konto...
                  </option>
                  <option
                    v-for="user in users"
                    :key="user.id"
                    :value="user.id"
                  >
                    {{ user.firstName }} {{ user.lastName }} ({{ user.email }})
                  </option>
                </select>

                <UIcon
                  name="i-lucide-chevron-down"
                  class="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-slate-400"
                />
              </div>
              <span class="block text-xs leading-5 text-slate-500">
                Operacje wykonane kluczem zapiszą się w historii zmian na tym koncie.
              </span>
            </label>

            <div
              v-if="grantsAdminAccess"
              class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-800"
            >
              To konto ma uprawnienia administratora, więc klucz sięgnie także po operacje
              administracyjne objęte zaznaczonymi uprawnieniami. Dla integracji bezpieczniej
              jest założyć osobne konto zwykłego użytkownika.
            </div>

            <fieldset class="space-y-3">
              <legend class="text-sm font-medium text-slate-700">
                Uprawnienia
              </legend>

              <div class="space-y-2 rounded-lg border border-slate-200 p-3">
                <div
                  v-for="group in scopeGroups"
                  :key="group.resource"
                  class="flex items-center justify-between gap-3 py-1"
                >
                  <span class="min-w-0 text-sm text-slate-700">
                    {{ group.label }}
                    <span
                      v-if="group.adminOnly"
                      class="block text-xs leading-4 text-slate-400"
                    >
                      tylko konto administratora
                    </span>
                  </span>

                  <div class="flex shrink-0 items-center gap-3">
                    <label
                      v-if="group.read"
                      class="flex items-center gap-1.5 text-xs text-slate-600"
                      :title="isReadImplied(group) ? 'Zapis obejmuje odczyt' : undefined"
                    >
                      <input
                        type="checkbox"
                        class="size-4 rounded border-slate-300 text-sky-600 focus:ring-sky-500 disabled:opacity-60"
                        :checked="isScopeChecked(group.read, group)"
                        :disabled="isReadImplied(group)"
                        @change="toggleScope(group.read)"
                      >
                      odczyt
                    </label>

                    <label
                      v-if="group.write"
                      class="flex items-center gap-1.5 text-xs text-slate-600"
                    >
                      <input
                        type="checkbox"
                        class="size-4 rounded border-slate-300 text-sky-600 focus:ring-sky-500"
                        :checked="isScopeChecked(group.write, group)"
                        @change="onToggleWrite(group)"
                      >
                      zapis
                    </label>

                    <span
                      v-else
                      class="w-[3.25rem] text-right text-xs text-slate-300"
                    >—</span>
                  </div>
                </div>
              </div>

              <p class="text-xs leading-5 text-slate-500">
                Nadaj tylko to, czego integracja naprawdę potrzebuje. Zapis obejmuje odczyt.
              </p>
            </fieldset>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Wygasa (opcjonalnie)</span>
              <input
                v-model="form.expiresAt"
                type="date"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
              <span class="block text-xs leading-5 text-slate-500">
                Klucz działa przez cały wskazany dzień. Puste pole oznacza klucz bezterminowy.
              </span>
            </label>

            <div
              v-if="createError"
              class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              {{ createError }}
            </div>

            <div class="flex flex-wrap items-center gap-3 pt-2">
              <button
                type="submit"
                class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-slate-300"
                :disabled="submitPending"
              >
                {{ submitPending ? 'Tworzenie...' : 'Wystaw klucz' }}
              </button>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                :disabled="submitPending"
                @click="resetForm()"
              >
                Wyczyść
              </button>
            </div>
          </form>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-50 p-6 text-sm leading-6 text-slate-600">
          <h3 class="font-medium text-slate-900">
            Jak używać klucza
          </h3>
          <p class="mt-2">
            Program powinien wysyłać klucz w nagłówku HTTP:
          </p>
          <code class="mt-2 block overflow-x-auto rounded-md border border-slate-200 bg-white px-3 py-2 font-mono text-xs text-slate-800">
            Authorization: Bearer clk_...
          </code>
          <p class="mt-3 text-xs leading-5 text-slate-500">
            Klucz nie pozwala zarządzać kontami ani innymi kluczami — te operacje wymagają
            zalogowania w przeglądarce.
          </p>
        </div>
      </aside>
    </div>
  </section>
</template>
