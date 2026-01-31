<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
  import { GetSettings, GetConnectionStatus, GetStatistics, CheckPermissions, GetInvisibilityStatus } from '../wailsjs/go/app/App';
  import Sidebar from './lib/Sidebar.svelte';
  import ConnectionPanel from './lib/ConnectionPanel.svelte';
  import ScreenshotView from './lib/ScreenshotView.svelte';
  import TextPanel from './lib/TextPanel.svelte';
  import SettingsPanel from './lib/SettingsPanel.svelte';
  import LogsPanel from './lib/LogsPanel.svelte';
  import Toast from './lib/Toast.svelte';
  import PermissionsModal from './lib/PermissionsModal.svelte';

  let currentView = 'connection';
  let isConnected = false;
  let roomCode = '';
  let role = 'controller';
  let isInvisible = false;

  // Toast уведомления
  let toasts: Array<{id: number, type: 'success' | 'error' | 'info' | 'warning', message: string}> = [];
  let toastComponent: Toast;

  // Permissions modal
  let showPermissionsModal = false;
  let permissionsList: Array<{type: string, status: string, name: string, description: string, required: boolean}> = [];
  let platformName = 'unknown';

  // Статистика сессии
  let statistics = {
    screenshotsSent: 0,
    screenshotsReceived: 0,
    textsSent: 0,
    textsReceived: 0,
    bytesSent: 0,
    bytesReceived: 0,
    connectedAt: '',
    totalConnectTime: 0
  };

  // Settings state
  let settings = {
    middlewareHost: 'localhost',
    middlewarePort: 8080,
    screenshotInterval: 4000,
    screenshotQuality: 80,
    clipboardSync: true,
    hotkeyToggle: 'Ctrl+Shift+J',
    hotkeyScreenshot: 'Ctrl+Shift+S',
    hotkeyPaste: 'Ctrl+Shift+V',
    hotkeyInvisibility: 'Ctrl+Shift+I',
    autoConnect: false,
    lastRole: 'controller',
    lastRoomCode: '',
    soundEnabled: true,
    theme: 'dark',
    screenshotSaveDir: '',
    screenshotHistoryLimit: 50
  };

  // Функция применения темы
  function applyTheme(theme: string) {
    if (theme === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.classList.toggle('light', !prefersDark);
    } else {
      document.documentElement.classList.toggle('light', theme === 'light');
    }
  }

  // Реактивно применяем тему при изменении
  $: applyTheme(settings.theme);

  // Logs
  let logs: Array<{time: string, level: string, message: string}> = [];

  // Screenshot data
  let screenshotData: {id?: number, data: string, width: number, height: number, timestamp?: string} | null = null;

  // Screenshot history (хранится здесь чтобы не терялась при переключении вкладок)
  type ScreenshotHistoryEntry = {
    id: number;
    timestamp: string;
    width: number;
    height: number;
    size: number;
    data?: string;
  };
  let screenshotHistory: ScreenshotHistoryEntry[] = [];

  // Received text
  let receivedText = '';

  function addLog(level: string, message: string) {
    const time = new Date().toLocaleTimeString();
    logs = [...logs, { time, level, message }];
    if (logs.length > 100) logs = logs.slice(-100);
  }

  // Функция для показа toast
  function showToast(type: 'success' | 'error' | 'info' | 'warning', message: string) {
    const id = Date.now();
    toasts = [...toasts, { id, type, message }];
    setTimeout(() => {
      toasts = toasts.filter(t => t.id !== id);
    }, 3000);
  }

  // Обновление статистики
  async function updateStatistics() {
    if (!isConnected) return;
    try {
      const stats = await GetStatistics();
      statistics = {
        screenshotsSent: stats.screenshotsSent,
        screenshotsReceived: stats.screenshotsReceived,
        textsSent: stats.textsSent,
        textsReceived: stats.textsReceived,
        bytesSent: stats.bytesSent,
        bytesReceived: stats.bytesReceived,
        connectedAt: stats.connectedAt,
        totalConnectTime: stats.totalConnectTime
      };
    } catch (e) {
      console.error('Failed to get statistics:', e);
    }
  }

  onMount(async () => {
    // Load settings from Go backend
    try {
      const s = await GetSettings();
      settings = {
        middlewareHost: s.middlewareHost,
        middlewarePort: s.middlewarePort,
        screenshotInterval: s.screenshotInterval,
        screenshotQuality: s.screenshotQuality,
        clipboardSync: s.clipboardSync,
        hotkeyToggle: s.hotkeyToggle,
        hotkeyScreenshot: s.hotkeyScreenshot,
        hotkeyPaste: s.hotkeyPaste,
        hotkeyInvisibility: s.hotkeyInvisibility || 'Ctrl+Shift+I',
        autoConnect: s.autoConnect || false,
        lastRole: s.lastRole || 'controller',
        lastRoomCode: s.lastRoomCode || '',
        soundEnabled: s.soundEnabled !== false,
        theme: s.theme || 'dark',
        screenshotSaveDir: s.screenshotSaveDir || '',
        screenshotHistoryLimit: s.screenshotHistoryLimit || 50
      };
      // Устанавливаем последнюю использованную роль
      role = settings.lastRole;
      // Применяем тему
      applyTheme(settings.theme);
    } catch (e) {
      console.error('Failed to load settings:', e);
    }

    // Get initial connection status
    try {
      const status = await GetConnectionStatus();
      isConnected = status.connected;
      roomCode = status.roomCode;
      role = status.role || settings.lastRole || 'controller';
    } catch (e) {
      console.error('Failed to get connection status:', e);
    }

    // Get initial invisibility status
    try {
      const invStatus = await GetInvisibilityStatus();
      isInvisible = invStatus.enabled;
    } catch (e) {
      console.error('Failed to get invisibility status:', e);
    }

    // Listen for log events from Go
    EventsOn('log', (data: {level: string, message: string}) => {
      addLog(data.level, data.message);
    });

    // Listen for connection events
    EventsOn('connected', () => {
      isConnected = true;
      showToast('success', 'Подключено к серверу');
    });

    EventsOn('disconnected', () => {
      isConnected = false;
      roomCode = '';
      showToast('info', 'Отключено от сервера');
    });

    EventsOn('roomCreated', (code: string) => {
      roomCode = code;
      showToast('success', `Комната создана: ${code}`);
    });

    EventsOn('roomJoined', (code: string) => {
      roomCode = code;
      showToast('success', `Подключено к комнате: ${code}`);
    });

    EventsOn('authError', (error: string) => {
      addLog('error', `Ошибка аутентификации: ${error}`);
      showToast('error', `Ошибка: ${error}`);
    });

    // Listen for screenshot events
    EventsOn('screenshot', (data: {data: string, width: number, height: number}) => {
      screenshotData = data;
      updateStatistics();
    });

    // Listen for text events
    EventsOn('textReceived', (text: string) => {
      receivedText = text;
      addLog('info', `Получен текст: ${text.substring(0, 50)}${text.length > 50 ? '...' : ''}`);
      showToast('info', `Получен текст (${text.length} симв.)`);
      updateStatistics();
    });

    // Listen for clipboard events
    EventsOn('clipboardReceived', (text: string) => {
      addLog('info', `Буфер обмена: ${text.substring(0, 30)}${text.length > 30 ? '...' : ''}`);
    });

    // Listen for permissions events
    EventsOn('permissionsRequired', (data: {permissions: any[], missing: any[], platform: string}) => {
      permissionsList = data.permissions;
      platformName = data.platform;
      if (data.missing.length > 0) {
        showPermissionsModal = true;
      }
    });

    // Listen for permissions changes (from polling)
    EventsOn('permissionsChanged', (data: {permissions: any[], allGranted: boolean}) => {
      permissionsList = data.permissions;
      if (data.allGranted) {
        addLog('info', 'Все разрешения получены');
        showToast('success', 'Все разрешения получены!');
      }
    });

    // Listen for invisibility changes
    EventsOn('invisibilityChanged', (enabled: boolean) => {
      isInvisible = enabled;
      if (enabled) {
        addLog('info', 'Режим невидимости ВКЛЮЧЁН');
        showToast('success', 'Режим невидимости включён');
      } else {
        addLog('info', 'Режим невидимости ВЫКЛЮЧЕН');
        showToast('info', 'Режим невидимости выключен');
      }
    });

    // Интервал обновления статистики
    const statsInterval = setInterval(updateStatistics, 5000);

    addLog('info', 'Buffer Sharer запущен');

    // Cleanup interval on destroy
    return () => {
      clearInterval(statsInterval);
    };
  });

  onDestroy(() => {
    EventsOff('log');
    EventsOff('connected');
    EventsOff('disconnected');
    EventsOff('roomCreated');
    EventsOff('roomJoined');
    EventsOff('authError');
    EventsOff('screenshot');
    EventsOff('textReceived');
    EventsOff('clipboardReceived');
    EventsOff('permissionsRequired');
    EventsOff('permissionsChanged');
    EventsOff('invisibilityChanged');
  });
