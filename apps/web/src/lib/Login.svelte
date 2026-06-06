<script lang="ts">
  import { api } from './api'
  import { currentUser } from './stores'
  import { t } from './i18n'
  import LanguageSwitcher from './LanguageSwitcher.svelte'

  let mode: 'login' | 'signup' = 'login'
  let email = ''
  let password = ''
  let displayName = ''
  let error = ''
  let busy = false

  async function submit() {
    error = ''
    busy = true
    try {
      const user =
        mode === 'login'
          ? await api.login(email, password)
          : await api.signup(email, password, displayName)
      currentUser.set(user)
    } catch (e) {
      error = (e as Error).message
    } finally {
      busy = false
    }
  }
</script>

<div class="wrap">
  <div class="card auth">
    <div class="top">
      <div class="brand">Coin<span>Hub</span></div>
      <LanguageSwitcher />
    </div>
    <p class="muted">{$t('login.tagline')}</p>

    <div class="tabs">
      <button class="tab" class:active={mode === 'login'} on:click={() => (mode = 'login')}>{$t('login.signIn')}</button>
      <button class="tab" class:active={mode === 'signup'} on:click={() => (mode = 'signup')}>{$t('login.createAccount')}</button>
    </div>

    <form on:submit|preventDefault={submit}>
      {#if mode === 'signup'}
        <label for="name">{$t('login.name')}</label>
        <input id="name" bind:value={displayName} placeholder={$t('login.namePlaceholder')} />
      {/if}
      <label for="email">{$t('login.email')}</label>
      <input id="email" type="email" bind:value={email} required placeholder="you@example.com" />
      <label for="password">{$t('login.password')}</label>
      <input id="password" type="password" bind:value={password} required placeholder={$t('login.passwordPlaceholder')} />
      {#if error}<p class="error">{error}</p>{/if}
      <button type="submit" disabled={busy} style="width:100%; margin-top:16px;">
        {busy ? $t('login.wait') : mode === 'login' ? $t('login.signIn') : $t('login.createAccount')}
      </button>
    </form>
  </div>
</div>

<style>
  .wrap { display: grid; place-items: center; min-height: 100vh; padding: 20px; }
  .auth { width: 100%; max-width: 400px; }
  .top { display: flex; justify-content: space-between; align-items: center; }
  .brand { font-size: 1.9em; font-weight: 800; }
  .brand span { color: var(--brand); }
  .tabs { display: flex; gap: 8px; margin: 18px 0; }
  .tab { flex: 1; background: transparent; border: 1px solid var(--border); color: var(--muted); }
  .tab.active { background: var(--surface-2); color: var(--text); border-color: var(--brand); }
</style>
