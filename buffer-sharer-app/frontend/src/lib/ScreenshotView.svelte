<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { GetScreenshotHistory, GetScreenshotByID, ClearScreenshotHistory, SaveScreenshotToFile, GetScreenshotSaveDir } from '../../wailsjs/go/main/App';

  export let isConnected: boolean;
  export let screenshotData: {id?: number, data: string, width: number, height: number, timestamp?: string} | null = null;
  export let role: string;
  // История передается из родительского компонента для сохранения при переключении вкладок
  export let history: HistoryEntry[] = [];
  // Maximum number of screenshots to keep in history (from settings)
  export let historyLimit: number = 50;

  const dispatch = createEventDispatcher();

  let saving = false;
  let saveSuccess = false;

  // Screenshot history type (exported for use in App.svelte)
  export type HistoryEntry = {
    id: number;
    timestamp: string;
    width: number;
    height: number;
    size: number;
    data?: string; // base64 data, loaded on demand
  };

  let selectedId: number | null = null;
  let loadingId: number | null = null;

  // When new screenshot arrives, add to local history
  $: if (screenshotData && screenshotData.id) {
    addToLocalHistory(screenshotData);
    selectedId = screenshotData.id;
  }

  function addToLocalHistory(data: {id?: number, data: string, width: number, height: number, timestamp?: string}) {
    if (!data.id) return;

    // Check if already exists
    const existing = history.find(h => h.id === data.id);
    if (existing) {
      existing.data = data.data;
      history = [...history]; // Trigger reactivity
      dispatch('historyUpdate', history);
      return;
    }

    const entry: HistoryEntry = {
      id: data.id,
      timestamp: data.timestamp || new Date().toISOString(),
      width: data.width,
      height: data.height,
      size: Math.round(data.data.length * 0.75), // approximate
      data: data.data
    };

    history = [...history, entry];

    // Limit local history based on settings
    const maxItems = historyLimit > 0 ? historyLimit : 50;
    if (history.length > maxItems) {
      history = history.slice(-maxItems);
    }

    // Notify parent about history update
    dispatch('historyUpdate', history);
  }

  async function selectScreenshot(id: number) {
    if (loadingId === id) return;
    selectedId = id;

    // Check if we have data cached
    const entry = history.find(h => h.id === id);
    if (entry && entry.data) {
      screenshotData = {
        id: entry.id,
        data: entry.data,
        width: entry.width,
        height: entry.height,
        timestamp: entry.timestamp
      };
      return;
    }

    // Load from backend
    loadingId = id;
    try {
      const result = await GetScreenshotByID(id);
      if (result) {
        screenshotData = {
          id: result.id,
          data: result.data,
          width: result.width,
          height: result.height
        };
        // Cache it
        if (entry) {
          entry.data = result.data;
        }
      }
    } catch (e) {
      console.error('Failed to load screenshot:', e);
    } finally {
      loadingId = null;
    }
  }

  async function clearHistory() {
    try {
      await ClearScreenshotHistory();
      history = [];
      selectedId = null;
      dispatch('historyUpdate', history);
    } catch (e) {
      console.error('Failed to clear history:', e);
    }
  }

  async function saveScreenshot() {
    if (!screenshotData || saving) return;

    saving = true;
    saveSuccess = false;

    try {
      const filename = `screenshot-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.jpg`;
      const savedPath = await SaveScreenshotToFile(screenshotData.data, filename);
      saveSuccess = true;
      dispatch('log', { level: 'info', message: `Скриншот сохранен: ${savedPath}` });

      // Показываем успех на 2 секунды
      setTimeout(() => {
        saveSuccess = false;
      }, 2000);
    } catch (e: any) {
      console.error('Failed to save screenshot:', e);
      dispatch('log', { level: 'error', message: `Ошибка сохранения: ${e.message || e}` });
    } finally {
      saving = false;
    }
  }

  // Fallback: сохранение через браузер если backend недоступен
  function saveScreenshotBrowser() {
    if (!screenshotData) return;

    const link = document.createElement('a');
    link.href = screenshotData.data;
    link.download = `screenshot-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.jpg`;
    link.click();
  }

  function formatTime(timestamp: string): string {
    try {
      return new Date(timestamp).toLocaleTimeString();
    } catch {
      return '';
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  }
</script>

<div class="screenshot-container">
  <!-- History sidebar (only for controller) -->
  {#if role === 'controller' && isConnected && history.length > 0}
    <aside class="history-sidebar">
      <div class="history-header">
        <span class="history-title">История ({history.length})</span>
        <button class="btn-icon-sm" on:click={clearHistory} title="Очистить историю">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="3 6 5 6 21 6"/>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
          </svg>
        </button>
      </div>
      <div class="history-list">
        {#each [...history].reverse() as entry (entry.id)}
          <button
            class="history-item {selectedId === entry.id ? 'active' : ''}"
            on:click={() => selectScreenshot(entry.id)}
          >
            {#if entry.data}
              <img
                src={entry.data}
                alt="Screenshot {entry.id}"
                class="history-thumb"
              />
            {:else}
              <div class="history-thumb placeholder">
                {#if loadingId === entry.id}
                  <div class="spinner-sm"></div>
                {:else}
                  <span>#{entry.id}</span>
                {/if}
              </div>
            {/if}
            <div class="history-meta">
              <span class="history-time">{formatTime(entry.timestamp)}</span>
              <span class="history-size">{entry.width}x{entry.height}</span>
            </div>
          </button>
        {/each}
      </div>
    </aside>
  {/if}

  <!-- Main content -->
  <main class="screenshot-main">
    <div class="screenshot-content">
      <div class="panel-header">
        <div>
          <h1 class="panel-title">Скриншоты</h1>
          <p class="panel-subtitle">
            {role === 'controller'
              ? 'Просмотр экрана клиента в реальном времени'
              : 'Ваш экран транслируется контроллеру'}
          </p>
        </div>
        {#if screenshotData?.timestamp}
          <span class="timestamp">Обновлено: {formatTime(screenshotData.timestamp)}</span>
        {/if}
      </div>

      {#if !isConnected}
        <div class="empty-state card">
          <div class="empty-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
              <circle cx="8.5" cy="8.5" r="1.5"/>
              <polyline points="21 15 16 10 5 21"/>
            </svg>
          </div>
          <p class="empty-title">Сначала подключитесь к комнате</p>
        </div>
      {:else if role === 'client'}
        <div class="empty-state card">
          <div class="empty-icon streaming">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
              <circle cx="12" cy="12" r="3"/>
            </svg>
          </div>
          <p class="empty-title">Ваш экран транслируется</p>
          <p class="empty-subtitle">Контроллер видит ваш экран в реальном времени</p>
        </div>
      {:else if screenshotData}
        <div class="screenshot-view card">
          <img
            src={screenshotData.data}
            alt="Screenshot"
            class="screenshot-image"
          />
          <div class="screenshot-meta">
            <span>{screenshotData.width} x {screenshotData.height}</span>
            {#if selectedId}
              <span>#{selectedId}</span>
            {/if}
          </div>
        </div>
      {:else}
        <div class="empty-state card">
          <div class="empty-icon loading">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <circle cx="12" cy="12" r="10"/>
              <polyline points="12 6 12 12 16 14"/>
            </svg>
          </div>
          <p class="empty-title">Ожидание скриншота от клиента...</p>
          <p class="empty-subtitle">Скриншоты обновляются автоматически</p>
        </div>
      {/if}

      <!-- Controls -->
      {#if role === 'controller'}
        <div class="controls">
          <button
            class="btn {saveSuccess ? 'btn-success' : 'btn-secondary'} save-btn"
            disabled={!isConnected || !screenshotData || saving}
            on:click={saveScreenshot}
          >
            {#if saving}
              <div class="spinner-sm"></div>
              <span>Сохранение...</span>
            {:else if saveSuccess}
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              <span>Сохранено!</span>
            {:else}
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
                <polyline points="17 21 17 13 7 13 7 21"/>
                <polyline points="7 3 7 8 15 8"/>
              </svg>
              <span>Сохранить</span>
            {/if}
          </button>
        </div>
      {/if}
    </div>
  </main>
</div>

<style>
  .screenshot-container {
    height: 100%;
    display: flex;
  }

  /* History Sidebar */
  .history-sidebar {
    width: 180px;
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur));
    -webkit-backdrop-filter: blur(var(--glass-blur));
    border-right: 1px solid var(--border-primary);
    display: flex;
    flex-direction: column;
  }

  .history-header {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-secondary);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .history-title {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-secondary);
  }

  .btn-icon-sm {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    color: var(--text-tertiary);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .btn-icon-sm:hover {
    background: var(--bg-hover);
    color: var(--color-error);
  }

  .history-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .history-item {
    width: 100%;
    background: var(--bg-tertiary);
    border: 2px solid var(--border-primary);
    border-radius: var(--radius-lg);
    overflow: hidden;
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
    text-align: left;
    padding: 0;
  }

  .history-item:hover {
    border-color: var(--border-hover);
  }

  .history-item.active {
    border-color: var(--accent-primary);
    box-shadow: 0 0 0 2px var(--accent-primary-muted);
  }

  .history-thumb {
    width: 100%;
    height: 80px;
    object-fit: cover;
    background: var(--bg-tertiary);
    display: block;
  }

  .history-thumb.placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .history-meta {
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .history-time {
    font-size: var(--text-xs);
    color: var(--text-secondary);
  }

  .history-size {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  /* Main Content */
  .screenshot-main {
    flex: 1;
    padding: var(--space-8);
    overflow: auto;
  }

  .screenshot-content {
    max-width: 1000px;
    margin: 0 auto;
  }

  .panel-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: var(--space-6);
  }

  .panel-title {
    font-size: var(--text-2xl);
    font-weight: 700;
    color: var(--text-primary);
    margin: 0 0 var(--space-2) 0;
    letter-spacing: var(--tracking-tight);
  }

  .panel-subtitle {
    font-size: var(--text-base);
    color: var(--text-secondary);
    margin: 0;
  }

  .timestamp {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  /* Empty State */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: var(--space-16);
    text-align: center;
  }

  .empty-icon {
    width: 80px;
    height: 80px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    border-radius: var(--radius-2xl);
    color: var(--text-muted);
    margin-bottom: var(--space-4);
  }

  .empty-icon.streaming {
    background: var(--color-success-muted);
    color: var(--color-success);
    animation: pulse-glow 2s var(--ease-in-out) infinite;
  }

  .empty-icon.loading {
    animation: pulse-glow 2s var(--ease-in-out) infinite;
  }

  .empty-title {
    font-size: var(--text-lg);
    font-weight: 500;
    color: var(--text-secondary);
    margin: 0 0 var(--space-2) 0;
  }

  .empty-subtitle {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
    margin: 0;
  }

  /* Screenshot View */
  .screenshot-view {
    padding: var(--space-2);
  }

  .screenshot-image {
    width: 100%;
    border-radius: var(--radius-lg);
    display: block;
  }

  .screenshot-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-2) var(--space-1);
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  /* Controls */
  .controls {
    margin-top: var(--space-6);
  }

  .save-btn {
    width: 100%;
    padding: var(--space-4);
  }

  /* Spinner */
  .spinner-sm {
    width: 16px;
    height: 16px;
    border: 2px solid currentColor;
    border-top-color: transparent;
    border-radius: var(--radius-full);
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @keyframes pulse-glow {
    0%, 100% {
      opacity: 1;
    }
    50% {
      opacity: 0.6;
    }
  }
</style>