</script>

<div class="app-container">
  <!-- Toast уведомления -->
  <Toast bind:toasts />

  <!-- Permissions Modal -->
  <PermissionsModal
    bind:show={showPermissionsModal}
    permissions={permissionsList}
    platform={platformName}
    on:granted={() => showToast('success', 'Все разрешения получены!')}
    on:skip={() => showToast('warning', 'Некоторые функции могут не работать')}
  />

  <!-- Sidebar -->
  <Sidebar
    bind:currentView
    {isConnected}
    {isInvisible}
    {roomCode}
    {statistics}
    currentTheme={settings.theme}
    on:themeChange={(e) => settings.theme = e.detail}
  />

  <!-- Main Content -->
  <main class="main-content">
    {#if currentView === 'connection'}
      <ConnectionPanel
        bind:isConnected
        bind:roomCode
        bind:role
        {settings}
        {isInvisible}
        on:log={(e) => addLog(e.detail.level, e.detail.message)}
        on:toast={(e) => showToast(e.detail.type, e.detail.message)}
      />
    {:else if currentView === 'screenshot'}
      <ScreenshotView
        {isConnected}
        {screenshotData}
        {role}
        historyLimit={settings.screenshotHistoryLimit}
        bind:history={screenshotHistory}
        on:historyUpdate={(e) => screenshotHistory = e.detail}
        on:log={(e) => addLog(e.detail.level, e.detail.message)}
      />
    {:else if currentView === 'text'}
      <TextPanel {isConnected} {role} {receivedText} />
    {:else if currentView === 'settings'}
      <SettingsPanel bind:settings on:log={(e) => addLog(e.detail.level, e.detail.message)} />
    {:else if currentView === 'logs'}
      <LogsPanel bind:logs />
    {/if}
  </main>
</div>

<style>
  .app-container {
    display: flex;
    height: 100vh;
    background-color: var(--bg-primary);
  }

  .main-content {
    flex: 1;
    overflow: hidden;
    background-color: var(--bg-primary);
  }
</style>
