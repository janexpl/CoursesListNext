<script setup lang="ts">
const route = useRoute()
const auth = useAuth()
const isMobileMenuOpen = ref(false)

const isLoginPage = computed(() => route.path === '/login')
const navigationItems = computed(() => {
  const items = [
    {
      label: 'Dashboard',
      to: '/'
    },
    {
      label: 'Kursanci',
      to: '/students'
    },
    {
      label: 'Firmy',
      to: '/companies'
    },
    {
      label: 'Kursy',
      to: '/courses'
    },
    {
      label: 'Zaświadczenia',
      to: '/certificates'
    },
    {
      label: 'Dzienniki',
      to: '/journals'
    }
  ]

  if (auth.user.value?.role === 1) {
    items.push({
      label: 'Administracja',
      to: '/admin/users'
    })
  }

  return items
})

function isActive(path: string) {
  if (path === '/') {
    return route.path === '/'
  }

  return route.path === path || route.path.startsWith(`${path}/`)
}

watch(
  () => route.path,
  () => {
    isMobileMenuOpen.value = false
  }
)

async function onLogout() {
  isMobileMenuOpen.value = false
  await auth.logout()
  await navigateTo('/login')
}
</script>

<template>
  <header class="border-b border-white/70 bg-white/85 backdrop-blur">
    <div class="mx-auto max-w-6xl px-4 py-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between gap-4">
        <div class="flex items-center gap-4">
          <NuxtLink to="/" class="text-lg font-semibold tracking-tight text-slate-900">
            Zaświadczenia
          </NuxtLink>

          <nav v-if="auth.user.value && !isLoginPage" class="hidden items-center gap-2 md:flex">
            <NuxtLink
              v-for="item in navigationItems"
              :key="item.to"
              :to="item.to"
              class="rounded-lg px-3 py-2 text-sm transition"
              :class="
                isActive(item.to)
                  ? 'bg-sky-100 text-sky-900'
                  : 'text-slate-500 hover:bg-white hover:text-slate-900'
              "
            >
              {{ item.label }}
            </NuxtLink>
          </nav>
        </div>

        <div class="flex items-center gap-3">
          <template v-if="auth.user.value">
            <NuxtLink
              to="/account"
              class="hidden rounded-lg px-3 py-2 text-right transition hover:bg-white/80 sm:block"
            >
              <p class="text-sm font-medium text-slate-900">
                {{ auth.user.value.firstName }} {{ auth.user.value.lastName }}
              </p>
              <p class="text-xs text-slate-500">Moje konto</p>
            </NuxtLink>

            <UButton
              color="neutral"
              variant="outline"
              class="md:hidden"
              :icon="isMobileMenuOpen ? 'i-lucide-x' : 'i-lucide-menu'"
              @click="isMobileMenuOpen = !isMobileMenuOpen"
            >
              Menu
            </UButton>

            <UButton
              color="neutral"
              variant="outline"
              icon="i-lucide-log-out"
              class="hidden md:inline-flex"
              @click="onLogout"
            >
              Wyloguj
            </UButton>
          </template>

          <UButton v-else-if="!isLoginPage" to="/login" color="primary" class="rounded-lg">
            Zaloguj się
          </UButton>
        </div>
      </div>

      <div
        v-if="auth.user.value && !isLoginPage && isMobileMenuOpen"
        class="mt-4 space-y-3 rounded-xl border border-slate-200 bg-white/95 p-4 shadow-sm md:hidden"
      >
        <div class="border-b border-slate-200 pb-3">
          <p class="text-sm font-medium text-slate-900">
            {{ auth.user.value.firstName }} {{ auth.user.value.lastName }}
          </p>
          <p class="text-xs text-slate-500">
            {{ auth.user.value.email }}
          </p>
        </div>

        <nav class="grid gap-2">
          <NuxtLink
            v-for="item in navigationItems"
            :key="item.to"
            :to="item.to"
            class="rounded-lg px-3 py-2 text-sm transition"
            :class="
              isActive(item.to)
                ? 'bg-sky-100 text-sky-900'
                : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
            "
          >
            {{ item.label }}
          </NuxtLink>
        </nav>

        <div class="flex flex-col gap-2 pt-2">
          <UButton to="/account" color="neutral" variant="outline" class="justify-center">
            Moje konto
          </UButton>

          <UButton
            color="neutral"
            variant="outline"
            icon="i-lucide-log-out"
            class="justify-center"
            @click="onLogout"
          >
            Wyloguj
          </UButton>
        </div>
      </div>
    </div>
  </header>
</template>
