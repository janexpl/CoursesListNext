export default defineNuxtRouteMiddleware(async () => {
  const auth = useAuth()

  if (auth.user.value) {
    return navigateTo('/')
  }

  try {
    await auth.fetchMe()
    return navigateTo('/')
  } catch {
    return
  }
})
