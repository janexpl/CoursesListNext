<script setup lang="ts">
import AuditHistoryPanel from '~/components/audit/AuditHistoryPanel.vue'

definePageMeta({
  middleware: 'admin'
})

const route = useRoute()
const api = useApi()
const auth = useAuth()

const userId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(userId.value) || userId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono użytkownika'
  })
}

const { data, pending, error, refresh } = await useAsyncData(
  `admin-user-edit:${userId.value}`,
  async () => {
    const response = await api.users()
    return response.data.find(user => user.id === userId.value) ?? null
  }
)

const {
  data: auditData,
  pending: auditPending,
  error: auditError,
  refresh: refreshAudit
} = await useAsyncData(
  `admin-user-audit:${userId.value}`,
  async () => await api.userAuditLog(userId.value)
)

const user = computed(() => data.value ?? null)
const auditEntries = computed(() => auditData.value?.data ?? [])
const auditErrorMessage = computed(() => {
  return auditError.value ? getApiErrorMessage(auditError.value, 'Nie udało się pobrać historii zmian użytkownika.') : ''
})
const detailsLink = computed(() => '/admin/users')

const form = reactive({
  email: '',
  firstName: '',
  lastName: '',
  role: 2
})

const passwordForm = reactive({
  newPassword: '',
  confirmPassword: ''
})

const submitPending = ref(false)
const passwordPending = ref(false)
const errorMessage = ref('')
const passwordErrorMessage = ref('')
const passwordSuccessMessage = ref('')
const isInitialized = ref(false)

watchEffect(() => {
  if (!user.value || isInitialized.value) {
    return
  }

  form.email = user.value.email || ''
  form.firstName = user.value.firstName || ''
  form.lastName = user.value.lastName || ''
  form.role = user.value.role || 2
  isInitialized.value = true
})

const trimmedEmail = computed(() => form.email.trim())
const trimmedFirstName = computed(() => form.firstName.trim())
const trimmedLastName = computed(() => form.lastName.trim())

const isDirty = computed(() => {
  if (!user.value) {
    return false
  }

  return (
    trimmedEmail.value !== user.value.email
    || trimmedFirstName.value !== user.value.firstName
    || trimmedLastName.value !== user.value.lastName
    || form.role !== user.value.role
  )
})

const hasPasswordInput = computed(() => {
  return !!passwordForm.newPassword || !!passwordForm.confirmPassword
})

const currentUserId = computed(() => auth.user.value?.id ?? null)
const isCurrentUser = computed(() => currentUserId.value === user.value?.id)

const passwordStrengthHint = computed(() => {
  const length = passwordForm.newPassword.length

  if (!length) {
    return 'Nowe hasło powinno mieć co najmniej 8 znaków.'
  }

  if (length < 8) {
    return 'Hasło jest za krótkie.'
  }

  if (length < 12) {
    return 'Hasło jest poprawne, ale warto użyć dłuższego.'
  }

  return 'Długość hasła wygląda dobrze.'
})

async function onSubmit() {
  errorMessage.value = ''

  if (!trimmedEmail.value || !trimmedFirstName.value || !trimmedLastName.value || form.role <= 0) {
    errorMessage.value = 'Uzupełnij email, imię, nazwisko i rolę.'
    return
  }

  submitPending.value = true

  try {
    await api.updateUser(userId.value, {
      email: trimmedEmail.value,
      firstName: trimmedFirstName.value,
      lastName: trimmedLastName.value,
      role: form.role
    })

    await navigateTo(detailsLink.value)
  } catch (apiError) {
    errorMessage.value = getApiErrorMessage(apiError, 'Nie udało się zapisać zmian użytkownika.')
  } finally {
    submitPending.value = false
  }
}

async function onResetPassword() {
  passwordErrorMessage.value = ''
  passwordSuccessMessage.value = ''

  if (isCurrentUser.value) {
    passwordErrorMessage.value = 'Własne hasło zmień w sekcji „Moje konto”.'
    return
  }

  if (!passwordForm.newPassword || !passwordForm.confirmPassword) {
    passwordErrorMessage.value = 'Uzupełnij oba pola nowego hasła.'
    return
  }

  if (passwordForm.newPassword.length < 8) {
    passwordErrorMessage.value = 'Nowe hasło musi mieć co najmniej 8 znaków.'
    return
  }

  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    passwordErrorMessage.value = 'Nowe hasło i potwierdzenie muszą być identyczne.'
    return
  }

  passwordPending.value = true

  try {
    await api.resetUserPassword(userId.value, {
      newPassword: passwordForm.newPassword
    })

    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
    passwordSuccessMessage.value = 'Hasło zostało zmienione. Wszystkie sesje użytkownika zostały unieważnione.'
  } catch (apiError) {
    passwordErrorMessage.value = getApiErrorMessage(apiError, 'Nie udało się zmienić hasła użytkownika.')
  } finally {
    passwordPending.value = false
  }
}

function resetForm() {
  if (!user.value) {
    return
  }

  form.email = user.value.email
  form.firstName = user.value.firstName
  form.lastName = user.value.lastName
  form.role = user.value.role
  errorMessage.value = ''
}

function resetPasswordForm() {
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordErrorMessage.value = ''
  passwordSuccessMessage.value = ''
}

