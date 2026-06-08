<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'
  import { currentUser } from './stores'
  import { t } from './i18n'
  import LanguageDropdown from './LanguageDropdown.svelte'
  import LegalFooter from './LegalFooter.svelte'

  let mode: 'login' | 'signup' = 'login'
  let email = ''
  let password = ''
  let displayName = ''
  let error = ''
  let busy = false
  let googleEnabled = false

  onMount(async () => {
    // The Google callback bounces back here with ?login_error=... when something went wrong.
    const params = new URLSearchParams(location.search)
    if (params.get('login_error')) {
      error = $t('login.googleError')
      history.replaceState(null, '', location.pathname + location.hash)
    }
    try {
      googleEnabled = (await api.getAuthProviders()).google
    } catch {
      googleEnabled = false
    }
  })

  async function submit() {
    error = ''
    busy = true
    try {
      const user =
        mode === 'login' ? await api.login(email, password) : await api.signup(email, password, displayName)
      currentUser.set(user)
    } catch (e) {
      error = (e as Error).message
    } finally {
      busy = false
    }
  }

  function googleLogin() {
    window.location.href = '/auth/google/login'
  }
</script>

<div class="wrap">
  <div class="card auth">
    <div class="top">
      <div class="brand">Coin<span>Hub</span></div>
      <LanguageDropdown compact />
    </div>
    <p class="muted tagline">{$t('login.tagline')}</p>

    {#if googleEnabled}
      <button type="button" class="ghost btn-block google" on:click={googleLogin}>
        <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
          <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
          <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
          <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
          <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
        </svg>
        {$t('login.google')}
      </button>
      <div class="divider"><span>{$t('login.or')}</span></div>
    {/if}

    <div class="tabs">
      <button class="tab" class:active={mode === 'login'} on:click={() => (mode = 'login')}>{$t('login.signIn')}</button>
      <button class="tab" class:active={mode === 'signup'} on:click={() => (mode = 'signup')}>{$t('login.createAccount')}</button>
    </div>

    <form on:submit|preventDefault={submit}>
      {#if mode === 'signup'}
        <div class="field">
          <label for="name">{$t('login.name')}</label>
          <input id="name" bind:value={displayName} placeholder={$t('login.namePlaceholder')} />
        </div>
      {/if}
      <div class="field">
        <label for="email">{$t('login.email')}</label>
        <input id="email" type="email" bind:value={email} required placeholder="you@example.com" />
      </div>
      <div class="field">
        <label for="password">{$t('login.password')}</label>
        <input id="password" type="password" bind:value={password} required placeholder={$t('login.passwordPlaceholder')} />
      </div>
      {#if error}<p class="error mt-3">{error}</p>{/if}
      <button type="submit" class="btn-block mt-5" disabled={busy}>
        {busy ? $t('login.wait') : mode === 'login' ? $t('login.signIn') : $t('login.createAccount')}
      </button>
    </form>
  </div>
  <LegalFooter />
</div>

<style>
  .wrap { display: grid; place-items: center; min-height: 100vh; padding: var(--space-5); gap: var(--space-4); align-content: center; }
  .auth { width: 100%; max-width: 400px; }
  .top { display: flex; justify-content: space-between; align-items: center; }
  .brand { font-size: var(--text-xl); font-weight: 800; }
  .brand span { color: var(--brand); }
  .tagline { margin-top: var(--space-2); line-height: 1.5; }
  .google { margin-top: var(--space-5); }
  .divider { display: flex; align-items: center; gap: var(--space-3); margin: var(--space-4) 0; color: var(--muted); font-size: var(--text-xs); text-transform: uppercase; letter-spacing: 0.06em; }
  .divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: var(--border); }
  .tabs { display: flex; gap: var(--space-2); margin: var(--space-4) 0; }
  .tab { flex: 1; background: transparent; border: 1px solid var(--border); color: var(--muted); font-weight: 700; }
  .tab.active { background: var(--surface-2); color: var(--text); border-color: var(--brand); }
</style>
