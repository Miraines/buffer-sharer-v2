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

<div class="h-full flex">
  <!-- History sidebar (only for controller) -->
  {#if role === 'controller' && isConnected && history.length > 0}
    <div class="w-48 bg-dark-900 border-r border-dark-700 flex flex-col">
      <div class="p-3 border-b border-dark-700 flex items-center justify-between">
        <span class="text-sm font-semibold text-gray-400">История ({history.length})</span>
        <button
          class="text-xs text-gray-500 hover:text-red-400 transition-colors"
          on:click={clearHistory}
          title="Очистить историю"
        >
          🗑️
        </button>
      </div>
      <div class="flex-1 overflow-y-auto p-2 space-y-2">
        {#each [...history].reverse() as entry (entry.id)}
          <button
            class="w-full text-left rounded-lg overflow-hidden border-2 transition-all duration-200 {selectedId === entry.id ? 'border-primary-500 ring-2 ring-primary-500/30' : 'border-dark-600 hover:border-dark-500'}"
            on:click={() => selectScreenshot(entry.id)}
          >
            {#if entry.data}
              <img
                src={entry.data}
                alt="Screenshot {entry.id}"
                class="w-full h-20 object-cover bg-dark-800"
              />
            {:else}
              <div class="w-full h-20 bg-dark-800 flex items-center justify-center">
                {#if loadingId === entry.id}
                  <span class="text-xs text-gray-500 animate-pulse">Загрузка...</span>
                {:else}
                  <span class="text-xs text-gray-500">#{entry.id}</span>
                {/if}
              </div>
            {/if}
            <div class="p-1.5 bg-dark-800">
              <div class="text-xs text-gray-400">{formatTime(entry.timestamp)}</div>
              <div class="text-xs text-gray-600">{entry.width}x{entry.height}</div>
            </div>
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Main content -->
  <div class="flex-1 p-8 overflow-auto">
    <div class="max-w-5xl mx-auto">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h2 class="text-2xl font-bold text-white">Скриншоты</h2>
          <p class="text-gray-400">
            {role === 'controller'
              ? 'Просмотр экрана клиента в реальном времени'
              : 'Ваш экран транслируется контроллеру'}
          </p>
        </div>
        {#if screenshotData?.timestamp}
          <span class="text-sm text-gray-500">Обновлено: {formatTime(screenshotData.timestamp)}</span>
        {/if}
      </div>

      {#if !isConnected}
        <div class="card flex flex-col items-center justify-center py-20">
          <span class="text-6xl mb-4 opacity-50">📷</span>
          <p class="text-gray-400 text-lg">Сначала подключитесь к комнате</p>
        </div>
      {:else if role === 'client'}
        <div class="card flex flex-col items-center justify-center py-20">
          <span class="text-6xl mb-4 opacity-50">📡</span>
          <p class="text-gray-400 text-lg">Ваш экран транслируется</p>
          <p class="text-sm text-gray-500 mt-2">Контроллер видит ваш экран в реальном времени</p>
        </div>
      {:else if screenshotData}
        <div class="card p-2">
          <img
            src={screenshotData.data}
            alt="Screenshot"
            class="w-full rounded-lg"
          />
          <div class="flex items-center justify-between mt-2 px-2 text-sm text-gray-500">
            <span>{screenshotData.width} x {screenshotData.height}</span>
            {#if selectedId}
              <span>#{selectedId}</span>
            {/if}
          </div>
        </div>
      {:else}
        <div class="card flex flex-col items-center justify-center py-20">
          <div class="animate-pulse">
            <span class="text-6xl opacity-50">⏳</span>
          </div>
          <p class="text-gray-400 text-lg mt-4">Ожидание скриншота от клиента...</p>
          <p class="text-sm text-gray-500 mt-2">Скриншоты обновляются автоматически</p>
        </div>
      {/if}

      <!-- Controls -->
      {#if role === 'controller'}
        <div class="flex gap-4 mt-6">
          <button
            class="btn flex-1 {saveSuccess ? 'btn-success' : 'btn-secondary'}"
            disabled={!isConnected || !screenshotData || saving}
            on:click={saveScreenshot}
          >
            {#if saving}
              ⏳ Сохранение...
            {:else if saveSuccess}
              ✓ Сохранено!
            {:else}
              💾 Сохранить
            {/if}
          </button>
        </div>
      {/if}
    </div>
  </div>
</div>
