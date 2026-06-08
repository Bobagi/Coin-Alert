import { writable } from 'svelte/store'
import type { User } from './api'

export const currentUser = writable<User | null>(null)

// Binance connection status, surfaced in the top nav. The Dashboard populates it after loading
// credentials so the header can show the active environment from anywhere.
export const binanceStatus = writable<{ has_active_credential: boolean; active_environment: string } | null>(null)

// Minimal hash-based routing — enough for the two authenticated views without pulling in a router.
export type Route = 'dashboard' | 'account'

function routeFromHash(): Route {
  if (typeof location !== 'undefined' && location.hash.replace(/^#\/?/, '') === 'settings') return 'account'
  return 'dashboard'
}

export const route = writable<Route>(routeFromHash())

export function navigate(to: Route) {
  const hash = to === 'account' ? '#/settings' : '#/'
  if (typeof location !== 'undefined' && location.hash !== hash) location.hash = hash
  route.set(to)
}

if (typeof window !== 'undefined') {
  window.addEventListener('hashchange', () => route.set(routeFromHash()))
}
