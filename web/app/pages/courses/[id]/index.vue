<script setup lang="ts">
import AuditHistoryPanel from '~/components/audit/AuditHistoryPanel.vue'

type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

type CertificateVariant = {
  key: string
  label: string
  languageCode: string
  courseName: string
  courseProgram: string
  certFrontPage: string
  isPrimary: boolean
}

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()
const auth = useAuth()

const courseId = computed(() => Number.parseInt(`${route.params.id}`, 10))

if (!Number.isFinite(courseId.value) || courseId.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: 'Nie znaleziono kursu'
  })
}

function formatExpiryLabel(value: string | null) {
  if (!value) {
    return 'Bez terminu ważności'
  }

  const numericValue = Number.parseInt(value, 10)
  if (!Number.isFinite(numericValue)) {
    return value
  }

  if (numericValue === 1) {
    return '1 rok'
  }

  return `${numericValue} lat`
}

function formatLanguageLabel(value: string) {
  const normalized = value.trim().toLowerCase()
  const labels: Record<string, string> = {
    en: 'angielski',
    de: 'niemiecki',
    uk: 'ukraiński',
    cs: 'czeski',
    sk: 'słowacki',
    lt: 'litewski'
  }

  if (!normalized) {
    return 'Nowa wersja'
  }

  return labels[normalized]
    ? `${normalized.toUpperCase()} - ${labels[normalized]}`
    : normalized.toUpperCase()
}

function parseCourseProgramEntries(value: string | null | undefined) {
  if (!value) {
    return [] as CourseProgramEntry[]
  }

  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? (parsed as CourseProgramEntry[]) : ([] as CourseProgramEntry[])
  } catch {
    return [] as CourseProgramEntry[]
  }
}

function buildProgramTotals(entries: CourseProgramEntry[]) {
  return entries.reduce(
    (acc, entry) => {
      acc.theory += Number.parseFloat(entry.TheoryTime ?? '0') || 0
      acc.practice += Number.parseFloat(entry.PracticeTime ?? '0') || 0
      return acc
    },
    {
      theory: 0,
      practice: 0
    }
  )
}

function buildCertificatePreviewDocument(html: string) {
  if (!html) {
    return ''
  }

  return `<!doctype html>
<html lang="pl">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
      :root {
        color-scheme: light;
      }

      @page {
        size: A4 portrait;
        margin: 0;
      }

      html {
        background: #f8fafc;
      }

      * {
        box-sizing: border-box;
      }

      body {
        margin: 0;
        padding: 20px;
        background:
          radial-gradient(circle at top left, rgb(14 165 233 / 0.10), transparent 25%),
          linear-gradient(180deg, #e2e8f0 0%, #f8fafc 100%);
        color: #0f172a;
        font-family: "Times New Roman", "Liberation Serif", Georgia, serif;
        line-height: 1.4;
      }

      .certificate-sheet {
        width: min(210mm, 100%);
        min-height: 297mm;
        margin: 0 auto;
        padding: 16mm 14mm;
        border: 1px solid #cbd5e1;
        border-radius: 8px;
        background: white;
        box-shadow:
          0 30px 70px rgb(15 23 42 / 0.10),
          0 10px 24px rgb(15 23 42 / 0.08);
      }

      .certificate-sheet > :first-child {
        margin-top: 0 !important;
      }

      .certificate-sheet > :last-child {
        margin-bottom: 0 !important;
      }

      h1, h2, h3, h4, h5, h6 {
        margin: 0 0 0.45rem;
        line-height: 1.2;
        color: #020617;
      }

      h1 {
        font-size: 32px;
        font-weight: 700;
      }

      h2 {
        font-size: 24px;
        font-weight: 700;
      }

      h3 {
        font-size: 18px;
        font-weight: 700;
      }

      p {
        margin: 0 0 0.4rem;
        font-size: 15px;
        line-height: 1.45;
      }

      ul, ol {
        margin: 0 0 0.45rem;
        padding-left: 1.25rem;
      }

      img {
        max-width: 100%;
        height: auto;
      }
    </style>
  </head>
  <body>
    <div class="certificate-sheet">
      ${html}
    </div>
  </body>
</html>`
}

