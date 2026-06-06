<script lang="ts">
  import { api } from './api'
  import { currentUser } from './stores'

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
    <div class="brand">Coin<span>Hub</span></div>
    <p class="muted">Crypto trading automation and your B3 portfolio, in one place.</p>

    <div class="tabs">
      <button class="tab" class:active={mode === 'login'} on:click={() => (mode = 'login')}>Sign in</button>
      <button class="tab" class:active={mode === 'signup'} on:click={() => (mode = 'signup')}>Create account</button>
    </div>

    <form on:submit|preventDefault={submit}>
      {#if mode === 'signup'}
        <label for="name">Name</label>
        <input id="name" bind:value={displayName} placeholder="Your name" />
      {/if}
      <label for="email">Email</label>
      <input id="email" type="email" bind:value={email} required placeholder="you@example.com" />
      <label for="password">Password</label>
      <input id="password" type="password" bind:value={password} required placeholder="At least 8 characters" />
      {#if error}<p class="error">{error}</p>{/if}
      <button type="submit" disabled={busy} style="width:100%; margin-top:16px;">
        {busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
      </button>
    </form>
  </div>
</div>

<style>
  .wrap { display: grid; place-items: center; min-height: 100vh; padding: 20px; }
  .auth { width: 100%; max-width: 400px; }
  .brand { font-size: 1.9em; font-weight: 800; }
  .brand span { color: var(--brand); }
  .tabs { display: flex; gap: 8px; margin: 18px 0; }
  .tab { flex: 1; background: transparent; border: 1px solid var(--border); color: var(--muted); }
  .tab.active { background: var(--surface-2); color: var(--text); border-color: var(--brand); }
</style>
