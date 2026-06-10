import { writable } from 'svelte/store'
import type { User } from './api'

export const currentUser = writable<User | null>(null)

// Binance connection status, surfaced in the top nav. The Dashboard populates it after loading
// credentials so the header can show the active environment from anywhere.
export const binanceStatus = writable<{ has_active_credential: boolean; active_environment: string } | null>(null)

// Minimal hash-based routing — enough for the authenticated views + the email-link pages, without
// pulling in a router. `reset` and `verify` are reached from email links (#/reset?token=…).
export type Route = 'dashboard' | 'account' | 'reset' | 'verify'

function pathFromHash(): string {
  if (typeof location === 'undefined') return ''
  return location.hash.replace(/^#\/?/, '').split('?')[0]
}

function routeFromHash(): Route {
  switch (pathFromHash()) {
    case 'settings':
      return 'account'
    case 'reset':
      return 'reset'
    case 'verify':
      return 'verify'
    default:
      return 'dashboard'
  }
}

// hashToken extracts ?token=… from the current hash (used by the reset/verify pages).
export function hashToken(): string {
  if (typeof location === 'undefined') return ''
  const questionMarkIndex = location.hash.indexOf('?')
  if (questionMarkIndex < 0) return ''
  return new URLSearchParams(location.hash.slice(questionMarkIndex + 1)).get('token') ?? ''
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
