<script setup lang="ts">
import type { CertificateSummary } from '~/composables/useApi'

definePageMeta({
  middleware: 'auth'
})

useSeoMeta({
  title: 'Zaświadczenia'
})

function certificateNumber(certificate: CertificateSummary) {
  return `${certificate.registryNumber}/${certificate.courseSymbol}/${certificate.registryYear}`
}

const api = useApi()
const search = ref('')
const debouncedSearch = ref('')
let searchDebounceTimer: ReturnType<typeof setTimeout> | undefined

watch(search, (value) => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }

  searchDebounceTimer = setTimeout(() => {
    debouncedSearch.value = value.trim()
  }, 300)
})

onBeforeUnmount(() => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }
})

const { data, pending, error, refresh } = await useAsyncData(
  'certificates',
  async () => {
    return await api.certificates({
      search: debouncedSearch.value || undefined,
      limit: 100
    })
  },
  {
    watch: [debouncedSearch]
  }
)

const certificates = computed(() => data.value?.data ?? [])
const hasActiveSearch = computed(() => debouncedSearch.value.length > 0)
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Zaświadczenia
        </p>
        <h1 class="text-3xl font-semibold tracking-tight text-slate-900">
          Lista zaświadczeń
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Ostatnie wpisy z rejestru. Możesz wyszukać kursanta, kurs lub numer zaświadczenia,
          a potem przejść do szczegółu.
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

        <NuxtLink
          to="/certificates/new"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Nowe zaświadczenie
        </NuxtLink>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
      <label class="block space-y-2">
        <span class="text-sm font-medium text-slate-700">Szukaj</span>
        <input
          v-model="search"
          type="text"
          placeholder="Nazwisko, firma, kurs, symbol lub numer zaświadczenia"
          class="w-full rounded-md border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none transition focus:border-sky-500 focus:ring-4 focus:ring-sky-100"
        >
      </label>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać listy zaświadczeń.
    </div>

    <div
      v-else-if="pending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie listy zaświadczeń...
    </div>

    <div
      v-else-if="certificates.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-sm text-slate-500"
    >
      {{ hasActiveSearch ? 'Brak wyników dla podanej frazy.' : 'Brak zaświadczeń do wyświetlenia.' }}
    </div>

    <div
      v-else
      class="grid gap-4"
    >
      <NuxtLink
        v-for="certificate in certificates"
        :key="certificate.id"
        :to="`/certificates/${certificate.id}`"
        class="grid gap-4 rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm transition hover:border-sky-300 hover:bg-white md:grid-cols-[minmax(0,1fr)_16rem]"
      >
        <div class="space-y-2">
          <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-slate-400">
            <span>{{ certificate.date }}</span>
            <span>•</span>
            <span>{{ certificate.courseSymbol }}</span>
          </div>

          <h2 class="text-lg font-semibold text-slate-900">
            {{ certificate.studentName }}
          </h2>

          <p class="text-sm text-slate-600">
            {{ certificate.companyName || 'Brak firmy' }}
          </p>

          <p class="text-sm text-slate-500">
            {{ certificate.courseName }}
          </p>
        </div>

        <div class="space-y-3 md:justify-self-end md:text-right">
          <div>
            <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
              Numer
            </p>
            <p class="mt-1 font-mono text-sm break-all text-slate-700">
              {{ certificateNumber(certificate) }}
            </p>
          </div>

          <div>
            <p class="text-xs uppercase tracking-[0.16em] text-slate-400">
              Ważność
            </p>
            <p class="mt-1 text-sm text-slate-700">
              {{ certificate.expiryDate ?? 'Brak terminu' }}
            </p>
          </div>
        </div>
      </NuxtLink>
    </div>
  </section>
</template>
