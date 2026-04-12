<script setup lang="ts">
import type { AdminUser } from '~/composables/useApi'

definePageMeta({
  middleware: 'admin'
})

useSeoMeta({
  title: 'Administracja użytkowników'
})

const api = useApi()
const auth = useAuth()

const search = ref('')
const form = reactive({
  email: '',
  password: '',
  firstName: '',
  lastName: '',
  role: 2
})

const submitPending = ref(false)
const deletePendingId = ref<number | null>(null)
const createError = ref('')
const createSuccess = ref('')
const deleteError = ref('')

const { data, pending, error, refresh } = await useAsyncData('admin-users', async () => {
  return await api.users()
})

const users = computed(() => data.value?.data ?? [])
const normalizedSearch = computed(() => search.value.trim().toLowerCase())
const currentUserId = computed(() => auth.user.value?.id ?? null)
const adminUsersCount = computed(() => users.value.filter(user => user.role === 1).length)

const filteredUsers = computed(() => {
  if (!normalizedSearch.value) {
    return users.value
  }

  return users.value.filter((user) => {
    const haystack = [
      user.email,
      user.firstName,
      user.lastName,
      roleLabel(user.role)
    ]
      .join(' ')
      .toLowerCase()

    return haystack.includes(normalizedSearch.value)
  })
})

const requiredFormComplete = computed(() => {
  return !!(
    form.email.trim()
    && form.password.trim()
    && form.firstName.trim()
    && form.lastName.trim()
    && form.role > 0
  )
})

const totalUsers = computed(() => users.value.length)
const regularUsersCount = computed(() => users.value.filter(user => user.role !== 1).length)

function roleLabel(role: number) {
  return role === 1 ? 'Administrator' : 'Użytkownik'
}

function roleBadgeClass(role: number) {
  if (role === 1) {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700'
  }

  return 'border-slate-200 bg-slate-50 text-slate-600'
}

function resetForm() {
  form.email = ''
  form.password = ''
  form.firstName = ''
  form.lastName = ''
  form.role = 2
}

function canDeleteUser(user: AdminUser) {
  if (currentUserId.value === user.id) {
    return false
  }

  if (user.role === 1 && adminUsersCount.value <= 1) {
    return false
  }

  return true
}

function deleteDisabledReason(user: AdminUser) {
  if (currentUserId.value === user.id) {
    return 'Nie możesz usunąć własnego konta.'
  }

  if (user.role === 1 && adminUsersCount.value <= 1) {
    return 'Nie można usunąć ostatniego administratora.'
  }

  return ''
}

async function onCreateUser() {
  createError.value = ''
  createSuccess.value = ''

  if (!requiredFormComplete.value) {
    createError.value = 'Uzupełnij email, hasło, imię, nazwisko i rolę.'
    return
  }

  submitPending.value = true

  try {
    const response = await api.createUser({
      email: form.email.trim(),
      password: form.password.trim(),
      firstName: form.firstName.trim(),
      lastName: form.lastName.trim(),
      role: form.role
    })

    resetForm()
    createSuccess.value = `Dodano użytkownika ${response.data.firstName} ${response.data.lastName}.`
    await refresh()
  } catch (apiError) {
    createError.value = getApiErrorMessage(apiError, 'Nie udało się utworzyć użytkownika.')
  } finally {
    submitPending.value = false
  }
}

