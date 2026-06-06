import { writable, derived } from 'svelte/store'

export type Locale = 'en' | 'pt' | 'es'

export const availableLocales: { code: Locale; label: string }[] = [
  { code: 'pt', label: 'PT' },
  { code: 'en', label: 'EN' },
  { code: 'es', label: 'ES' }
]

const STORAGE_KEY = 'coinhub_locale'

// Auto-detect: explicit saved choice wins, otherwise the browser's preferred languages.
function detectInitialLocale(): Locale {
  if (typeof localStorage !== 'undefined') {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'en' || saved === 'pt' || saved === 'es') return saved
  }
  const candidates =
    typeof navigator !== 'undefined' ? (navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language]) : []
  for (const candidate of candidates) {
    const normalized = (candidate || '').toLowerCase()
    if (normalized.startsWith('pt')) return 'pt'
    if (normalized.startsWith('es')) return 'es'
    if (normalized.startsWith('en')) return 'en'
  }
  return 'en'
}

export const locale = writable<Locale>(detectInitialLocale())

export function setLocale(next: Locale) {
  locale.set(next)
}

locale.subscribe((value) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, value)
  if (typeof document !== 'undefined') document.documentElement.lang = value
})

type Dictionary = Record<string, string>

const en: Dictionary = {
  'app.loading': 'Loading…',
  'login.tagline': 'Crypto trading automation and your B3 portfolio, in one place.',
  'login.signIn': 'Sign in',
  'login.createAccount': 'Create account',
  'login.name': 'Name',
  'login.namePlaceholder': 'Your name',
  'login.email': 'Email',
  'login.password': 'Password',
  'login.passwordPlaceholder': 'At least 8 characters',
  'login.wait': 'Please wait…',
  'header.binance': 'Binance:',
  'header.notConnected': 'not connected',
  'header.signOut': 'Sign out',
  'help.summary': 'How it works',
  'start.title': 'Getting started',
  'start.intro': 'Coin Hub automates crypto trades on Binance and shows your B3 portfolio. Quick path:',
  'start.s1': 'Connect Binance — start on Testnet to practice with fake money.',
  'start.s2': 'Set your bot settings — capital, profit target and stop-loss.',
  'start.s3': 'Place a buy, or let the daily auto-buy (DCA) run for you.',
  'start.s4': 'Optionally add your Investidor10 wallet to see your B3 portfolio.',
  'binance.title': 'Binance connection',
  'binance.subtitle': 'Link your Binance account so Coin Hub can trade for you.',
  'binance.help':
    'Coin Hub trades through your Binance account using API keys (a key + secret you generate on Binance). It never sees your password and cannot withdraw funds. Begin on Testnet — Binance’s free practice exchange that uses fake money. Testnet needs its OWN key and secret, created at testnet.binance.vision; the fields cannot be empty. When you’re ready for real money, create trade-only keys (withdrawals disabled) on binance.com and switch the environment to Production.',
  'binance.environment': 'Environment',
  'binance.testnet': 'Testnet (practice)',
  'binance.production': 'Production (real money)',
  'binance.apiKey': 'API key',
  'binance.apiSecret': 'API secret',
  'binance.save': 'Validate & save',
  'binance.saving': 'Validating…',
  'binance.activePrefix': 'Active',
  'binance.connectHint': 'Use trade-only keys (withdrawals disabled). New accounts start on Testnet.',
  'binance.validatedSaved': 'Validated and saved.',
  'buy.title': 'Buy',
  'buy.subtitle': 'Market buy plus a take-profit limit sell at your target.',
  'buy.help':
    'A market buy purchases the selected pair instantly at the current price. Coin Hub then places a take-profit limit sell at your target % above the buy price, so if the price reaches it the position closes in profit automatically, at exchange speed. The amount is in the quote currency (for BTCUSDT, that’s USDT).',
  'buy.pair': 'Pair',
  'buy.currentPrice': 'Current price: {price}',
  'buy.amount': 'Amount (quote currency)',
  'buy.target': 'Target profit %',
  'buy.button': 'Buy + set take-profit',
  'buy.placing': 'Placing…',
  'buy.bought': 'Bought {qty} {symbol} @ {price}.',
  'settings.title': 'Bot settings',
  'settings.subtitle': 'Defaults and rules the automation follows.',
  'settings.help':
    'These drive the automation. Capital per buy and Target profit % are the defaults applied to new buys. Stop-loss % market-sells a position if its price falls that far below your buy price (leave empty to disable). Daily buy hour (UTC) runs one automatic recurring buy (DCA) per day at that hour. Enable live trading must be ON before any real-money (Production) order is allowed — on Testnet it is ignored. The bot reviews your open positions about every 30 seconds.',
  'settings.defaultPair': 'Default pair',
  'settings.capital': 'Capital per buy',
  'settings.target': 'Target profit %',
  'settings.stopLoss': 'Stop-loss %',
  'settings.stopLossNone': 'none',
  'settings.dailyHour': 'Daily buy hour (UTC)',
  'settings.enableLive': 'Enable live (real-money) trading',
  'settings.save': 'Save settings',
  'settings.saving': 'Saving…',
  'settings.saved': 'Settings saved.',
  'alloc.title': 'Open allocation',
  'alloc.help': 'Shows how your currently open positions split across trading pairs, by the capital put into each.',
  'alloc.none': 'No open positions yet.',
  'ops.title': 'Operations',
  'ops.none': 'No operations yet. Connect Binance and place your first buy.',
  'ops.pair': 'Pair',
  'ops.status': 'Status',
  'ops.qty': 'Qty',
  'ops.buyPrice': 'Buy price',
  'ops.target': 'Target',
  'ops.purchased': 'Purchased',
  'portfolio.title': 'B3 portfolio (Investidor10)',
  'portfolio.subtitle': 'See your stocks/FIIs and upcoming dividend dates.',
  'portfolio.help':
    'Paste the public URL of your Investidor10 wallet. Coin Hub reads it (read-only) to list your stocks/FIIs and the upcoming ex-dividend (data-com) dates. It uses a headless browser, so loading can take up to a minute.',
  'portfolio.placeholder': 'https://investidor10.com.br/carteiras/...',
  'portfolio.saveUrl': 'Save URL',
  'portfolio.loadAssets': 'Load assets',
  'portfolio.dividends': 'Dividend dates',
  'portfolio.loading': 'Loading…',
  'portfolio.upcoming': 'Upcoming ex-dividend dates',
  'portfolio.asset': 'Asset',
  'portfolio.date': 'Date',
  'portfolio.saved': 'Saved.',
  'common.saving': 'Saving…'
}

