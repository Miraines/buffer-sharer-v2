<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { ToggleInvisibility } from '../../wailsjs/go/app/App';

  export let currentView: string;
  export let isConnected: boolean;
  export let isInvisible: boolean = false;
  export let roomCode: string;
  export let currentTheme: string = 'dark';
  export let statistics: {
    screenshotsSent: number;
    screenshotsReceived: number;
    textsSent: number;
    textsReceived: number;
    bytesSent: number;
    bytesReceived: number;
    totalConnectTime: number;
  } = {
    screenshotsSent: 0,
    screenshotsReceived: 0,
    textsSent: 0,
    textsReceived: 0,
    bytesSent: 0,
    bytesReceived: 0,
    totalConnectTime: 0
  };

  const dispatch = createEventDispatcher();

  const menuItems = [
    { id: 'connection', icon: 'link', label: 'Подключение' },
    { id: 'screenshot', icon: 'image', label: 'Скриншоты' },
    { id: 'text', icon: 'type', label: 'Текст' },
    { id: 'settings', icon: 'settings', label: 'Настройки' },
    { id: 'logs', icon: 'terminal', label: 'Логи' },
  ];

  const themes = ['dark', 'light', 'system'];

  function toggleTheme() {
    const currentIndex = themes.indexOf(currentTheme);
    const nextIndex = (currentIndex + 1) % themes.length;
    dispatch('themeChange', themes[nextIndex]);
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatTime(seconds: number): string {
    if (seconds < 60) return `${seconds}с`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}м`;
    return `${Math.floor(seconds / 3600)}ч ${Math.floor((seconds % 3600) / 60)}м`;
  }

  function getThemeIcon(theme: string): string {
    switch (theme) {
      case 'light': return 'sun';
      case 'dark': return 'moon';
      default: return 'monitor';
    }
  }

  async function toggleInvisibility() {
    try {
      await ToggleInvisibility();
    } catch (e) {
      console.error('Failed to toggle invisibility:', e);
    }
  }
</script>

<aside class="sidebar">
  <!-- Logo -->
  <div class="sidebar-header">
    <div class="logo">
      <div class="logo-icon">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
          <line x1="8" y1="21" x2="16" y2="21"/>
          <line x1="12" y1="17" x2="12" y2="21"/>
        </svg>
      </div>
      <div class="logo-text">
        <span class="logo-title">Buffer Sharer</span>
        <span class="logo-version">v2.0</span>
      </div>
    </div>
  </div>

  <!-- Connection Status -->
  <div class="status-section">
    <div class="status-indicator">
      <div class="status-dot {isConnected ? 'status-connected' : 'status-disconnected'}"></div>
      <div class="status-info">
        <span class="status-label {isConnected ? 'connected' : 'disconnected'}">
          {isConnected ? 'Подключено' : 'Отключено'}
        </span>
        {#if roomCode}
          <span class="room-code copyable">{roomCode}</span>
        {/if}
      </div>
    </div>

    <!-- Invisibility Status -->
    <button
      class="invisibility-toggle"
      on:click={toggleInvisibility}
      title={isInvisible ? 'Нажмите чтобы выключить' : 'Нажмите чтобы включить'}
    >
      <div class="status-dot {isInvisible ? 'status-connected' : 'status-disconnected'}"></div>
      <span class="invisibility-label {isInvisible ? 'enabled' : 'disabled'}">
        Невидимость
      </span>
    </button>

    <!-- Session Stats -->
    {#if isConnected}
      <div class="stats-grid">
        <div class="stat-item">
          <svg class="stat-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
            <circle cx="8.5" cy="8.5" r="1.5"/>
            <polyline points="21 15 16 10 5 21"/>
          </svg>
          <span>{statistics.screenshotsSent}/{statistics.screenshotsReceived}</span>
        </div>
        <div class="stat-item">
          <svg class="stat-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="4 7 4 4 20 4 20 7"/>
            <line x1="9" y1="20" x2="15" y2="20"/>
            <line x1="12" y1="4" x2="12" y2="20"/>
          </svg>
          <span>{statistics.textsSent}/{statistics.textsReceived}</span>
        </div>
        <div class="stat-item stat-full">
          <svg class="stat-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/>
          </svg>
          <span>↑{formatBytes(statistics.bytesSent)} ↓{formatBytes(statistics.bytesReceived)}</span>
        </div>
        {#if statistics.totalConnectTime > 0}
          <div class="stat-item stat-full">
            <svg class="stat-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/>
              <polyline points="12 6 12 12 16 14"/>
            </svg>
            <span>{formatTime(statistics.totalConnectTime)}</span>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Navigation -->
  <nav class="nav-section">
    {#each menuItems as item}
      <button
        class="nav-item {currentView === item.id ? 'active' : ''}"
        on:click={() => currentView = item.id}
      >
        <div class="nav-icon">
          {#if item.icon === 'link'}
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
              <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
            </svg>
          {:else if item.icon === 'image'}
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
              <circle cx="8.5" cy="8.5" r="1.5"/>
              <polyline points="21 15 16 10 5 21"/>
            </svg>
          {:else if item.icon === 'type'}
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 7 4 4 20 4 20 7"/>
              <line x1="9" y1="20" x2="15" y2="20"/>
              <line x1="12" y1="4" x2="12" y2="20"/>
            </svg>
          {:else if item.icon === 'settings'}
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="3"/>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
            </svg>
          {:else if item.icon === 'terminal'}
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 17 10 11 4 5"/>
              <line x1="12" y1="19" x2="20" y2="19"/>
            </svg>
          {/if}
        </div>
        <span class="nav-label">{item.label}</span>
        {#if currentView === item.id}
          <div class="nav-indicator"></div>
        {/if}
      </button>
    {/each}
  </nav>

  <!-- Footer -->
  <div class="sidebar-footer">
    <button class="theme-toggle" on:click={toggleTheme} title="Переключить тему">
      {#if getThemeIcon(currentTheme) === 'moon'}
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
        </svg>
      {:else if getThemeIcon(currentTheme) === 'sun'}
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="5"/>
          <line x1="12" y1="1" x2="12" y2="3"/>
          <line x1="12" y1="21" x2="12" y2="23"/>
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
          <line x1="1" y1="12" x2="3" y2="12"/>
          <line x1="21" y1="12" x2="23" y2="12"/>
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
        </svg>
      {:else}
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
          <line x1="8" y1="21" x2="16" y2="21"/>
          <line x1="12" y1="17" x2="12" y2="21"/>
        </svg>
      {/if}
    </button>
    <span class="copyright">© 2026</span>
  </div>
</aside>

<style>
  .sidebar {
    width: 240px;
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur-heavy)) saturate(180%);
    -webkit-backdrop-filter: blur(var(--glass-blur-heavy)) saturate(180%);
    border-right: 1px solid var(--border-primary);
  }

  /* Header */
  .sidebar-header {
    padding: var(--space-5) var(--space-5);
    border-bottom: 1px solid var(--border-secondary);
  }

  .logo {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .logo-icon {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-primary-muted);
    border-radius: var(--radius-md);
    color: var(--accent-primary);
  }

  .logo-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .logo-title {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-primary);
    letter-spacing: var(--tracking-tight);
  }

  .logo-version {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  /* Status Section */
  .status-section {
    padding: var(--space-4) var(--space-5);
    border-bottom: 1px solid var(--border-secondary);
  }

  .status-indicator {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .status-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .status-label {
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .status-label.connected {
    color: var(--color-success);
  }

  .status-label.disconnected {
    color: var(--color-error);
  }

  .room-code {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--accent-primary);
    letter-spacing: var(--tracking-wide);
  }

  /* Invisibility Toggle */
  .invisibility-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-top: var(--space-3);
    padding: 0;
    background: transparent;
    border: none;
    cursor: pointer;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .invisibility-toggle:hover {
    opacity: 0.8;
  }

  .invisibility-label {
    font-size: var(--text-sm);
    font-weight: 500;
    transition: color var(--duration-fast) var(--ease-out);
  }

  .invisibility-label.enabled {
    color: var(--color-success);
  }

  .invisibility-label.disabled {
    color: var(--color-error);
  }

  /* Stats Grid */
  .stats-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-2);
    margin-top: var(--space-3);
    padding-top: var(--space-3);
    border-top: 1px solid var(--border-secondary);
  }

  .stat-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-tertiary);
  }

  .stat-item.stat-full {
    grid-column: span 2;
  }

  .stat-icon {
    opacity: 0.5;
    flex-shrink: 0;
  }

  /* Navigation */
  .nav-section {
    flex: 1;
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    overflow-y: auto;
  }

  .nav-item {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background: transparent;
    border: none;
    border-radius: var(--radius-lg);
    color: var(--text-secondary);
    font-size: var(--text-sm);
    font-weight: 500;
    text-align: left;
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
    width: 100%;
  }

  .nav-item:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .nav-item.active {
    background: var(--accent-primary-muted);
    color: var(--accent-primary);
  }

  .nav-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    flex-shrink: 0;
  }

  .nav-label {
    flex: 1;
  }

  .nav-indicator {
    position: absolute;
    left: 0;
    top: 50%;
    transform: translateY(-50%);
    width: 3px;
    height: 20px;
    background: var(--accent-primary);
    border-radius: 0 var(--radius-full) var(--radius-full) 0;
  }

  /* Footer */
  .sidebar-footer {
    padding: var(--space-4) var(--space-5);
    border-top: 1px solid var(--border-secondary);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .theme-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    background: var(--bg-hover);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .theme-toggle:hover {
    background: var(--bg-active);
    color: var(--text-primary);
    border-color: var(--border-hover);
  }

  .copyright {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
</style>