const { data, pending, error, refresh } = await useAsyncData(
  `course:${courseId.value}`,
  async () => await api.course(courseId.value)
)

const course = computed(() => data.value?.data ?? null)
const isAdmin = computed(() => auth.user.value?.role === 1)

const {
  data: auditData,
  pending: auditPending,
  error: auditError,
  refresh: refreshAudit
} = await useAsyncData(
  `course-audit:${courseId.value}`,
  async () => {
    if (!isAdmin.value) {
      return { data: [] }
    }

    return await api.courseAuditLog(courseId.value)
  },
  {
    watch: [isAdmin]
  }
)

const auditEntries = computed(() => auditData.value?.data ?? [])
const auditErrorMessage = computed(() => {
  return auditError.value
    ? getApiErrorMessage(auditError.value, 'Nie udało się pobrać historii zmian kursu.')
    : ''
})

const activeVariantKey = ref<string>('pl')

const certificateVariants = computed<CertificateVariant[]>(() => {
  if (!course.value) {
    return []
  }

  return [
    {
      key: 'pl',
      label: 'PL - podstawowa',
      languageCode: 'pl',
      courseName: course.value.name,
      courseProgram: course.value.courseProgram,
      certFrontPage: course.value.certFrontPage,
      isPrimary: true
    },
    ...course.value.certificateTranslations.map((translation) => {
      return {
        key: `translation:${translation.languageCode}`,
        label: formatLanguageLabel(translation.languageCode),
        languageCode: translation.languageCode,
        courseName: translation.courseName,
        courseProgram: translation.courseProgram,
        certFrontPage: translation.certFrontPage,
        isPrimary: false
      }
    })
  ]
})

watch(
  certificateVariants,
  (variants) => {
    if (!variants.length) {
      activeVariantKey.value = 'pl'
      return
    }

    if (!variants.some(variant => variant.key === activeVariantKey.value)) {
      activeVariantKey.value = variants[0]?.key ?? 'pl'
    }
  },
  { immediate: true }
)

const activeCertificateVariant = computed<CertificateVariant | null>(() => {
  return certificateVariants.value.find(variant => variant.key === activeVariantKey.value) ?? null
})

const courseProgramEntries = computed(() => {
  return parseCourseProgramEntries(course.value?.courseProgram)
})

const activeCertificateVariantProgramEntries = computed(() => {
  return parseCourseProgramEntries(activeCertificateVariant.value?.courseProgram)
})

const hasInvalidActiveCertificateVariantProgram = computed(() => {
  if (!activeCertificateVariant.value?.courseProgram) {
    return false
  }

  return activeCertificateVariantProgramEntries.value.length === 0
})

const activeCertificateVariantProgramTotals = computed(() => {
  return buildProgramTotals(activeCertificateVariantProgramEntries.value)
})

const certificateLink = computed(() => {
  if (!course.value) {
    return '/certificates/new'
  }

  return {
    path: '/certificates/new',
    query: {
      courseId: course.value.id,
      courseName: course.value.name,
      courseSymbol: course.value.symbol,
      courseMainName: course.value.mainName || undefined,
      courseExpiryTime: course.value.expiryTime || undefined
    }
  }
})

const courseCertificatesLink = computed(() => {
  return `/courses/${courseId.value}/certificates`
})

const activeCertificateVariantDocument = computed(() => {
  return buildCertificatePreviewDocument(activeCertificateVariant.value?.certFrontPage ?? '')
})

useSeoMeta({
  title: () => course.value?.name || 'Szczegół kursu'
})

async function refreshAll() {
  await Promise.all([
    refresh(),
    isAdmin.value ? refreshAudit() : Promise.resolve()
  ])
}
</script>

