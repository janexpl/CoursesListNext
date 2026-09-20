<script setup lang="ts">
import type { PublicCertificate } from '~/composables/useApi'

// Strona celowo BEZ definePageMeta z middleware: to adres z kodu QR na zaświadczeniu,
// otwierany przez osoby, które nie mają konta w systemie. Middleware nie są globalne,
// więc brak deklaracji oznacza stronę publiczną.

const route = useRoute()
const api = useApi()

const code = computed(() => String(route.params.code ?? '').trim().toUpperCase())

const { data, pending, error } = await useAsyncData(
  () => `verify:${code.value}`,
  async () => await api.publicCertificate(code.value),
  { watch: [code] }
)

const certificate = computed<PublicCertificate | null>(() => data.value?.data ?? null)

const errorStatus = computed(() => (error.value as { statusCode?: number } | null)?.statusCode ?? 0)

// Nieznany kod i kod w złym formacie to dla odwiedzającego ta sama sytuacja:
// dokumentu nie ma w rejestrze. Rozróżnienie służy tylko podpowiedzi, czego szukać.
const notFound = computed(() => errorStatus.value === 404 || errorStatus.value === 400)
const malformed = computed(() => errorStatus.value === 400)
const throttled = computed(() => errorStatus.value === 429)

const errorMessage = computed(() => {
  if (!error.value || notFound.value) {
    return ''
  }
  if (throttled.value) {
    return 'Za dużo sprawdzeń tego dokumentu w krótkim czasie. Spróbuj ponownie za minutę.'
  }

  return getApiErrorMessage(
    error.value,
    'Nie udało się sprawdzić zaświadczenia. Spróbuj ponownie za chwilę.'
  )
})

type StatusBadge = {
  label: string
  color: 'success' | 'warning' | 'error'
  icon: string
  description: string
}

const statusBadge = computed<StatusBadge | null>(() => {
  if (!certificate.value) {
    return null
  }

  if (certificate.value.status === 'revoked') {
    return {
      label: 'Unieważnione',
      color: 'error',
      icon: 'i-lucide-shield-x',
      description: 'Dokument został unieważniony i nie jest ważny.'
    }
  }

  if (certificate.value.expired) {
    return {
      label: 'Po terminie ważności',
      color: 'warning',
      icon: 'i-lucide-shield-alert',
      description: 'Dokument pochodzi z rejestru, ale minął jego termin ważności.'
    }
  }

  return {
    label: 'Ważne',
    color: 'success',
    icon: 'i-lucide-shield-check',
    description: 'Dokument pochodzi z rejestru i jest ważny.'
  }
})

function formatDate(value: string | null) {
  if (!value) {
    return ''
  }

  const [year, month, day] = value.slice(0, 10).split('-')
  if (!year || !month || !day) {
    return value
  }

  return `${day}.${month}.${year}`
}

const courseDates = computed(() => {
  if (!certificate.value) {
    return ''
  }

  const start = formatDate(certificate.value.courseDateStart)
  const end = formatDate(certificate.value.courseDateEnd)
  return end && end !== start ? `${start} – ${end}` : start
})

useSeoMeta({
  title: 'Weryfikacja zaświadczenia',
  robots: 'noindex, nofollow'
})
</script>

<template>
  <section class="mx-auto max-w-2xl space-y-6">
    <div class="space-y-2">
      <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Weryfikacja</p>
      <h1 class="text-3xl font-semibold tracking-tight text-slate-900">Sprawdzenie zaświadczenia</h1>
      <p class="text-sm leading-6 text-slate-600">
        Kod weryfikacyjny: <span class="font-mono text-slate-900">{{ code }}</span>
      </p>
    </div>

    <div
      v-if="pending"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Sprawdzanie w rejestrze...
    </div>

    <div
      v-else-if="notFound"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-8 shadow-sm"
    >
      <div class="flex items-start gap-3">
        <UIcon name="i-lucide-search-x" class="mt-0.5 size-6 text-slate-400" />
        <div class="space-y-2">
          <h2 class="text-lg font-semibold text-slate-900">Nie znaleziono dokumentu o tym kodzie</h2>
          <p v-if="malformed" class="text-sm leading-6 text-slate-600">
            Kod ma 12 znaków i nie zawiera cyfr 0 i 1 ani liter O i I. Sprawdź, czy został
            przepisany dokładnie tak, jak na dokumencie.
          </p>
          <p v-else class="text-sm leading-6 text-slate-600">
            Sprawdź, czy kod został przepisany dokładnie tak, jak na dokumencie. Jeżeli kod się
            zgadza, dokument nie pochodzi z tego rejestru.
          </p>
        </div>
      </div>
    </div>

    <div
      v-else-if="errorMessage"
      class="rounded-xl border border-red-200 bg-red-50 px-6 py-5 text-sm text-red-700"
    >
      {{ errorMessage }}
    </div>

    <template v-else-if="certificate && statusBadge">
      <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
        <div class="flex items-start gap-3">
          <UIcon
            :name="statusBadge.icon"
            class="mt-0.5 size-7"
            :class="{
              'text-emerald-600': statusBadge.color === 'success',
              'text-amber-600': statusBadge.color === 'warning',
              'text-red-600': statusBadge.color === 'error'
            }"
          />
          <div class="space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <UBadge :color="statusBadge.color" variant="subtle">
                {{ statusBadge.label }}
              </UBadge>
              <UBadge
                v-if="certificate.duplicateIssued"
                color="neutral"
                variant="subtle"
              >
                Wystawiono duplikat
              </UBadge>
            </div>
            <p class="text-sm leading-6 text-slate-700">
              {{ statusBadge.description }}
              <span v-if="certificate.status === 'revoked' && certificate.revokedAt">
                Data unieważnienia: {{ formatDate(certificate.revokedAt) }}.
              </span>
              <span v-else-if="certificate.duplicateIssued && certificate.duplicateIssuedAt">
                Duplikat wystawiono {{ formatDate(certificate.duplicateIssuedAt) }} — to ten sam
                dokument o tym samym numerze.
              </span>
            </p>
          </div>
        </div>

        <dl class="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Kursant</dt>
            <dd class="mt-1 text-sm text-slate-900">
              {{ certificate.studentName }}
            </dd>
          </div>

          <div>
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Numer zaświadczenia</dt>
            <dd class="mt-1 break-all font-mono text-sm text-slate-900">
              {{ certificate.certificateNumber }}
            </dd>
          </div>

          <div class="sm:col-span-2">
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Szkolenie</dt>
            <dd class="mt-1 text-sm text-slate-900">
              {{ certificate.courseName }}
            </dd>
          </div>

          <div>
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Termin szkolenia</dt>
            <dd class="mt-1 text-sm text-slate-900">
              {{ courseDates }}
            </dd>
          </div>

          <div>
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Data wystawienia</dt>
            <dd class="mt-1 text-sm text-slate-900">
              {{ formatDate(certificate.issuedAt) }}
            </dd>
          </div>

          <div>
            <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Ważne do</dt>
            <dd class="mt-1 text-sm text-slate-900">
              {{ certificate.validUntil ? formatDate(certificate.validUntil) : 'Bezterminowo' }}
            </dd>
          </div>
        </dl>
      </div>

      <p class="text-xs leading-5 text-slate-500">
        Dane pochodzą z rejestru zaświadczeń firmy Nasza Era Sp. z o.o. Strona pokazuje wyłącznie informacje
        widoczne na samym dokumencie.
      </p>
    </template>
  </section>
</template>
