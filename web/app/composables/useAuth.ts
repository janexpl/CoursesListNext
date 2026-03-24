export function useAuth() {
  const api = useApi()
  const user = useState<AuthUser | null>('auth:user', () => null)

  const isAuthenticated = computed(() => user.value !== null)

  async function fetchMe() {
    try {
      const response = await api.me()
      user.value = response.data
      return response.data
    } catch (error) {
      user.value = null
      throw error
    }
  }

  async function login(payload: LoginPayload) {
    const response = await api.login(payload)
    user.value = response.data
    return response.data
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      user.value = null
    }
  }

  return {
    user,
    isAuthenticated,
    fetchMe,
    login,
    logout
  }
}