const pt: Dictionary = {
  'app.loading': 'Carregando…',
  'login.tagline': 'Automação de trade de cripto e sua carteira da B3, em um só lugar.',
  'login.signIn': 'Entrar',
  'login.createAccount': 'Criar conta',
  'login.name': 'Nome',
  'login.namePlaceholder': 'Seu nome',
  'login.email': 'E-mail',
  'login.password': 'Senha',
  'login.passwordPlaceholder': 'Pelo menos 8 caracteres',
  'login.wait': 'Aguarde…',
  'header.binance': 'Binance:',
  'header.notConnected': 'não conectada',
  'header.signOut': 'Sair',
  'help.summary': 'Como funciona',
  'start.title': 'Primeiros passos',
  'start.intro': 'O Coin Hub automatiza operações de cripto na Binance e mostra sua carteira da B3. Caminho rápido:',
  'start.s1': 'Conecte a Binance — comece no Testnet para praticar com dinheiro fictício.',
  'start.s2': 'Defina os ajustes do robô — capital, alvo de lucro e stop-loss.',
  'start.s3': 'Faça uma compra, ou deixe a compra diária automática (DCA) rodar por você.',
  'start.s4': 'Opcional: adicione sua carteira do Investidor10 para ver seu portfólio da B3.',
  'binance.title': 'Conexão Binance',
  'binance.subtitle': 'Vincule sua conta Binance para o Coin Hub negociar por você.',
  'binance.help':
    'O Coin Hub negocia pela sua conta Binance usando chaves de API (uma key + secret geradas na Binance). Ele nunca vê sua senha e não consegue sacar. Comece no Testnet — a corretora de testes gratuita da Binance, com dinheiro fictício. O Testnet precisa da PRÓPRIA key e secret, criadas em testnet.binance.vision; os campos não podem ficar vazios. Quando quiser usar dinheiro real, crie chaves só de negociação (saque desativado) em binance.com e mude o ambiente para Produção.',
  'binance.environment': 'Ambiente',
  'binance.testnet': 'Testnet (prática)',
  'binance.production': 'Produção (dinheiro real)',
  'binance.apiKey': 'API key',
  'binance.apiSecret': 'API secret',
  'binance.save': 'Validar e salvar',
  'binance.saving': 'Validando…',
  'binance.activePrefix': 'Ativa',
  'binance.connectHint': 'Use chaves só de negociação (saque desativado). Contas novas começam no Testnet.',
  'binance.validatedSaved': 'Validada e salva.',
  'buy.title': 'Comprar',
  'buy.subtitle': 'Compra a mercado + venda-limite de realização no seu alvo.',
  'buy.help':
    'A compra a mercado adquire o par escolhido na hora, ao preço atual. Em seguida o Coin Hub coloca uma venda-limite de realização no seu alvo de % acima do preço de compra; se o preço chegar lá, a posição é fechada com lucro automaticamente, na velocidade da corretora. O valor é na moeda de cotação (para BTCUSDT, é USDT).',
  'buy.pair': 'Par',
  'buy.currentPrice': 'Preço atual: {price}',
  'buy.amount': 'Valor (moeda de cotação)',
  'buy.target': 'Alvo de lucro %',
  'buy.button': 'Comprar + definir realização',
  'buy.placing': 'Enviando…',
  'buy.bought': 'Comprado {qty} {symbol} @ {price}.',
  'settings.title': 'Ajustes do robô',
  'settings.subtitle': 'Padrões e regras que a automação segue.',
  'settings.help':
    'Estes comandam a automação. Capital por compra e Alvo de lucro % são os padrões aplicados a novas compras. Stop-loss % vende a mercado uma posição se o preço cair essa % abaixo do preço de compra (deixe vazio para desativar). Hora da compra diária (UTC) faz uma compra recorrente automática (DCA) por dia naquele horário. Ativar trading real precisa estar LIGADO para permitir qualquer ordem com dinheiro real (Produção) — no Testnet é ignorado. O robô revisa suas posições abertas a cada ~30 segundos.',
  'settings.defaultPair': 'Par padrão',
  'settings.capital': 'Capital por compra',
  'settings.target': 'Alvo de lucro %',
  'settings.stopLoss': 'Stop-loss %',
  'settings.stopLossNone': 'nenhum',
  'settings.dailyHour': 'Hora da compra diária (UTC)',
  'settings.enableLive': 'Ativar trading real (dinheiro real)',
  'settings.save': 'Salvar ajustes',
  'settings.saving': 'Salvando…',
  'settings.saved': 'Ajustes salvos.',
  'alloc.title': 'Alocação aberta',
  'alloc.help': 'Mostra como suas posições abertas se dividem entre os pares, pelo capital aplicado em cada uma.',
  'alloc.none': 'Nenhuma posição aberta ainda.',
  'ops.title': 'Operações',
  'ops.none': 'Nenhuma operação ainda. Conecte a Binance e faça sua primeira compra.',
  'ops.pair': 'Par',
  'ops.status': 'Status',
  'ops.qty': 'Qtd',
  'ops.buyPrice': 'Preço compra',
  'ops.target': 'Alvo',
  'ops.purchased': 'Comprado em',
  'portfolio.title': 'Carteira B3 (Investidor10)',
  'portfolio.subtitle': 'Veja suas ações/FIIs e as próximas datas de dividendos.',
  'portfolio.help':
    'Cole a URL pública da sua carteira no Investidor10. O Coin Hub lê (somente leitura) para listar suas ações/FIIs e as próximas datas-com (ex-dividendo). Ele usa um navegador headless, então pode levar até um minuto.',
  'portfolio.placeholder': 'https://investidor10.com.br/carteiras/...',
  'portfolio.saveUrl': 'Salvar URL',
  'portfolio.loadAssets': 'Carregar ativos',
  'portfolio.dividends': 'Datas de dividendos',
  'portfolio.loading': 'Carregando…',
  'portfolio.upcoming': 'Próximas datas-com',
  'portfolio.asset': 'Ativo',
  'portfolio.date': 'Data',
  'portfolio.saved': 'Salvo.',
  'common.saving': 'Salvando…'
}

