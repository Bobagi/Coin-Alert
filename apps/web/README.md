# apps/web — SvelteKit frontend (Phase 4)

Placeholder. The dashboard SPA is scaffolded in Phase 4. It will talk to the Go API
(`apps/api`) over JSON and replace the legacy Go `html/template` dashboard.

Planned:
- Auth screens (sign up / sign in).
- Crypto panel: credentials, buy / DCA / alerts, open orders, execution history.
- B3 portfolio: holdings, order history, dividend calendar.
- Charts that don't exist today: allocation donut, PnL over time, price history,
  dividend calendar.

Tooling: SvelteKit + Vite, pnpm, a charting lib (Chart.js or ECharts), built to a static
or Node adapter and served behind the existing nginx vhost.
