import { createError, getRequestURL, getRouterParam, proxyRequest } from 'h3'
import { joinURL } from 'ufo'

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event)
  const path = getRouterParam(event, 'path') || ''
  const requestURL = getRequestURL(event)

  // Internal API endpoints (e.g. notification candidates) are reached directly
  // by trusted services, never through this public-facing proxy. Hide them so
  // they cannot be probed from the browser.
  if (path.split('/').includes('internal')) {
    throw createError({ statusCode: 404, statusMessage: 'Not Found' })
  }

  const targetURL = joinURL(config.apiTarget, 'api', path) + requestURL.search

  return await proxyRequest(event, targetURL)
})
