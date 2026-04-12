<script setup lang="ts">
definePageMeta({
  middleware: 'auth'
})

const auth = useAuth()
const api = useApi()

const profileForm = reactive({
  email: '',
  firstName: '',
  lastName: ''
})

const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const profilePending = ref(false)
const passwordPending = ref(false)
const profileErrorMessage = ref('')
const passwordErrorMessage = ref('')
const profileSuccessMessage = ref('')
const passwordSuccessMessage = ref('')
const isInitialized = ref(false)

watchEffect(() => {
  if (!auth.user.value || isInitialized.value) {
    return
  }

  profileForm.email = auth.user.value.email
  profileForm.firstName = auth.user.value.firstName
  profileForm.lastName = auth.user.value.lastName
  isInitialized.value = true
})

const trimmedEmail = computed(() => profileForm.email.trim())
const trimmedFirstName = computed(() => profileForm.firstName.trim())
const trimmedLastName = computed(() => profileForm.lastName.trim())

const hasProfileValues = computed(() => {
  return !!trimmedEmail.value && !!trimmedFirstName.value && !!trimmedLastName.value
})

const isProfileDirty = computed(() => {
  if (!auth.user.value) {
    return false
  }

  return (
    trimmedEmail.value !== auth.user.value.email
    || trimmedFirstName.value !== auth.user.value.firstName
    || trimmedLastName.value !== auth.user.value.lastName
  )
})

const hasPasswordInput = computed(() => {
  return (
    !!passwordForm.currentPassword
    || !!passwordForm.newPassword
    || !!passwordForm.confirmPassword
  )
})

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

function resetProfileForm() {
  if (!auth.user.value) {
    return
  }

  profileForm.email = auth.user.value.email
  profileForm.firstName = auth.user.value.firstName
  profileForm.lastName = auth.user.value.lastName
  profileErrorMessage.value = ''
  profileSuccessMessage.value = ''
}

function resetPasswordForm() {
  passwordForm.currentPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordErrorMessage.value = ''
  passwordSuccessMessage.value = ''
}

async function onProfileSubmit() {
  profileErrorMessage.value = ''
  profileSuccessMessage.value = ''

  if (!hasProfileValues.value) {
    profileErrorMessage.value = 'Uzupełnij wszystkie wymagane pola.'
    return
  }

  profilePending.value = true

  try {
    const response = await api.updateProfile({
      email: trimmedEmail.value,
      firstName: trimmedFirstName.value,
      lastName: trimmedLastName.value
    })

    auth.user.value = response.data
    profileSuccessMessage.value = 'Dane profilu zostały zapisane.'
  } catch (error) {
    profileErrorMessage.value = getApiErrorMessage(error, 'Nie udało się zapisać danych profilu.')
  } finally {
    profilePending.value = false
  }
}

async function onPasswordSubmit() {
  passwordErrorMessage.value = ''
  passwordSuccessMessage.value = ''

  if (!passwordForm.currentPassword || !passwordForm.newPassword || !passwordForm.confirmPassword) {
    passwordErrorMessage.value = 'Uzupełnij wszystkie pola hasła.'
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
    await api.updatePassword({
      currentPassword: passwordForm.currentPassword,
      newPassword: passwordForm.newPassword
    })

    auth.user.value = null
    await navigateTo('/login?passwordChanged=1', { replace: true })
  } catch (error) {
    passwordErrorMessage.value = getApiErrorMessage(error, 'Nie udało się zmienić hasła.')
  } finally {
    passwordPending.value = false
  }
}

useUnsavedChangesWarning(() => {
  return (
    (isProfileDirty.value && !profilePending.value)
    || (hasPasswordInput.value && !passwordPending.value)
  )
})

useSeoMeta({
  title: 'Moje konto'
})
</script>

