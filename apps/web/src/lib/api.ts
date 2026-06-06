// Typed client for the Coin Hub JSON API. Cookies carry the session, so every call uses
// credentials: 'include'. Paths are relative (same-origin: dev proxy or nginx in production).

export interface User {
  id: number
  email: string
  display_name: string
}

export interface TradingSettings {
  trading_pair_symbol: string
  capital_threshold: number
  target_profit_percent: number
  stop_loss_percent: number | null
  auto_sell_interval_minutes: number
  daily_purchase_hour_utc: number
  live_trading_enabled: boolean
  active_binance_environment: string
}

export interface CredentialStatus {
  has_active_credential: boolean
  active_environment: string
  masked_api_key: string
  configured_environments: string[]
}

export interface Operation {
  id: number
  symbol: string
  quantity: number
  purchase_price_per_unit: number
  target_profit_percent: number
  status: string
  sell_price_per_unit: number | null
  sell_target_price_per_unit: number | null
  buy_order_id: string | null
  sell_order_id: string | null
  purchased_at: string
  sold_at: string | null
}

export interface Execution {
  id: number
  symbol: string
  operation_type: string
  unit_price: number
  quantity: number
  total_value: number
  executed_at: string
  success: boolean
  error_message: string | null
  order_id: string | null
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined
  })
  const rawText = await response.text()
  const data = rawText ? JSON.parse(rawText) : null
  if (!response.ok) {
    const message = data && typeof data.error === 'string' ? data.error : `Request failed (${response.status})`
    throw new Error(message)
  }
  return data as T
}

export const api = {
  signup: (email: string, password: string, displayName: string) =>
    request<User>('POST', '/auth/signup', { email, password, display_name: displayName }),
  login: (email: string, password: string) =>
    request<User>('POST', '/auth/login', { email, password }),
  logout: () => request<{ message: string }>('POST', '/auth/logout'),
  me: () => request<User>('GET', '/auth/me'),

  getSettings: () => request<TradingSettings>('GET', '/api/v1/settings'),
  saveSettings: (settings: TradingSettings) => request<TradingSettings>('PUT', '/api/v1/settings', settings),

  getCredentials: () => request<CredentialStatus>('GET', '/api/v1/binance/credentials'),
  saveCredentials: (environment: string, apiKey: string, apiSecret: string) =>
    request<{ message: string }>('POST', '/api/v1/binance/credentials', {
      environment,
      api_key: apiKey,
      api_secret: apiSecret
    }),
  activateEnvironment: (environment: string) =>
    request<{ message: string }>('POST', '/api/v1/binance/credentials/activate', { environment }),

  getPrice: (symbol: string) =>
    request<{ symbol: string; price: number }>('GET', `/api/v1/binance/price?symbol=${encodeURIComponent(symbol)}`),
  getSymbols: () => request<{ symbols: string[] }>('GET', '/api/v1/binance/symbols'),

  getOperations: () => request<Operation[]>('GET', '/api/v1/operations'),
  getExecutions: () => request<Execution[]>('GET', '/api/v1/operations/executions'),
  buy: (symbol: string, quoteAmount: number, targetProfitPercent: number) =>
    request<Operation>('POST', '/api/v1/operations', {
      symbol,
      quote_amount: quoteAmount,
      target_profit_percent: targetProfitPercent
    })
}
