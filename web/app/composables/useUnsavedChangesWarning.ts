import { onBeforeUnmount, onMounted, toValue } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { onBeforeRouteLeave } from '#imports'

const DEFAULT_MESSAGE = 'Masz niezapisane zmiany. Czy na pewno chcesz opuścić formularz?'

export function useUnsavedChangesWarning(
  isDirty: MaybeRefOrGetter<boolean>,
  message = DEFAULT_MESSAGE
) {
  const handleBeforeUnload = (event: BeforeUnloadEvent) => {
    if (!toValue(isDirty)) {
      return
    }

    event.preventDefault()
    event.returnValue = ''
  }

  onMounted(() => {
    window.addEventListener('beforeunload', handleBeforeUnload)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', handleBeforeUnload)
  })

  onBeforeRouteLeave(() => {
    if (!toValue(isDirty)) {
      return true
    }

    return window.confirm(message)
  })
}
