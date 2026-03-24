<script setup lang="ts">
import type { JournalDetails } from '~/composables/useApi'

defineProps<{
  journal: JournalDetails
}>()
</script>

<template>
  <section class="rounded-xl border border-slate-200 bg-white/90 p-6 shadow-sm">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-slate-900">Informacje podstawowe</h2>
      <p class="text-sm text-slate-500">
        Nagłówek dziennika i dane organizacyjne zapisane w momencie utworzenia.
      </p>
    </div>

    <dl class="mt-5 grid gap-4 text-sm md:grid-cols-2">
      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Organizator</dt>
        <dd class="text-slate-700">
          {{ journal.organizerName }}
        </dd>
      </div>

      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Miejsce</dt>
        <dd class="text-slate-700">
          {{ journal.location }}
        </dd>
      </div>

      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Forma szkolenia</dt>
        <dd class="text-slate-700">
          {{ journal.formOfTraining }}
        </dd>
      </div>

      <div class="space-y-1">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Firma</dt>
        <dd class="text-slate-700">
          {{ journal.companyName || 'Bez przypisanej firmy' }}
        </dd>
      </div>

      <div class="space-y-1 md:col-span-2">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Podstawa prawna</dt>
        <dd class="text-slate-700">
          {{ journal.legalBasis }}
        </dd>
      </div>

      <div class="space-y-1 md:col-span-2">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Adres organizatora</dt>
        <dd class="text-slate-700">
          {{ journal.organizerAddress || 'Brak adresu organizatora' }}
        </dd>
      </div>

      <div class="space-y-1 md:col-span-2">
        <dt class="text-xs uppercase tracking-[0.16em] text-slate-400">Notatki</dt>
        <dd class="text-slate-700">
          {{ journal.notes || 'Brak notatek' }}
        </dd>
      </div>
    </dl>
  </section>
</template>
