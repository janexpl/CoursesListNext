import { getRequestURL, getRouterParam, proxyRequest } from 'h3'
import { joinURL } from 'ufo'

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event)
  const path = getRouterParam(event, 'path') || ''
  const requestURL = getRequestURL(event)

  const targetURL = joinURL(config.apiTarget, 'api', path) + requestURL.search

  return await proxyRequest(event, targetURL)
})
