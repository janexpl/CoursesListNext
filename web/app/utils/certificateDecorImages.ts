import type { CertificatePrintDecor } from '~/composables/useApi'

// Nadruki wydruku pobrane do postaci data URI.
//
// Podgląd zaświadczenia i przycisk "Drukuj" muszą mieć obrazy gotowe w treści dokumentu,
// a nie pod adresem: print() woła się natychmiast po otwarciu ramki i nie zdąży poczekać
// na pobranie plików. Tak samo rozwiązany jest kod QR, który przychodzi z API od razu
// jako data URI.

export type ResolvedDecor = {
  stamp1: { dataUri: string, widthMm: number } | null
  stamp2: { dataUri: string, widthMm: number } | null
  signature: { dataUri: string, widthMm: number } | null
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

export async function loadCertificateDecorImages(decor: CertificatePrintDecor): Promise<ResolvedDecor> {
  const [stamp1, stamp2, signature, guillocheFront, guillocheBack] = await Promise.all([
    decor.stamp1 ? loadDecorDataUri(decor.stamp1.url) : Promise.resolve(''),
    decor.stamp2 ? loadDecorDataUri(decor.stamp2.url) : Promise.resolve(''),
    decor.signature ? loadDecorDataUri(decor.signature.url) : Promise.resolve(''),
    loadDecorDataUri(decor.guillocheFrontUrl),
    loadDecorDataUri(decor.guillocheBackUrl)
  ])

  return {
    stamp1: stamp1 && decor.stamp1 ? { dataUri: stamp1, widthMm: decor.stamp1.widthMm } : null,
    stamp2: stamp2 && decor.stamp2 ? { dataUri: stamp2, widthMm: decor.stamp2.widthMm } : null,
    signature: signature && decor.signature ? { dataUri: signature, widthMm: decor.signature.widthMm } : null,
    guillocheFront,
    guillocheBack
  }
}