const es: Dictionary = {
  'app.loading': 'Cargando…',
  'login.tagline': 'Automatización de trading de cripto y tu cartera de la B3, en un solo lugar.',
  'login.signIn': 'Iniciar sesión',
  'login.createAccount': 'Crear cuenta',
  'login.name': 'Nombre',
  'login.namePlaceholder': 'Tu nombre',
  'login.email': 'Correo',
  'login.password': 'Contraseña',
  'login.passwordPlaceholder': 'Al menos 8 caracteres',
  'login.wait': 'Espera…',
  'header.binance': 'Binance:',
  'header.notConnected': 'no conectada',
  'header.signOut': 'Salir',
  'help.summary': 'Cómo funciona',
  'start.title': 'Primeros pasos',
  'start.intro': 'Coin Hub automatiza operaciones de cripto en Binance y muestra tu cartera de la B3. Camino rápido:',
  'start.s1': 'Conecta Binance — empieza en Testnet para practicar con dinero ficticio.',
  'start.s2': 'Configura el bot — capital, objetivo de ganancia y stop-loss.',
  'start.s3': 'Haz una compra, o deja que la compra diaria automática (DCA) lo haga por ti.',
  'start.s4': 'Opcional: añade tu cartera de Investidor10 para ver tu portafolio de la B3.',
  'binance.title': 'Conexión Binance',
  'binance.subtitle': 'Vincula tu cuenta de Binance para que Coin Hub opere por ti.',
  'binance.help':
    'Coin Hub opera a través de tu cuenta de Binance usando claves de API (una key + secret que generas en Binance). Nunca ve tu contraseña y no puede retirar fondos. Empieza en Testnet — el exchange de práctica gratuito de Binance con dinero ficticio. Testnet necesita su PROPIA key y secret, creadas en testnet.binance.vision; los campos no pueden quedar vacíos. Cuando quieras dinero real, crea claves solo de trading (retiros deshabilitados) en binance.com y cambia el entorno a Producción.',
  'binance.environment': 'Entorno',
  'binance.testnet': 'Testnet (práctica)',
  'binance.production': 'Producción (dinero real)',
  'binance.apiKey': 'API key',
  'binance.apiSecret': 'API secret',
  'binance.save': 'Validar y guardar',
  'binance.saving': 'Validando…',
  'binance.activePrefix': 'Activa',
  'binance.connectHint': 'Usa claves solo de trading (retiros deshabilitados). Las cuentas nuevas empiezan en Testnet.',
  'binance.validatedSaved': 'Validada y guardada.',
  'buy.title': 'Comprar',
  'buy.subtitle': 'Compra a mercado + venta límite de toma de ganancias en tu objetivo.',
  'buy.help':
    'La compra a mercado adquiere el par elegido al instante, al precio actual. Luego Coin Hub coloca una venta límite de toma de ganancias en tu objetivo de % por encima del precio de compra; si el precio llega, la posición se cierra en ganancia automáticamente, a la velocidad del exchange. El monto es en la moneda de cotización (para BTCUSDT, es USDT).',
  'buy.pair': 'Par',
  'buy.currentPrice': 'Precio actual: {price}',
  'buy.amount': 'Monto (moneda de cotización)',
  'buy.target': 'Objetivo de ganancia %',
  'buy.button': 'Comprar + fijar toma de ganancias',
  'buy.placing': 'Enviando…',
  'buy.bought': 'Comprado {qty} {symbol} @ {price}.',
  'settings.title': 'Ajustes del bot',
  'settings.subtitle': 'Valores por defecto y reglas que sigue la automatización.',
  'settings.help':
    'Estos controlan la automatización. Capital por compra y Objetivo de ganancia % son los valores por defecto para nuevas compras. Stop-loss % vende a mercado una posición si su precio cae ese % por debajo del precio de compra (déjalo vacío para desactivar). Hora de compra diaria (UTC) hace una compra recurrente automática (DCA) por día a esa hora. Activar trading real debe estar ENCENDIDO antes de cualquier orden con dinero real (Producción) — en Testnet se ignora. El bot revisa tus posiciones abiertas cada ~30 segundos.',
  'settings.defaultPair': 'Par por defecto',
  'settings.capital': 'Capital por compra',
  'settings.target': 'Objetivo de ganancia %',
  'settings.stopLoss': 'Stop-loss %',
  'settings.stopLossNone': 'ninguno',
  'settings.dailyHour': 'Hora de compra diaria (UTC)',
  'settings.enableLive': 'Activar trading real (dinero real)',
  'settings.save': 'Guardar ajustes',
  'settings.saving': 'Guardando…',
  'settings.saved': 'Ajustes guardados.',
  'alloc.title': 'Asignación abierta',
  'alloc.help': 'Muestra cómo se reparten tus posiciones abiertas entre los pares, según el capital invertido en cada una.',
  'alloc.none': 'Aún no hay posiciones abiertas.',
  'ops.title': 'Operaciones',
  'ops.none': 'Aún no hay operaciones. Conecta Binance y haz tu primera compra.',
  'ops.pair': 'Par',
  'ops.status': 'Estado',
  'ops.qty': 'Cant.',
  'ops.buyPrice': 'Precio compra',
  'ops.target': 'Objetivo',
  'ops.purchased': 'Comprado el',
  'portfolio.title': 'Cartera B3 (Investidor10)',
  'portfolio.subtitle': 'Mira tus acciones/FIIs y las próximas fechas de dividendos.',
  'portfolio.help':
    'Pega la URL pública de tu cartera en Investidor10. Coin Hub la lee (solo lectura) para listar tus acciones/FIIs y las próximas fechas ex-dividendo (data-com). Usa un navegador headless, así que puede tardar hasta un minuto.',
  'portfolio.placeholder': 'https://investidor10.com.br/carteiras/...',
  'portfolio.saveUrl': 'Guardar URL',
  'portfolio.loadAssets': 'Cargar activos',
  'portfolio.dividends': 'Fechas de dividendos',
  'portfolio.loading': 'Cargando…',
  'portfolio.upcoming': 'Próximas fechas ex-dividendo',
  'portfolio.asset': 'Activo',
  'portfolio.date': 'Fecha',
  'portfolio.saved': 'Guardado.',
  'common.saving': 'Guardando…'
}

const dictionaries: Record<Locale, Dictionary> = { en, pt, es }

export const t = derived(locale, ($locale) => {
  return (key: string, vars?: Record<string, string | number>): string => {
    let message = dictionaries[$locale][key] ?? dictionaries.en[key] ?? key
    if (vars) {
      for (const variableName of Object.keys(vars)) {
        message = message.replace(new RegExp(`\\{${variableName}\\}`, 'g'), String(vars[variableName]))
      }
    }
    return message
  }
})