useUnsavedChangesWarning(() => {
  return (isDirty.value && !submitPending.value)
    || (hasPasswordInput.value && !passwordPending.value)
})

useSeoMeta({
  title: () => user.value ? `Edycja: ${user.value.firstName} ${user.value.lastName}` : 'Edycja użytkownika'
})

async function refreshAll() {
  await Promise.all([refresh(), refreshAudit()])
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
          Edycja użytkownika
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Zaktualizuj dane konta i rolę użytkownika. Zmiana hasła pozostaje osobną operacją.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending || auditPending"
          @click="refreshAll()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          to="/admin/users"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Wróć do listy
        </NuxtLink>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać danych użytkownika.
    </div>

    <div
      v-else-if="pending || !user"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie danych użytkownika...
    </div>

    <template v-else>
      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <div class="grid gap-4 md:grid-cols-4">
          <div>
            <p class="text-sm text-slate-500">
              ID
            </p>
            <p class="mt-2 text-lg font-semibold text-slate-900">
              {{ user.id }}
            </p>
          </div>

          <div class="md:col-span-2">
            <p class="text-sm text-slate-500">
              Aktualny email
            </p>
            <p class="mt-2 break-all text-lg font-semibold text-slate-900">
              {{ user.email }}
            </p>
          </div>

          <div>
            <p class="text-sm text-slate-500">
              Rola
            </p>
            <p class="mt-2 text-lg font-semibold text-slate-900">
              {{ user.role === 1 ? 'Administrator' : 'Użytkownik' }}
            </p>
          </div>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
        <form
          class="space-y-6 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm"
          @submit.prevent="onSubmit"
        >
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
              Formularz edycji
            </p>
            <h2 class="text-xl font-semibold tracking-tight text-slate-900">
              Dane użytkownika
            </h2>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <label class="block space-y-2 md:col-span-2">
              <span class="text-sm font-medium text-slate-700">Email</span>
              <input
                v-model="form.email"
                type="email"
                autocomplete="email"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Imię</span>
              <input
                v-model="form.firstName"
                type="text"
                autocomplete="given-name"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Nazwisko</span>
              <input
                v-model="form.lastName"
                type="text"
                autocomplete="family-name"
                class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              >
            </label>

            <label class="block space-y-2 md:col-span-2">
              <span class="text-sm font-medium text-slate-700">Rola</span>
              <div class="relative">
                <select
                  v-model="form.role"
                  class="h-12.5 w-full appearance-none rounded-md border border-slate-300 bg-white px-4 py-3 pr-11 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
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
          </div>

          <div
            v-if="errorMessage"
            class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ errorMessage }}
          </div>

          <div class="flex flex-wrap items-center gap-3 pt-2">
            <button
              type="submit"
              class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-slate-300"
              :disabled="submitPending || !isDirty"
            >
              {{ submitPending ? 'Zapisywanie...' : 'Zapisz zmiany' }}
            </button>

            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              :disabled="submitPending || !isDirty"
              @click="resetForm()"
            >
              Przywróć
            </button>
          </div>
        </form>

        <form
          class="space-y-6 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm"
          @submit.prevent="onResetPassword"
        >
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
              Bezpieczeństwo
            </p>
            <h2 class="text-xl font-semibold tracking-tight text-slate-900">
              Reset hasła
            </h2>
            <p class="text-sm leading-6 text-slate-600">
              Ustaw nowe hasło dla tego użytkownika. Wszystkie aktywne sesje tego konta zostaną unieważnione.
            </p>
          </div>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Nowe hasło</span>
            <input
              v-model="passwordForm.newPassword"
              type="password"
              autocomplete="new-password"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="Minimum 8 znaków"
            >
          </label>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Potwierdź nowe hasło</span>
            <input
              v-model="passwordForm.confirmPassword"
              type="password"
              autocomplete="new-password"
              class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="Powtórz nowe hasło"
            >
          </label>

          <p
            class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600"
            :class="passwordForm.newPassword.length >= 8 ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : ''"
          >
            {{ passwordStrengthHint }}
          </p>

          <p
            v-if="isCurrentUser"
            class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700"
          >
            Własne hasło zmień w sekcji „Moje konto”.
          </p>

          <div
            v-if="passwordErrorMessage"
            class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ passwordErrorMessage }}
          </div>

          <div
            v-if="passwordSuccessMessage"
            class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
          >
            {{ passwordSuccessMessage }}
          </div>

          <div class="flex flex-wrap items-center gap-3 pt-2">
            <button
              type="submit"
              class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-slate-300"
              :disabled="passwordPending || isCurrentUser"
            >
              {{ passwordPending ? 'Zmiana hasła...' : 'Ustaw nowe hasło' }}
            </button>

            <button
              type="button"
              class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
              :disabled="passwordPending || !hasPasswordInput"
              @click="resetPasswordForm()"
            >
              Wyczyść
            </button>
          </div>
        </form>
      </div>

      <AuditHistoryPanel
        :entries="auditEntries"
        :pending="auditPending"
        :error-message="auditErrorMessage"
        title="Historia zmian użytkownika"
        description="Zmiany danych konta, ról i operacji bezpieczeństwa zapisane przez audit log."
        empty-message="Brak wpisów historii zmian dla tego użytkownika."
      />
    </template>
  </section>
</template>