async function onDeleteUser(user: AdminUser) {
  deleteError.value = ''
  createSuccess.value = ''

  if (!canDeleteUser(user)) {
    deleteError.value = deleteDisabledReason(user)
    return
  }

  if (!window.confirm(`Czy na pewno usunąć użytkownika ${user.firstName} ${user.lastName}?`)) {
    return
  }

  deletePendingId.value = user.id

  try {
    await api.deleteUser(user.id)
    await refresh()
  } catch (apiError) {
    deleteError.value = getApiErrorMessage(apiError, 'Nie udało się usunąć użytkownika.')
  } finally {
    deletePendingId.value = null
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
          Użytkownicy systemu
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Dodawaj nowych operatorów i administratorów oraz usuwaj konta, które nie powinny już mieć dostępu.
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

    <div class="grid gap-4 md:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Wszyscy użytkownicy</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ totalUsers }}
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Administratorzy</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ adminUsersCount }}
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Pozostali użytkownicy</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ regularUsersCount }}
        </p>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_23rem]">
      <div class="space-y-5">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Szukaj użytkownika</span>
            <input
              v-model="search"
              type="text"
              placeholder="Np. admin@example.com, Jan lub Administrator"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
            >
          </label>
        </div>

        <div
          v-if="deleteError"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          {{ deleteError }}
        </div>

        <div
          v-if="error"
          class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
        >
          Nie udało się pobrać listy użytkowników.
        </div>

        <div
          v-else-if="pending"
          class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
        >
          Ładowanie użytkowników...
        </div>

        <div
          v-else-if="filteredUsers.length === 0"
          class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm text-slate-500"
        >
          Brak użytkowników pasujących do podanej frazy.
        </div>

        <div
          v-else
          class="grid gap-4"
        >
          <article
            v-for="user in filteredUsers"
            :key="user.id"
            class="grid gap-4 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm md:grid-cols-[minmax(0,1fr)_13rem]"
          >
            <div class="space-y-3">
              <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-400">
                <span>ID {{ user.id }}</span>
                <span>•</span>
                <span>{{ user.email }}</span>
              </div>

              <div>
                <h2 class="text-lg font-semibold text-slate-900">
                  {{ user.firstName }} {{ user.lastName }}
                </h2>
                <p class="mt-1 text-sm text-slate-500">
                  {{ user.email }}
                </p>
              </div>

              <div class="flex flex-wrap gap-2">
                <span
                  class="inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium"
                  :class="roleBadgeClass(user.role)"
                >
                  {{ roleLabel(user.role) }}
                </span>

                <span
                  v-if="currentUserId === user.id"
                  class="inline-flex items-center rounded-full border border-sky-200 bg-sky-50 px-3 py-1 text-xs font-medium text-sky-700"
                >
                  Twoje konto
                </span>
              </div>
            </div>

            <div class="flex flex-col items-start gap-3 md:items-end">
              <NuxtLink
                :to="`/admin/users/${user.id}/edit`"
                class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              >
                Edytuj
              </NuxtLink>

              <button
                type="button"
                class="inline-flex items-center justify-center rounded-lg border px-4 py-2 text-sm font-medium transition"
                :class="canDeleteUser(user)
                  ? 'border-red-200 bg-red-50 text-red-700 hover:border-red-300 hover:bg-red-100'
                  : 'cursor-not-allowed border-slate-200 bg-slate-100 text-slate-400'"
                :disabled="!canDeleteUser(user) || deletePendingId === user.id"
                @click="onDeleteUser(user)"
              >
                {{ deletePendingId === user.id ? 'Usuwanie...' : 'Usuń użytkownika' }}
              </button>

              <p
                v-if="deleteDisabledReason(user)"
                class="max-w-48 text-right text-xs leading-5 text-slate-500"
              >
                {{ deleteDisabledReason(user) }}
              </p>
            </div>
          </article>
        </div>
      </div>

      <aside class="space-y-4 lg:sticky lg:top-24 lg:self-start">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
              Nowy użytkownik
            </p>
            <h2 class="text-xl font-semibold tracking-tight text-slate-900">
              Dodaj konto
            </h2>
            <p class="text-sm leading-6 text-slate-600">
              Nowe konto zacznie działać od razu po zapisaniu.
            </p>
          </div>

          <form
            class="mt-6 space-y-4"
            novalidate
            :data-show-validation="createError ? 'true' : null"
            @submit.prevent="onCreateUser"
          >
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Email</span>
              <input
                v-model="form.email"
                type="email"
                autocomplete="email"
                required
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Hasło</span>
              <input
                v-model="form.password"
                type="password"
                autocomplete="new-password"
                required
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Imię</span>
                <input
                  v-model="form.firstName"
                  type="text"
                  autocomplete="given-name"
                  required
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>

              <label class="block space-y-2">
                <span class="text-sm font-medium text-slate-700">Nazwisko</span>
                <input
                  v-model="form.lastName"
                  type="text"
                  autocomplete="family-name"
                  required
                  class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
              </label>
            </div>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Rola</span>
              <div class="relative">
                <select
                  v-model="form.role"
                  :data-manual-invalid="createError && form.role <= 0 ? 'true' : null"
                  class="h-[50px] w-full appearance-none rounded-md border border-slate-300 bg-white px-4 py-3 pr-11 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                >
                  <option :value="2">
                    Użytkownik
                  </option>
                  <option :value="1">
                    Administrator
                  </option>
                </select>

                <UIcon
                  name="i-lucide-chevron-down"
                  class="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-slate-400"
                />
              </div>
            </label>

            <div
              v-if="createError"
              class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            >
              {{ createError }}
            </div>

            <div
              v-if="createSuccess"
              class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
            >
              {{ createSuccess }}
            </div>

            <div class="flex flex-wrap items-center gap-3 pt-2">
              <button
                type="submit"
                class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-slate-300"
                :disabled="submitPending"
              >
                {{ submitPending ? 'Zapisywanie...' : 'Dodaj użytkownika' }}
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
      </aside>
    </div>
  </section>
</template>
