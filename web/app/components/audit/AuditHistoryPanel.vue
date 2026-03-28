<script setup lang="ts">
import type { AuditLogEntry } from '~/composables/useApi'

withDefaults(defineProps<{
  entries: AuditLogEntry[]
  pending?: boolean
  errorMessage?: string | null
  title?: string
  description?: string
  emptyMessage?: string
}>(), {
  pending: false,
  errorMessage: '',
  title: 'Historia zmian',
  description: 'Zdarzenia zapisane przez audit log dla tej encji.',
  emptyMessage: 'Brak wpisów historii zmian.'
})

type ActionPresentation = {
  label: string
  badgeClass: string
  summary: string
}

const fieldLabelMap: Record<string, string> = {
  firstName: 'imię',
  firstname: 'imię',
  lastName: 'nazwisko',
  lastname: 'nazwisko',
  secondName: 'drugie imię',
  secondname: 'drugie imię',
  email: 'email',
  role: 'rola',
  companyId: 'firma',
  companyName: 'firma',
  courseId: 'kurs',
  courseName: 'nazwa kursu',
  courseProgram: 'program kursu',
  certFrontPage: 'szablon zaświadczenia',
  certificateTranslations: 'wersje językowe',
  languageCode: 'język',
  expiryTime: 'okres ważności',
  symbol: 'symbol',
  mainName: 'główna nazwa',
  name: 'nazwa',
  nip: 'NIP',
  city: 'miasto',
  street: 'ulica',
  zipcode: 'kod pocztowy',
  telephone: 'telefon',
  telephoneno: 'telefon',
  contactPerson: 'osoba kontaktowa',
  contactperson: 'osoba kontaktowa',
  note: 'notatka',
  birthDate: 'data urodzenia',
  birthdate: 'data urodzenia',
  birthPlace: 'miejsce urodzenia',
  birthplace: 'miejsce urodzenia',
  pesel: 'PESEL',
  addressStreet: 'ulica',
  addressstreet: 'ulica',
  addressCity: 'miasto',
  addresscity: 'miasto',
  addressZip: 'kod pocztowy',
  addresszip: 'kod pocztowy',
  date: 'data',
  registryYear: 'rok rejestru',
  registryNumber: 'numer rejestru',
  courseDateStart: 'data rozpoczęcia kursu',
  courseDateEnd: 'data zakończenia kursu',
  expiryDate: 'data ważności',
  status: 'status',
  title: 'tytuł',
  location: 'miejsce',
  organizerName: 'organizator',
  organizerAddress: 'adres organizatora',
  legalBasis: 'podstawa prawna',
  formOfTraining: 'forma szkolenia',
  notes: 'uwagi',
  metadata: 'metadane'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function prettyJSON(value: unknown) {
  if (value === null || value === undefined) {
    return 'null'
  }

  return JSON.stringify(value, null, 2)
}

function formatAction(action: string) {
  switch (action) {
    case 'create':
      return 'Utworzenie'
    case 'update':
      return 'Aktualizacja'
    case 'delete':
      return 'Usunięcie'
    case 'soft_delete':
      return 'Archiwizacja'
    case 'profile_update':
      return 'Aktualizacja profilu'
    case 'password_change':
      return 'Zmiana hasła'
    default:
      return action.replaceAll('_', ' ')
  }
}

function describeAction(action: string): ActionPresentation {
  switch (action) {
    case 'create':
      return {
        label: 'Utworzenie',
        badgeClass: 'border-emerald-200 bg-emerald-50 text-emerald-700',
        summary: 'Utworzono nowy rekord.'
      }
    case 'update':
      return {
        label: 'Aktualizacja',
        badgeClass: 'border-sky-200 bg-sky-50 text-sky-700',
        summary: 'Zmieniono dane rekordu.'
      }
    case 'delete':
      return {
        label: 'Usunięcie',
        badgeClass: 'border-rose-200 bg-rose-50 text-rose-700',
        summary: 'Usunięto rekord.'
      }
    case 'soft_delete':
      return {
        label: 'Archiwizacja',
        badgeClass: 'border-amber-200 bg-amber-50 text-amber-700',
        summary: 'Zarchiwizowano rekord.'
      }
    case 'profile_update':
      return {
        label: 'Aktualizacja profilu',
        badgeClass: 'border-sky-200 bg-sky-50 text-sky-700',
        summary: 'Zmieniono dane profilu.'
      }
    case 'password_change':
      return {
        label: 'Zmiana hasła',
        badgeClass: 'border-violet-200 bg-violet-50 text-violet-700',
        summary: 'Zmieniono hasło użytkownika.'
      }
    default:
      return {
        label: formatAction(action),
        badgeClass: 'border-slate-200 bg-slate-50 text-slate-700',
        summary: 'Zarejestrowano operację systemową.'
      }
  }
}

function formatDateTime(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat('pl-PL', {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(parsed)
}

function describeActor(entry: AuditLogEntry) {
  if (entry.actorUserName) {
    return entry.actorUserEmail ? `${entry.actorUserName} (${entry.actorUserEmail})` : entry.actorUserName
  }

  if (entry.actorUserEmail) {
    return entry.actorUserEmail
  }

  return 'Nieznany użytkownik'
}

function changedFields(entry: AuditLogEntry) {
  const beforeValue = entry.before
  const afterValue = entry.after

  if (!isRecord(beforeValue) || !isRecord(afterValue)) {
    return []
  }

  const keys = new Set([...Object.keys(beforeValue), ...Object.keys(afterValue)])

  return [...keys].filter((key) => {
    return JSON.stringify(beforeValue[key]) !== JSON.stringify(afterValue[key])
  })
}

function formatFieldLabel(field: string) {
  return fieldLabelMap[field] ?? field
}

function summarizeEntry(entry: AuditLogEntry) {
  const presentation = describeAction(entry.action)
  const fields = changedFields(entry)

  if (!fields.length) {
    return presentation.summary
  }

  if (fields.length === 1) {
    return `Zmodyfikowano pole: ${formatFieldLabel(fields[0] ?? '')}.`
  }

  return `Zmodyfikowano ${fields.length} pól.`
}

function limitChangedFields(entry: AuditLogEntry) {
  return changedFields(entry).slice(0, 6)
}

function hasMoreChangedFields(entry: AuditLogEntry) {
  return changedFields(entry).length > 6
}

function hasPayload(value: unknown) {
  return value !== null && value !== undefined
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-slate-900">
        {{ title }}
      </h2>
      <p class="text-sm text-slate-500">
        {{ description }}
      </p>
    </div>

    <div
      v-if="errorMessage"
      class="mt-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ errorMessage }}
    </div>

    <div
      v-else-if="pending"
      class="mt-5 rounded-lg border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-500"
    >
      Ładowanie historii zmian...
    </div>

    <div
      v-else-if="!entries.length"
      class="mt-5 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500"
    >
      {{ emptyMessage }}
    </div>

    <div
      v-else
      class="mt-6 space-y-4"
    >
      <article
        v-for="entry in entries"
        :key="entry.id"
        class="rounded-xl border border-slate-200 bg-gradient-to-br from-slate-50 to-white p-5 shadow-sm"
      >
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="inline-flex items-center justify-center rounded-full border px-3 py-1 text-xs font-medium"
                :class="describeAction(entry.action).badgeClass"
              >
                {{ describeAction(entry.action).label }}
              </span>
              <span class="text-xs uppercase tracking-[0.16em] text-slate-400">
                ID wpisu {{ entry.id }}
              </span>
            </div>

            <p class="text-sm font-medium text-slate-900">
              {{ describeActor(entry) }}
            </p>
            <p class="text-sm text-slate-500">
              {{ formatDateTime(entry.createdAt) }}
            </p>
            <p class="text-sm leading-6 text-slate-600">
              {{ summarizeEntry(entry) }}
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500">
            <span
              v-if="entry.requestId"
              class="rounded-full border border-slate-200 bg-white px-3 py-1 font-medium text-slate-600"
            >
              {{ entry.requestId }}
            </span>
          </div>
        </div>

        <div
          v-if="changedFields(entry).length"
          class="mt-4 rounded-lg border border-slate-200 bg-white px-4 py-4"
        >
          <p class="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">
            Zmienione pola
          </p>

          <div class="mt-3 flex flex-wrap gap-2">
            <span
              v-for="field in limitChangedFields(entry)"
              :key="field"
              class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-700"
            >
              {{ formatFieldLabel(field) }}
            </span>
            <span
              v-if="hasMoreChangedFields(entry)"
              class="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-500"
            >
              +{{ changedFields(entry).length - 6 }} kolejnych
            </span>
          </div>
        </div>

        <div class="mt-5 grid gap-4 lg:grid-cols-3">
          <details
            v-if="hasPayload(entry.before)"
            class="rounded-lg border border-slate-200 bg-white p-4"
          >
            <summary class="cursor-pointer text-sm font-medium text-slate-800">
              Stan przed zmianą
            </summary>
            <pre class="mt-3 overflow-x-auto rounded-md bg-slate-950 px-4 py-3 text-xs leading-6 text-slate-100">{{ prettyJSON(entry.before) }}</pre>
          </details>

          <details
            v-if="hasPayload(entry.after)"
            class="rounded-lg border border-slate-200 bg-white p-4"
          >
            <summary class="cursor-pointer text-sm font-medium text-slate-800">
              Stan po zmianie
            </summary>
            <pre class="mt-3 overflow-x-auto rounded-md bg-slate-950 px-4 py-3 text-xs leading-6 text-slate-100">{{ prettyJSON(entry.after) }}</pre>
          </details>

          <details
            v-if="hasPayload(entry.metadata)"
            class="rounded-lg border border-slate-200 bg-white p-4"
          >
            <summary class="cursor-pointer text-sm font-medium text-slate-800">
              Metadane operacji
            </summary>
            <pre class="mt-3 overflow-x-auto rounded-md bg-slate-950 px-4 py-3 text-xs leading-6 text-slate-100">{{ prettyJSON(entry.metadata) }}</pre>
          </details>
        </div>
      </article>
    </div>
  </section>
</template>
