<script setup lang="ts">
import type { PrintAsset } from '~/composables/useApi'

definePageMeta({
  middleware: 'admin'
})

useSeoMeta({
  title: 'Nadruki zaświadczeń'
})

const api = useApi()

type AssetKind = PrintAsset['kind']

const kinds: Array<{ kind: AssetKind, label: string, hint: string }> = [
  { kind: 'pieczatka_1', label: 'Pieczątka 1', hint: 'Zwykle pieczątka firmowa organizatora.' },
  { kind: 'pieczatka_2', label: 'Pieczątka 2', hint: 'Zwykle pieczątka wykładowcy albo uprawnienia.' },
  { kind: 'podpis', label: 'Podpis', hint: 'Podpis osoby upoważnionej przez organizatora.' }
]

const { data, pending, error, refresh } = await useAsyncData(
  'certificate-print-assets',
  async () => await api.certificatePrintAssets()
)

const assets = computed(() => data.value?.data ?? [])
const loadError = computed(() =>
  error.value ? getApiErrorMessage(error.value, 'Nie udało się pobrać listy nadruków.') : ''
)

const actionError = ref('')
const busyKind = ref<AssetKind | null>(null)
const widths = reactive<Record<string, number>>({})

function assetFor(kind: AssetKind) {
  return assets.value.find(asset => asset.kind === kind) ?? null
}

// Adres pliku niesie znacznik czasu wgrania, żeby po podmianie przeglądarka nie pokazała
// poprzedniej pieczątki z własnego cache.
function fileUrl(asset: PrintAsset) {
  return `/api/v1/certificate-print-assets/${asset.kind}/file?v=${encodeURIComponent(asset.uploadedAt)}`
}

// Adresy wiązane przez skrypt, a nie wpisane w atrybut na sztywno: statyczny src
// Vite próbuje rozwiązać jako import zasobu i build się wywraca.
const guillocheFrontUrl = '/api/v1/certificate-print-assets/guilloche'
const guillocheBackUrl = '/api/v1/certificate-print-assets/guilloche?side=back'

function formatSize(bytes: number) {
  return `${Math.round(bytes / 1024)} kB`
}

async function onUpload(kind: AssetKind, event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) {
    return
  }

  actionError.value = ''
  busyKind.value = kind
  try {
    await api.uploadCertificatePrintAsset(kind, file, widths[kind])
    await refresh()
  } catch (uploadError) {
    actionError.value = getApiErrorMessage(uploadError, 'Nie udało się wgrać pliku.')
  } finally {
    busyKind.value = null
    // Bez tego wybranie tego samego pliku drugi raz nie wywołałoby zdarzenia.
    input.value = ''
  }
}

async function onDelete(kind: AssetKind) {
  actionError.value = ''
  busyKind.value = kind
  try {
    await api.deleteCertificatePrintAsset(kind)
    await refresh()
  } catch (deleteError) {
    actionError.value = getApiErrorMessage(deleteError, 'Nie udało się usunąć nadruku.')
  } finally {
    busyKind.value = null
  }
}

async function onWidthChange(kind: AssetKind) {
  const asset = assetFor(kind)
  if (!asset || !widths[kind] || widths[kind] === asset.printWidthMm) {
    return
  }

  actionError.value = ''
  busyKind.value = kind
  try {
    // Szerokość zmieniamy przez ponowne wgranie tego samego pliku - inaczej trzeba by
    // osobnej trasy tylko po to, żeby zmienić jedną liczbę.
    const blob = await $fetch<Blob>(fileUrl(asset), { responseType: 'blob' })
    const file = new File([blob], asset.fileName, { type: asset.contentType })
    await api.uploadCertificatePrintAsset(kind, file, widths[kind])
    await refresh()
  } catch (widthError) {
    actionError.value = getApiErrorMessage(widthError, 'Nie udało się zmienić szerokości.')
  } finally {
    busyKind.value = null
  }
}

watch(assets, (value) => {
  for (const asset of value) {
    widths[asset.kind] = asset.printWidthMm
  }
}, { immediate: true })
</script>