<template>
  <section class="space-y-8">
    <div
      class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between"
    >
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Konto użytkownika
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Moje konto</h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Zmień dane profilu i hasło dostępu do aplikacji.
        </p>
      </div>

      <div
        v-if="auth.user.value"
        class="rounded-lg border border-sky-100 bg-sky-50/90 px-4 py-3 text-sm text-sky-900"
      >
        <p class="font-medium">{{ auth.user.value.firstName }} {{ auth.user.value.lastName }}</p>
        <p class="text-sky-700">
          {{ auth.user.value.email }}
        </p>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)]">
      <section
        class="space-y-5 rounded-xl border border-white/60 bg-white/90 p-8 shadow-sm backdrop-blur"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Profil</p>
            <h2 class="text-2xl font-semibold tracking-tight text-slate-900">Dane użytkownika</h2>
            <p class="text-sm leading-6 text-slate-600">
              Zaktualizuj adres e-mail oraz dane wyświetlane w panelu.
            </p>
          </div>

          <span
            class="rounded-md px-3 py-2 text-xs font-medium uppercase tracking-[0.16em]"
            :class="
              isProfileDirty ? 'bg-amber-50 text-amber-700' : 'bg-emerald-50 text-emerald-700'
            "
          >
            {{ isProfileDirty ? 'Niezapisane zmiany' : 'Brak zmian' }}
          </span>
        </div>

        <form
          class="space-y-5"
          novalidate
          :data-show-validation="profileErrorMessage ? 'true' : null"
          @submit.prevent="onProfileSubmit"
        >
          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">E-mail</span>
            <input
              v-model="profileForm.email"
              type="email"
              autocomplete="email"
              required
              class="h-12.5 w-full rounded-lg border border-slate-300 bg-white px-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="jan@example.com"
            >
          </label>

          <div class="grid gap-5 md:grid-cols-2">
            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Imię</span>
              <input
                v-model="profileForm.firstName"
                type="text"
                autocomplete="given-name"
                required
                class="h-12.5 w-full rounded-lg border border-slate-300 bg-white px-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                placeholder="Jan"
              >
            </label>

            <label class="block space-y-2">
              <span class="text-sm font-medium text-slate-700">Nazwisko</span>
              <input
                v-model="profileForm.lastName"
                type="text"
                autocomplete="family-name"
                required
                class="h-12.5 w-full rounded-lg border border-slate-300 bg-white px-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
                placeholder="Nowak"
              >
            </label>
          </div>

          <p
            v-if="profileErrorMessage"
            class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ profileErrorMessage }}
          </p>

          <p
            v-if="profileSuccessMessage"
            class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
          >
            {{ profileSuccessMessage }}
          </p>

          <div class="flex flex-wrap items-center gap-3">
            <UButton
              type="submit"
              color="neutral"
              :loading="profilePending"
              :disabled="profilePending || !isProfileDirty"
            >
              Zapisz dane
            </UButton>

            <UButton
              type="button"
              color="neutral"
              variant="outline"
              :disabled="profilePending || !isProfileDirty"
              @click="resetProfileForm"
            >
              Przywróć
            </UButton>
          </div>
        </form>
      </section>

      <section
        class="space-y-5 rounded-xl border border-white/60 bg-white/90 p-8 shadow-sm backdrop-blur"
      >
        <div class="space-y-2">
          <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Bezpieczeństwo</p>
          <h2 class="text-2xl font-semibold tracking-tight text-slate-900">Zmiana hasła</h2>
          <p class="text-sm leading-6 text-slate-600">
            Po zapisaniu hasła wszystkie aktywne sesje zostaną unieważnione i trzeba będzie
            zalogować się ponownie.
          </p>
        </div>

        <form
          class="space-y-5"
          novalidate
          :data-show-validation="passwordErrorMessage ? 'true' : null"
          @submit.prevent="onPasswordSubmit"
        >
          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Aktualne hasło</span>
            <input
              v-model="passwordForm.currentPassword"
              type="password"
              autocomplete="current-password"
              required
              class="h-[50px] w-full rounded-lg border border-slate-300 bg-white px-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="••••••••"
            >
          </label>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Nowe hasło</span>
            <input
              v-model="passwordForm.newPassword"
              type="password"
              autocomplete="new-password"
              required
              class="h-[50px] w-full rounded-lg border border-slate-300 bg-white px-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="Minimum 8 znaków"
            >
          </label>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Potwierdź nowe hasło</span>
            <input
              v-model="passwordForm.confirmPassword"
              type="password"
              autocomplete="new-password"
              required
              class="h-[50px] w-full rounded-lg border border-slate-300 bg-white px-4 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="Powtórz nowe hasło"
            >
          </label>

          <p
            class="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600"
            :class="
              passwordForm.newPassword.length >= 8
                ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
                : ''
            "
          >
            {{ passwordStrengthHint }}
          </p>

          <p
            v-if="passwordErrorMessage"
            class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ passwordErrorMessage }}
          </p>

          <p
            v-if="passwordSuccessMessage"
            class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
          >
            {{ passwordSuccessMessage }}
          </p>

          <div class="flex flex-wrap items-center gap-3">
            <UButton
              type="submit"
              color="neutral"
              :loading="passwordPending"
              :disabled="passwordPending"
            >
              Zmień hasło
            </UButton>

            <UButton
              type="button"
              color="neutral"
              variant="outline"
              :disabled="passwordPending || !hasPasswordInput"
              @click="resetPasswordForm"
            >
              Wyczyść
            </UButton>
          </div>
        </form>
      </section>
    </div>
  </section>
</template>
