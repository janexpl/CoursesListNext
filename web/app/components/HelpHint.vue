<script setup lang="ts">
// Wyjaśnienie schowane pod znakiem zapytania zamiast stałego akapitu pod nagłówkiem.
//
// Zachowanie zależy od urządzenia, a nie od szerokości ekranu (laptop z ekranem
// dotykowym ma oba sposoby naraz):
// - mysz: dymek otwiera się po najechaniu i zostaje, gdy kursor przejdzie na dymek,
// - dotyk: stuknięcie otwiera, drugie stuknięcie albo stuknięcie obok zamyka -
//   sam tryb "po najechaniu" byłby na telefonie martwy,
// - klawiatura: otwiera się po przejściu Tabem, Escape zamyka.
//
// Wyzwalacz to <span role="button">, a nie <button>, bo podpowiedź często stoi w <label>
// obok pola. <button> jest elementem etykietowalnym: postawiony przed polem przejąłby
// etykietę, a stuknięcie w niego przenosiłoby fokus do pola i wysuwało klawiaturę
// telefonu. Kliknięcie ma wyłączone działanie domyślne z tego samego powodu.
//
// Tło dymka jest jasne na sztywno, a nie z motywu biblioteki (bg-default): przy ciemnym
// motywie systemu biblioteka przełączała je na ciemne, a reszta aplikacji - w tym tekst
// dymka - ciemnego motywu nie ma, więc wychodził ciemny tekst na ciemnym tle.
//
// Czytniki ekranu nie ogłaszają dymków otwieranych po najechaniu, dlatego pełna treść
// jest też w samym wyzwalaczu jako tekst niewidoczny.
withDefaults(defineProps<{
  text: string
  label?: string
}>(), {
  label: 'Wyjaśnienie'
})
</script>

<template>
  <UPopover
    mode="hover"
    enable-touch
    :open-delay="120"
    :close-delay="150"
    :content="{ side: 'top', sideOffset: 6, collisionPadding: 12 }"
    :ui="{ content: 'bg-white ring-slate-200', arrow: 'fill-white stroke-slate-200' }"
    arrow
  >
    <span
      role="button"
      tabindex="0"
      class="help-hint relative inline-flex size-5 shrink-0 cursor-help items-center justify-center rounded-full align-middle text-slate-400 transition select-none after:absolute after:-inset-2 after:content-[''] hover:text-sky-600 focus-visible:text-sky-600 focus-visible:ring-2 focus-visible:ring-sky-300 focus-visible:outline-none data-[state=open]:text-sky-600 print:hidden pointer-coarse:after:-inset-3"
      @click.prevent.stop
    >
      <UIcon name="i-lucide-circle-help" class="size-4" aria-hidden="true" />
      <span class="sr-only">{{ label }}: {{ text }}</span>
    </span>

    <template #content>
      <p class="max-w-[min(22rem,calc(100vw-1.5rem))] px-3 py-2 text-sm leading-6 font-normal tracking-normal text-slate-700 normal-case">
        {{ text }}
      </p>
    </template>
  </UPopover>
</template>
