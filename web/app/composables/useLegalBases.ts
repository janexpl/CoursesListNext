// Lista podstaw prawnych współdzielona przez formularz kursu i komponent wyboru -
// wspólny klucz useAsyncData sprawia, że dodanie nowej podstawy odświeża oba miejsca.
export function useLegalBases() {
  const api = useApi()
  const { data, pending, error, refresh } = useAsyncData('legal-bases', async () => await api.legalBases())
  const legalBases = computed(() => data.value?.data ?? [])

  function findLegalBasis(id: number | null | undefined) {
    if (id === null || id === undefined) {
      return null
    }
    return legalBases.value.find(basis => basis.id === id) ?? null
  }

  return { legalBases, pending, error, refresh, findLegalBasis }
}
