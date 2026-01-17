<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let currentView: string;
  export let isConnected: boolean;
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
    { id: 'connection', icon: '🔗', label: 'Подключение' },
    { id: 'screenshot', icon: '📷', label: 'Скриншоты' },
    { id: 'text', icon: '📝', label: 'Текст' },
    { id: 'settings', icon: '⚙️', label: 'Настройки' },
    { id: 'logs', icon: '📋', label: 'Логи' },
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
    if (seconds < 60) return `${seconds} сек`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)} мин`;
    return `${Math.floor(seconds / 3600)} ч ${Math.floor((seconds % 3600) / 60)} мин`;
  }
</script>

<aside class="w-64 bg-dark-900 border-r border-dark-700 flex flex-col">
  <!-- Logo -->
  <div class="p-6 border-b border-dark-700">
    <h1 class="text-xl font-bold text-white flex items-center gap-2">
      <span class="text-2xl">🖥️</span>
      Buffer Sharer
    </h1>
    <p class="text-xs text-gray-500 mt-1">v2.0 with Rooms</p>
  </div>

  <!-- Status -->
  <div class="px-6 py-4 border-b border-dark-700">
    <div class="flex items-center gap-3">
      <div class="status-dot {isConnected ? 'status-connected' : 'status-disconnected'}"></div>
      <div>
        <p class="text-sm font-medium {isConnected ? 'text-emerald-400' : 'text-red-400'}">
          {isConnected ? 'Подключено' : 'Отключено'}
        </p>
        {#if roomCode}
          <p class="text-xs text-gray-500">Комната: <span class="font-mono text-primary-400">{roomCode}</span></p>
        {/if}
      </div>
    </div>

    <!-- Статистика сессии (только когда подключены) -->
    {#if isConnected}
      <div class="mt-3 pt-3 border-t border-dark-600 grid grid-cols-2 gap-2 text-xs">
        <div class="text-gray-500">
          📷 {statistics.screenshotsSent}/{statistics.screenshotsReceived}
        </div>
        <div class="text-gray-500">
          📝 {statistics.textsSent}/{statistics.textsReceived}
        </div>
        <div class="text-gray-500 col-span-2">
          📊 ↑{formatBytes(statistics.bytesSent)} ↓{formatBytes(statistics.bytesReceived)}
        </div>
        {#if statistics.totalConnectTime > 0}
          <div class="text-gray-500 col-span-2">
            ⏱️ {formatTime(statistics.totalConnectTime)}
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Navigation -->
  <nav class="flex-1 p-4">
    <ul class="space-y-1">
      {#each menuItems as item}
        <li>
          <button
            class="w-full flex items-center gap-3 px-4 py-3 rounded-lg text-left transition-all duration-200
                   {currentView === item.id
                     ? 'bg-primary-600/20 text-primary-400 border-l-2 border-primary-500'
                     : 'text-gray-400 hover:bg-dark-800 hover:text-gray-200'}"
            on:click={() => currentView = item.id}
          >
            <span class="text-lg">{item.icon}</span>
            <span class="font-medium">{item.label}</span>
          </button>
        </li>
      {/each}
    </ul>
  </nav>

  <!-- Footer -->
  <div class="p-4 border-t border-dark-700">
    <div class="flex items-center justify-between mb-3">
      <span class="text-xs text-gray-500">Тема</span>
      <button
        class="flex items-center gap-1 text-xs px-2 py-1 rounded-lg transition-colors
               hover:bg-dark-700 text-gray-400 hover:text-gray-200"
        on:click={toggleTheme}
      >
        {#if currentTheme === 'dark'}
          <span>🌙</span>
        {:else if currentTheme === 'light'}
          <span>☀️</span>
        {:else}
          <span>💻</span>
        {/if}
      </button>
    </div>
    <p class="text-xs text-gray-600 text-center">
      © 2026 Buffer Sharer
    </p>
  </div>
</aside>
