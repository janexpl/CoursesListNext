export default defineNuxtRouteMiddleware(async () => {
  const auth = useAuth()

  if (!auth.user.value) {
    try {
      await auth.fetchMe()
    } catch {
      return navigateTo('/login')
    }
  }

  if (auth.user.value?.role !== 1) {
    return navigateTo('/')
  }
})
