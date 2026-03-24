export default defineNuxtRouteMiddleware(async () => {
  const auth = useAuth()

  if (auth.user.value) {
    return
  }

  try {
    await auth.fetchMe()
  } catch {
    return navigateTo('/login')
  }
})
