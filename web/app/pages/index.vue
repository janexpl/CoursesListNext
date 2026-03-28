<script setup lang="ts">
definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Dashboard'
})

const api = useApi()
const auth = useAuth()

const { data, pending, error, refresh } = await useAsyncData('dashboard', async () => {
  return await api.dashboard()
})

const stats = computed(() => data.value?.data.stats)
const expiringCertificates = computed(() => data.value?.data.expiringCertificates ?? [])
const expiringCount = computed(() => data.value?.data.expiring.in30Days ?? 0)
</script>

<template>
  <section class="space-y-8">
    <div
      class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between"
    >
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Panel operacyjny</p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Dashboard</h1>
        <p class="max-w-2xl text-sm leading-6 text-slate-600">
          Witaj{{ auth.user.value ? `, ${auth.user.value.firstName}` : '' }}.
        </p>
      </div>

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

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać danych dashboardu.
    </div>

    <div class="grid gap-4 md:grid-cols-3">
      <div class="rounded-xl border border-slate-200 bg-white/85 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Kursanci</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ stats?.students ?? 0 }}
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/85 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Firmy</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ stats?.companies ?? 0 }}
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/85 p-6 shadow-sm">
        <p class="text-sm text-slate-500">Wydane certyfikaty</p>
        <p class="mt-3 text-4xl font-semibold tracking-tight text-slate-900">
          {{ stats?.certificates ?? 0 }}
        </p>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-[18rem_minmax(0,1fr)]">
      <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
        <p class="text-sm uppercase tracking-[0.18em] text-sky-300">Wygasające</p>
        <p class="mt-4 text-5xl font-semibold tracking-tight">
          {{ expiringCount }}
        </p>
        <p class="mt-3 text-sm leading-6 text-slate-300">
          Certyfikatów wygasających w ciągu najbliższych 30 dni.
        </p>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white/90 shadow-sm">
        <div class="border-b border-slate-200 px-6 py-5">
          <h2 class="text-lg font-semibold text-slate-900">Wygasające certyfikaty</h2>
        </div>

        <div v-if="pending" class="px-6 py-10 text-sm text-slate-500">Ładowanie danych...</div>

        <div
          v-else-if="expiringCertificates.length === 0"
          class="px-6 py-10 text-sm text-slate-500"
        >
          Brak wygasających certyfikatów.
        </div>

        <div v-else class="divide-y divide-slate-200">
          <article
            v-for="certificate in expiringCertificates"
            :key="certificate.certificateId"
            class="grid gap-3 px-6 py-5 md:grid-cols-[minmax(0,1fr)_15rem]"
          >
            <div class="space-y-1">
              <p class="font-medium text-slate-900">
                {{ certificate.studentName }}
              </p>
              <p class="text-sm text-slate-600">
                {{ certificate.companyName }}
              </p>
              <p class="text-sm text-slate-500">
                {{ certificate.courseName }}
              </p>
            </div>

            <div class="space-y-2 text-sm md:justify-self-end md:max-w-60 md:text-right">
              <p class="font-medium text-slate-900">
                {{ certificate.expiryDate }}
              </p>
              <div class="text-slate-500">
                <p class="text-xs uppercase tracking-[0.14em] text-slate-400">Numer</p>
                <p class="mt-1 font-mono text-[13px] leading-5 break-all text-slate-600">
                  {{ certificate.registryNumber }}/{{ certificate.courseSymbol }}/{{
                    certificate.registryYear
                  }}
                </p>
              </div>
            </div>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>
