import type { Ref } from 'vue'

type UseSearchableSelectOptions<T> = {
  query: Ref<string>
  fetchOptions: (normalizedQuery: string) => Promise<T[]>
  getOptionLabel: (option: T) => string
  getErrorMessage: (error: unknown) => string
  minQueryLength?: number
  debounceMs?: number
}

export function useSearchableSelect<T>(options: UseSearchableSelectOptions<T>) {
  const minQueryLength = options.minQueryLength ?? 2
  const debounceMs = options.debounceMs ?? 250
  const selectedOption = ref<T | null>(null)
  const availableOptions = ref<T[]>([])
  const pending = ref(false)
  const error = ref('')
  const normalizedQuery = computed(() => options.query.value.trim())
  const showNoResults = computed(() => {
    return (
      normalizedQuery.value.length >= minQueryLength
      && !selectedOption.value
      && !pending.value
      && !availableOptions.value.length
      && !error.value
    )
  })

  let searchTimer: ReturnType<typeof setTimeout> | undefined
  let requestId = 0

  function clearResults() {
    availableOptions.value = []
    pending.value = false
    error.value = ''
  }

  function initializeSelection(option: T | null) {
    requestId += 1
    selectedOption.value = option
    options.query.value = option ? options.getOptionLabel(option) : ''
    clearResults()
  }

  function selectOption(option: T) {
    initializeSelection(option)
  }

  function clearSelection() {
    initializeSelection(null)
  }

  async function runSearch(query: string) {
    const normalized = query.trim()

    if (normalized.length < minQueryLength) {
      clearResults()
      return
    }

    const currentRequestId = ++requestId
    pending.value = true

    try {
      const nextOptions = await options.fetchOptions(normalized)

      if (currentRequestId !== requestId) {
        return
      }

      availableOptions.value = nextOptions
      error.value = ''
    } catch (caughtError: unknown) {
      if (currentRequestId !== requestId) {
        return
      }

      availableOptions.value = []
      error.value = options.getErrorMessage(caughtError)
    } finally {
      if (currentRequestId === requestId) {
        pending.value = false
      }
    }
  }

  async function refreshOptions() {
    await runSearch(options.query.value)
  }

  watch(options.query, (value) => {
    const selectedLabel = selectedOption.value
      ? options.getOptionLabel(selectedOption.value)
      : null

    if (selectedLabel && value !== selectedLabel) {
      selectedOption.value = null
    }

    if (searchTimer) {
      clearTimeout(searchTimer)
    }

    if (selectedLabel && value === selectedLabel) {
      clearResults()
      return
    }

    if (value.trim().length < minQueryLength) {
      clearResults()
      return
    }

    searchTimer = setTimeout(() => {
      void runSearch(value)
    }, debounceMs)
  })

  onBeforeUnmount(() => {
    if (searchTimer) {
      clearTimeout(searchTimer)
    }
  })

  return {
    selectedOption,
    options: availableOptions,
    pending,
    error,
    normalizedQuery,
    showNoResults,
    initializeSelection,
    selectOption,
    clearSelection,
    refreshOptions
  }
}
