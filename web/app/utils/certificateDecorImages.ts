import type { CertificatePrintDecor, CertificatePrintImage } from '~/composables/useApi'

// Nadruki wydruku pobrane do postaci data URI.
//
// Podgląd zaświadczenia i przycisk "Drukuj" muszą mieć obrazy gotowe w treści dokumentu,
// a nie pod adresem: print() woła się natychmiast po otwarciu ramki i nie zdąży poczekać
// na pobranie plików. Tak samo rozwiązany jest kod QR, który przychodzi z API od razu
// jako data URI.

type DecorImage = { dataUri: string, widthMm: number }

export type ResolvedDecor = {
  stampRound: DecorImage | null
  stampCompany: DecorImage | null
  stampPersonal: DecorImage | null
  signature: DecorImage | null
  guillocheFront: string
  guillocheBack: string
}

// Cache na poziomie modułu: te same pliki wracają przy każdym otwarciu zaświadczenia,
// a adres niesie znacznik czasu wgrania, więc po podmianie pieczątki klucz się zmienia.
const cache = new Map<string, Promise<string>>()

async function toDataUri(url: string): Promise<string> {
  const blob = await $fetch<Blob>(url, { responseType: 'blob' })

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })
}

export function loadDecorDataUri(url: string): Promise<string> {
  const cached = cache.get(url)
  if (cached) {
    return cached
  }

  const pending = toDataUri(url)
  cache.set(url, pending)
  // Nieudanego pobrania nie zapamiętujemy - następne wejście ma spróbować ponownie.
  pending.catch(() => cache.delete(url))
  return pending
}

async function resolveImage(image: CertificatePrintImage | null): Promise<DecorImage | null> {
  if (!image) {
    return null
  }

  return { dataUri: await loadDecorDataUri(image.url), widthMm: image.widthMm }
}

export async function loadCertificateDecorImages(decor: CertificatePrintDecor): Promise<ResolvedDecor> {
  const [stampRound, stampCompany, stampPersonal, signature, guillocheFront, guillocheBack] = await Promise.all([
    resolveImage(decor.stampRound),
    resolveImage(decor.stampCompany),
    resolveImage(decor.stampPersonal),
    resolveImage(decor.signature),
    loadDecorDataUri(decor.guillocheFrontUrl),
    loadDecorDataUri(decor.guillocheBackUrl)
  ])

  return { stampRound, stampCompany, stampPersonal, signature, guillocheFront, guillocheBack }
}