<template>
  <section class="space-y-6">
    <div class="space-y-2">
      <h1 class="text-2xl font-semibold tracking-tight text-slate-900">Nadruki zaświadczeń</h1>
      <p class="max-w-3xl text-sm leading-6 text-slate-600">
        Pieczątki i podpis trafiają wyłącznie na zaświadczenia wystawiane przez platformę
        e-learningową — takie dokumenty kursant dostaje elektronicznie i nikt nie przystawia
        na nich pieczątki ręcznie. Wydruki z aplikacji idą na papier firmowy i zostają bez zmian.
      </p>
      <p class="max-w-3xl text-sm leading-6 text-slate-600">
        Najlepszy plik to <strong>PNG z przezroczystym tłem</strong>. Pieczątka na białym
        prostokącie zasłoni gilosz pod spodem.
      </p>
    </div>

    <div v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700">
      {{ loadError }}
    </div>
    <div v-if="actionError" class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700">
      {{ actionError }}
    </div>

    <div v-if="pending" class="rounded-xl border border-slate-200 bg-white/90 px-6 py-10 text-sm text-slate-500">
      Wczytywanie...
    </div>

    <div v-else class="grid gap-4 lg:grid-cols-3">
      <div
        v-for="item in kinds"
        :key="item.kind"
        class="flex flex-col gap-4 rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm"
      >
        <div>
          <h2 class="text-sm font-semibold text-slate-900">{{ item.label }}</h2>
          <p class="mt-1 text-xs leading-5 text-slate-500">{{ item.hint }}</p>
        </div>

        <div class="flex min-h-32 items-center justify-center rounded-lg border border-dashed border-slate-300 bg-slate-50 p-3">
          <img
            v-if="assetFor(item.kind)"
            :src="fileUrl(assetFor(item.kind)!)"
            :alt="item.label"
            class="max-h-28 w-auto"
          >
          <span v-else class="text-xs text-slate-400">Brak pliku</span>
        </div>

        <dl v-if="assetFor(item.kind)" class="space-y-1 text-xs text-slate-500">
          <div class="flex justify-between gap-2">
            <dt>Plik</dt>
            <dd class="truncate text-slate-700">{{ assetFor(item.kind)!.fileName }}</dd>
          </div>
          <div class="flex justify-between gap-2">
            <dt>Rozmiar</dt>
            <dd class="text-slate-700">{{ formatSize(assetFor(item.kind)!.fileSize) }}</dd>
          </div>
          <div class="flex justify-between gap-2">
            <dt>Wgrano</dt>
            <dd class="text-slate-700">{{ assetFor(item.kind)!.uploadedAt }}</dd>
          </div>
        </dl>

        <div v-if="assetFor(item.kind)" class="space-y-1">
          <label class="text-xs font-medium text-slate-600" :for="`width-${item.kind}`">
            Szerokość na wydruku (mm)
          </label>
          <input
            :id="`width-${item.kind}`"
            v-model.number="widths[item.kind]"
            type="number"
            min="10"
            max="60"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            @change="onWidthChange(item.kind)"
          >
        </div>

        <div class="mt-auto flex flex-wrap gap-2">
          <label
            class="inline-flex cursor-pointer items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-400"
            :class="{ 'pointer-events-none opacity-60': busyKind === item.kind }"
          >
            {{ assetFor(item.kind) ? 'Podmień plik' : 'Wgraj plik' }}
            <input
              type="file"
              accept="image/png,image/jpeg"
              class="hidden"
              @change="onUpload(item.kind, $event)"
            >
          </label>

          <UButton
            v-if="assetFor(item.kind)"
            color="error"
            variant="outline"
            :loading="busyKind === item.kind"
            @click="onDelete(item.kind)"
          >
            Usuń
          </UButton>
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white/90 p-5 shadow-sm">
      <h2 class="text-sm font-semibold text-slate-900">Tło giloszowe</h2>
      <p class="mt-1 max-w-3xl text-xs leading-5 text-slate-500">
        Wzór pochodzi z blankietu organizatora i jest wbudowany w API — nie ma czego wgrywać
        ani konfigurować. Przód niesie monogram, odwrót samą ramkę z siatką.
      </p>
      <div class="mt-4 flex flex-wrap gap-4">
        <figure class="space-y-1">
          <img
            :src="guillocheFrontUrl"
            alt="Gilosz - przód"
            class="h-56 w-auto rounded border border-slate-200"
          >
          <figcaption class="text-xs text-slate-500">Przód</figcaption>
        </figure>
        <figure class="space-y-1">
          <img
            :src="guillocheBackUrl"
            alt="Gilosz - odwrót"
            class="h-56 w-auto rounded border border-slate-200"
          >
          <figcaption class="text-xs text-slate-500">Odwrót</figcaption>
        </figure>
      </div>
    </div>
  </section>
</template>
