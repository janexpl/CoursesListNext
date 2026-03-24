<script setup lang="ts">
import type { JournalDetails } from '~/composables/useApi'

defineProps<{
  journal: JournalDetails
}>()

function formatDateTime(value: string | null) {
  if (!value) {
    return 'Brak'
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return parsed.toLocaleString('pl-PL')
}
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-slate-900">Informacje techniczne</h2>
      <p class="text-sm text-slate-500">
        Snapshot danych i informacje administracyjne dla tego dziennika.
      </p>
    </div>

    <dl class="mt-5 grid gap-4 text-sm">
      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Utworzono</dt>
        <dd class="text-slate-700">
          {{ formatDateTime(journal.createdAt) }}
        </dd>
      </div>

      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Ostatnia zmiana</dt>
        <dd class="text-slate-700">
          {{ formatDateTime(journal.updatedAt) }}
        </dd>
      </div>

      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Zamknięto</dt>
        <dd class="text-slate-700">
          {{ formatDateTime(journal.closedAt) }}
        </dd>
      </div>
    </dl>
  </section>
</template>
