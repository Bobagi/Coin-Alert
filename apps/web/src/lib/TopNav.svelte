<script lang="ts">
  import { currentUser, binanceStatus, navigate } from './stores'
  import { api } from './api'
  import { t } from './i18n'
  import LanguageDropdown from './LanguageDropdown.svelte'

  let open = false
  let container: HTMLDivElement

  $: displayName = $currentUser?.display_name?.trim() || $currentUser?.email || ''

  async function logout() {
    open = false
    try {
      await api.logout()
    } catch {
      /* ignore */
    }
    binanceStatus.set(null)
    currentUser.set(null)
    navigate('dashboard')
  }

  function goAccount() {
    open = false
    navigate('account')
  }

  function onWindowClick(event: MouseEvent) {
    if (open && container && !container.contains(event.target as Node)) open = false
  }
  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') open = false
  }
</script>

<svelte:window on:click={onWindowClick} on:keydown={onKeydown} />

<header class="topbar">
  <button class="brand" type="button" on:click={() => navigate('dashboard')}>Coin<span>Hub</span></button>
  <div class="spacer"></div>
  {#if $binanceStatus}
    <span class="pill binance" title="Binance">
      <span class="dot" class:on={$binanceStatus.has_active_credential}></span>
      {$t('header.binance')}
      {$binanceStatus.has_active_credential ? $binanceStatus.active_environment : $t('header.notConnected')}
    </span>
  {/if}
  <LanguageDropdown compact />
  <div class="account" bind:this={container}>
    <button
      type="button"
      class="ghost trigger"
      aria-haspopup="menu"
      aria-expanded={open}
      on:click|stopPropagation={() => (open = !open)}
    >
      <span class="avatar">{(displayName[0] || '?').toUpperCase()}</span>
      <span class="who">{displayName}</span>
      <span class="caret" class:up={open}>▾</span>
    </button>
    {#if open}
      <div class="menu" role="menu">
        <button type="button" class="menu-item" role="menuitem" on:click={goAccount}>⚙&nbsp; {$t('nav.account')}</button>
        <div class="menu-divider"></div>
        <button type="button" class="menu-item" role="menuitem" on:click={logout}>⎋&nbsp; {$t('header.signOut')}</button>
      </div>
    {/if}
  </div>
</header>

<style>
  .topbar {
    display: flex; align-items: center; gap: var(--space-3);
    padding: var(--space-3) var(--space-5);
    border-bottom: 1px solid var(--border);
    background: rgba(21, 19, 13, 0.72);
    backdrop-filter: blur(8px);
    position: sticky; top: 0; z-index: 30;
  }
  .brand { background: transparent; border: none; padding: 0; height: auto; color: var(--text); font-weight: 800; font-size: var(--text-lg); }
  .brand span { color: var(--brand); }
  .binance { gap: var(--space-2); }
  .binance .dot { width: 8px; height: 8px; border-radius: var(--radius-pill); background: var(--muted); }
  .binance .dot.on { background: var(--green); box-shadow: 0 0 0 3px rgba(43, 214, 106, 0.18); }
  .account { position: relative; }
  .trigger { gap: var(--space-2); }
  .avatar { display: grid; place-items: center; width: 22px; height: 22px; border-radius: var(--radius-pill); background: var(--brand); color: var(--on-brand); font-size: 0.72rem; font-weight: 800; }
  .who { max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .caret { font-size: 0.7em; transition: transform 0.15s ease; }
  .caret.up { transform: rotate(180deg); }
  .menu { right: 0; top: calc(100% + 6px); }
  @media (max-width: 600px) { .who { display: none; } .binance { display: none; } }
</style>
