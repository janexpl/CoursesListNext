<script setup lang="ts">
definePageMeta({
  middleware: 'guest'
})

useSeoMeta({
  title: 'Logowanie'
})

const auth = useAuth()
const route = useRoute()

const form = reactive({
  email: '',
  password: ''
})

const pending = ref(false)
const errorMessage = ref('')
const successMessage = computed(() => {
  if (route.query.passwordChanged === '1') {
    return 'Hasło zostało zmienione. Zaloguj się ponownie.'
  }

  return ''
})

async function onSubmit() {
  errorMessage.value = ''
  pending.value = true

  try {
    await auth.login({
      email: form.email,
      password: form.password
    })

    await navigateTo('/')
  } catch (error) {
    errorMessage.value = getApiErrorMessage(error, 'Nie udało się zalogować.')
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="grid min-h-[calc(100vh-10rem)] place-items-center">
    <section
      class="w-full max-w-md rounded-xl border border-white/70 bg-white/92 p-8 shadow-lg backdrop-blur"
    >
      <div class="space-y-2 border-b border-slate-200 pb-6">
        <p class="text-sm uppercase tracking-[0.2em] text-sky-700">
          CoursesList
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-950">
          Logowanie
        </h1>
        <p class="text-sm leading-6 text-slate-500">
          Zaloguj się do nowego panelu operacyjnego.
        </p>
      </div>

      <div class="pt-6">
        <div class="mb-8">
          <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
            Logowanie
          </p>
          <h2 class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            Zaloguj się do panelu
          </h2>
          <p class="mt-2 text-sm leading-6 text-slate-500">
            Użyj danych z tabeli <code>users</code> z obecnej aplikacji.
          </p>
        </div>

        <form
          class="space-y-5"
          @submit.prevent="onSubmit"
        >
          <p
            v-if="successMessage"
            class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
          >
            {{ successMessage }}
          </p>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">E-mail</span>
            <input
              v-model="form.email"
              type="email"
              autocomplete="email"
              required
              class="w-full rounded-lg border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="jan@example.com"
            >
          </label>

          <label class="block space-y-2">
            <span class="text-sm font-medium text-slate-700">Hasło</span>
            <input
              v-model="form.password"
              type="password"
              autocomplete="current-password"
              required
              class="w-full rounded-lg border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
              placeholder="••••••••"
            >
          </label>

          <p
            v-if="errorMessage"
            class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          >
            {{ errorMessage }}
          </p>

          <button
            type="submit"
            :disabled="pending"
            class="inline-flex w-full items-center justify-center rounded-lg bg-sky-600 px-4 py-3 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700 disabled:cursor-not-allowed disabled:bg-sky-300"
          >
            Zaloguj się
          </button>
        </form>
      </div>
    </section>
  </div>
</template>