<template>
  <section class="space-y-8">
    <div
      class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between"
    >
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">Kursy</p>
        <h1 class="wrap-break-word text-3xl font-semibold tracking-tight text-slate-900">
          {{ course?.name || 'Szczegół kursu' }}
        </h1>
        <p class="max-w-3xl text-sm leading-6 text-slate-600">
          Program kursu, okres ważności i szablon zaświadczenia.
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          :loading="pending || (isAdmin && auditPending)"
          @click="refreshAll()"
        >
          Odśwież
        </UButton>

        <NuxtLink
          to="/courses"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
        >
          Lista kursów
        </NuxtLink>

        <button
          type="button"
          class="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          @click="navigateTo(`/courses/${courseId}/edit`)"
        >
          Edytuj kurs
        </button>

        <NuxtLink
          :to="certificateLink"
          class="inline-flex items-center justify-center rounded-lg bg-sky-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-sky-700"
        >
          Wystaw zaświadczenie
        </NuxtLink>

        <NuxtLink
          :to="courseCertificatesLink"
          class="inline-flex items-center justify-center rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm font-medium text-sky-700 transition hover:border-sky-300 hover:bg-sky-100"
        >
          Wystawione zaświadczenia
        </NuxtLink>
      </div>
    </div>

    <div
      v-if="error"
      class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700"
    >
      Nie udało się pobrać danych kursu.
    </div>

    <div
      v-else-if="pending || !course"
      class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500 shadow-sm"
    >
      Ładowanie szczegółów kursu...
    </div>

    <template v-else>
      <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Symbol</p>
          <p
            class="mt-3 break-all font-mono text-xl font-semibold leading-tight tracking-tight text-slate-900"
          >
            {{ course.symbol }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Nazwa</p>
          <p
            class="mt-3 wrap-break-word text-xl font-semibold leading-tight tracking-tight text-slate-900"
          >
            {{ course.mainName || 'Brak' }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Ważność</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ formatExpiryLabel(course.expiryTime) }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.16em] text-sky-300">Program</p>
          <p class="mt-3 text-lg font-semibold tracking-tight">
            {{ courseProgramEntries.length }} tematów
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">Wersje obcojęzyczne</p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ course.certificateTranslations.length }}
          </p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
        <div class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Dane kursu</h2>

            <dl class="mt-5 grid gap-4 md:grid-cols-2">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Nazwa szczegółowa
                </dt>
                <dd class="mt-1 wrap-break-word text-sm text-slate-900">
                  {{ course.name }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Symbol</dt>
                <dd class="mt-1 break-all font-mono text-sm text-slate-900">
                  {{ course.symbol }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Nazwa</dt>
                <dd class="mt-1 wrap-break-word text-sm text-slate-900">
                  {{ course.mainName || 'Brak' }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Ważność</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ formatExpiryLabel(course.expiryTime) }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="space-y-1">
              <h2 class="text-lg font-semibold text-slate-900">Warianty zaświadczenia</h2>
              <p class="text-sm text-slate-500">
                Wybierz wersję podstawową albo obcojęzyczną, aby podejrzeć odpowiadający jej program
                i szablon.
              </p>
            </div>

            <div class="mt-5 flex flex-wrap gap-2">
              <button
                v-for="variant in certificateVariants"
                :key="variant.key"
                type="button"
                class="inline-flex items-center gap-2 rounded-full border px-4 py-2 text-sm font-medium transition"
                :class="
                  activeVariantKey === variant.key
                    ? 'border-sky-600 bg-sky-600 text-white shadow-sm'
                    : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:bg-slate-50'
                "
                @click="activeVariantKey = variant.key"
              >
                <span
                  class="inline-flex h-2.5 w-2.5 rounded-full"
                  :class="variant.isPrimary ? 'bg-amber-400' : 'bg-emerald-400'"
                />
                {{ variant.label }}
              </button>
            </div>

            <div v-if="activeCertificateVariant" class="mt-6 space-y-6">
              <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div class="space-y-1">
                    <h3 class="text-base font-semibold text-slate-900">Program szkolenia</h3>
                    <p class="text-sm text-slate-500">
                      Zakres tematyczny i liczba godzin dla wariantu
                      {{ activeCertificateVariant.label }}.
                    </p>
                  </div>

                  <span
                    class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
                    :class="
                      activeCertificateVariant.isPrimary
                        ? 'border-amber-200 bg-amber-50 text-amber-700'
                        : 'border-sky-200 bg-sky-50 text-sky-700'
                    "
                  >
                    {{
                      activeCertificateVariant.isPrimary ? 'Wersja podstawowa' : 'Wersja dodatkowa'
                    }}
                  </span>
                </div>

                <div
                  v-if="activeCertificateVariantProgramEntries.length"
                  class="mt-5 overflow-hidden rounded-lg border border-slate-200"
                >
                  <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-slate-200 text-sm">
                      <thead class="bg-slate-50">
                        <tr class="text-left text-slate-600">
                          <th class="w-14 px-4 py-3 font-medium">Lp.</th>
                          <th class="px-4 py-3 font-medium">Temat szkolenia</th>
                          <th class="w-28 px-4 py-3 text-center font-medium">Teoria</th>
                          <th class="w-28 px-4 py-3 text-center font-medium">Praktyka</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-slate-200 bg-white">
                        <tr
                          v-for="(entry, index) in activeCertificateVariantProgramEntries"
                          :key="`${entry.Subject || 'subject'}-${index}`"
                        >
                          <td class="px-4 py-3 text-slate-500">
                            {{ index + 1 }}
                          </td>
                          <td class="px-4 py-3 text-slate-900">
                            {{ entry.Subject || 'Brak tematu' }}
                          </td>
                          <td class="px-4 py-3 text-center text-slate-700">
                            {{ entry.TheoryTime || '0' }}
                          </td>
                          <td class="px-4 py-3 text-center text-slate-700">
                            {{ entry.PracticeTime || '0' }}
                          </td>
                        </tr>
                        <tr class="bg-slate-50 font-semibold text-slate-900">
                          <td colspan="2" class="px-4 py-3">Razem</td>
                          <td class="px-4 py-3 text-center">
                            {{ activeCertificateVariantProgramTotals.theory.toFixed(1) }}
                          </td>
                          <td class="px-4 py-3 text-center">
                            {{ activeCertificateVariantProgramTotals.practice.toFixed(1) }}
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>

                <div
                  v-else-if="hasInvalidActiveCertificateVariantProgram"
                  class="mt-5 rounded-lg border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-700"
                >
                  Program wybranego wariantu ma nieobsługiwany format JSON i nie może zostać
                  pokazany w tabeli.
                </div>

                <div
                  v-else
                  class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
                >
                  Brak programu dla wybranego wariantu.
                </div>
              </section>

              <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
                <div class="space-y-1">
                  <h3 class="text-base font-semibold text-slate-900">Podgląd szablonu</h3>
                  <p class="text-sm text-slate-500">
                    Podgląd frontu zaświadczenia dla wariantu {{ activeCertificateVariant.label }}.
                  </p>
                </div>

                <div
                  v-if="activeCertificateVariantDocument"
                  class="mt-5 overflow-hidden rounded-lg border border-slate-200 bg-slate-50"
                >
                  <iframe
                    :title="`Podgląd szablonu ${activeCertificateVariant.label}`"
                    :srcdoc="activeCertificateVariantDocument"
                    class="h-[52rem] w-full bg-white"
                  />
                </div>

                <div
                  v-else
                  class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
                >
                  Brak szablonu dla wybranego wariantu.
                </div>
              </section>
            </div>
          </section>
        </div>

        <aside class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">Metadane</h2>

            <dl class="mt-5 space-y-4">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">ID</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ course.id }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Symbol</dt>
                <dd class="mt-1 break-all font-mono text-sm text-slate-900">
                  {{ course.symbol }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Aktywny wariant</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ activeCertificateVariant?.label || 'Brak' }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Tematy w programie
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ activeCertificateVariantProgramEntries.length }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Wersje dodatkowe</dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ course.certificateTranslations.length }}
                </dd>
              </div>
            </dl>
          </section>
        </aside>
      </div>

      <AuditHistoryPanel
        v-if="isAdmin"
        :entries="auditEntries"
        :pending="auditPending"
        :error-message="auditErrorMessage"
        title="Historia zmian kursu"
        description="Zmiany zapisane dla kursu i jego wersji językowych."
        empty-message="Brak wpisów historii zmian dla tego kursu."
      />
    </template>
  </section>
</template>
