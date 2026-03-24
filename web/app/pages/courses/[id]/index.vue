<script setup lang="ts">
type CourseProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

definePageMeta({
  middleware: 'auth'
})

const route = useRoute()
const api = useApi()

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

const { data, pending, error, refresh } = await useAsyncData(
  `course:${courseId.value}`,
  async () => await api.course(courseId.value)
)

const course = computed(() => data.value?.data ?? null)

const courseProgramEntries = computed(() => {
  if (!course.value?.courseProgram) {
    return [] as CourseProgramEntry[]
  }

  try {
    const parsed = JSON.parse(course.value.courseProgram)
    if (!Array.isArray(parsed)) {
      return [] as CourseProgramEntry[]
    }

    return parsed as CourseProgramEntry[]
  } catch {
    return [] as CourseProgramEntry[]
  }
})

const hasInvalidCourseProgram = computed(() => {
  if (!course.value?.courseProgram) {
    return false
  }

  return courseProgramEntries.value.length === 0
})

const programTotals = computed(() => {
  return courseProgramEntries.value.reduce((acc, entry) => {
    acc.theory += Number.parseFloat(entry.TheoryTime ?? '0') || 0
    acc.practice += Number.parseFloat(entry.PracticeTime ?? '0') || 0
    return acc
  }, {
    theory: 0,
    practice: 0
  })
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

const certFrontPageDocument = computed(() => {
  if (!course.value?.certFrontPage) {
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
      ${course.value.certFrontPage}
    </div>
  </body>
</html>`
})

useSeoMeta({
  title: () => course.value?.name || 'Szczegół kursu'
})
</script>

<template>
  <section class="space-y-8">
    <div class="flex flex-col gap-3 rounded-xl border border-white/60 bg-white/85 p-8 shadow-sm backdrop-blur sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <p class="text-sm font-medium uppercase tracking-[0.18em] text-sky-700">
          Kursy
        </p>
        <h1 class="break-words text-3xl font-semibold tracking-tight text-slate-900">
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
          :loading="pending"
          @click="refresh()"
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
      <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">
            Symbol
          </p>
          <p class="mt-3 break-all font-mono text-xl font-semibold leading-tight tracking-tight text-slate-900">
            {{ course.symbol }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">
            Nazwa
          </p>
          <p class="mt-3 break-words text-xl font-semibold leading-tight tracking-tight text-slate-900">
            {{ course.mainName || 'Brak' }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
          <p class="text-sm text-slate-500">
            Ważność
          </p>
          <p class="mt-3 text-2xl font-semibold tracking-tight text-slate-900">
            {{ formatExpiryLabel(course.expiryTime) }}
          </p>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-950 p-6 text-white shadow-sm">
          <p class="text-sm uppercase tracking-[0.16em] text-sky-300">
            Program
          </p>
          <p class="mt-3 text-lg font-semibold tracking-tight">
            {{ courseProgramEntries.length }} tematów
          </p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
        <div class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">
              Dane kursu
            </h2>

            <dl class="mt-5 grid gap-4 md:grid-cols-2">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Nazwa szczegółowa
                </dt>
                <dd class="mt-1 break-words text-sm text-slate-900">
                  {{ course.name }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Symbol
                </dt>
                <dd class="mt-1 break-all font-mono text-sm text-slate-900">
                  {{ course.symbol }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Nazwa
                </dt>
                <dd class="mt-1 break-words text-sm text-slate-900">
                  {{ course.mainName || 'Brak' }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Ważność
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ formatExpiryLabel(course.expiryTime) }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-slate-900">
                  Program kursu
                </h2>
                <p class="mt-1 text-sm text-slate-500">
                  Zakres tematyczny oraz liczba godzin zajęć.
                </p>
              </div>
            </div>

            <div
              v-if="courseProgramEntries.length"
              class="mt-5 overflow-hidden rounded-lg border border-slate-200"
            >
              <div class="overflow-x-auto">
                <table class="min-w-full divide-y divide-slate-200 text-sm">
                  <thead class="bg-slate-50">
                    <tr class="text-left text-slate-600">
                      <th class="w-14 px-4 py-3 font-medium">
                        Lp.
                      </th>
                      <th class="px-4 py-3 font-medium">
                        Temat szkolenia
                      </th>
                      <th class="w-28 px-4 py-3 text-center font-medium">
                        Teoria
                      </th>
                      <th class="w-28 px-4 py-3 text-center font-medium">
                        Praktyka
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-200 bg-white">
                    <tr
                      v-for="(entry, index) in courseProgramEntries"
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
                      <td
                        colspan="2"
                        class="px-4 py-3"
                      >
                        Razem
                      </td>
                      <td class="px-4 py-3 text-center">
                        {{ programTotals.theory.toFixed(1) }}
                      </td>
                      <td class="px-4 py-3 text-center">
                        {{ programTotals.practice.toFixed(1) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div
              v-else-if="hasInvalidCourseProgram"
              class="mt-5 rounded-lg border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-700"
            >
              Program kursu ma nieobsługiwany format JSON i nie może zostać pokazany w tabeli.
            </div>

            <div
              v-else
              class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
            >
              Brak programu kursu.
            </div>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <div class="space-y-1">
              <h2 class="text-lg font-semibold text-slate-900">
                Podgląd szablonu zaświadczenia
              </h2>
              <p class="text-sm text-slate-500">
                Podgląd aktualnego HTML używanego jako front zaświadczenia.
              </p>
            </div>

            <div
              v-if="certFrontPageDocument"
              class="mt-5 overflow-hidden rounded-lg border border-slate-200 bg-slate-50"
            >
              <iframe
                title="Podgląd szablonu zaświadczenia"
                :srcdoc="certFrontPageDocument"
                class="h-[72rem] w-full bg-white"
              />
            </div>

            <div
              v-else
              class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
            >
              Brak szablonu zaświadczenia dla tego kursu.
            </div>
          </section>
        </div>

        <aside class="space-y-6">
          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">
              Metadane
            </h2>

            <dl class="mt-5 space-y-4">
              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  ID
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ course.id }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Symbol
                </dt>
                <dd class="mt-1 break-all font-mono text-sm text-slate-900">
                  {{ course.symbol }}
                </dd>
              </div>

              <div>
                <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">
                  Tematy w programie
                </dt>
                <dd class="mt-1 text-sm text-slate-900">
                  {{ courseProgramEntries.length }}
                </dd>
              </div>
            </dl>
          </section>

          <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
            <h2 class="text-lg font-semibold text-slate-900">
              Surowe dane programu
            </h2>

            <pre class="mt-5 overflow-x-auto rounded-lg bg-slate-950 px-4 py-4 text-xs leading-6 text-slate-100">{{ course.courseProgram || '' }}</pre>
          </section>
        </aside>
      </div>
    </template>
  </section>
</template>
