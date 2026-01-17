<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
  import {
    RequestAllPermissions,
    RequestPermission,
    OpenPermissionSettings,
    CheckPermissions,
    RecheckPermissions,
    RestartApp,
    GetDetailedPermissionStatus,
    StartPermissionPolling
  } from '../../wailsjs/go/main/App';

  export let show = false;
  export let permissions: Array<{
    type: string;
    status: string;
    name: string;
    description: string;
    required: boolean;
  }> = [];
  export let platform = 'unknown';

  const dispatch = createEventDispatcher();

  let requesting = false;
  let rechecking = false;
  let showRestartHint = false;
  let permissionRequestedAt: number | null = null;
  let keyInterceptorRunning = false;
  let needsRestart = false;

  const platformNames: Record<string, string> = {
    darwin: 'macOS',
    windows: 'Windows',
    linux: 'Linux',
    unknown: 'Unknown'
  };

  const permissionIcons: Record<string, string> = {
    screen_capture: '📷',
    accessibility: '⌨️',
    microphone: '🎤'
  };

  const statusColors: Record<string, string> = {
    granted: 'text-emerald-400',
    denied: 'text-red-400',
    not_asked: 'text-amber-400',
    restricted: 'text-gray-400',
    unknown: 'text-gray-400'
  };

  const statusLabels: Record<string, string> = {
    granted: 'Разрешено',
    denied: 'Запрещено',
    not_asked: 'Не запрошено',
    restricted: 'Ограничено',
    unknown: 'Неизвестно'
  };

  // Подписываемся на события изменения разрешений
  onMount(() => {
    EventsOn('permissionsChanged', (data: {permissions: any[], allGranted: boolean}) => {
      permissions = data.permissions;
      if (data.allGranted) {
        checkDetailedStatus();
      }
    });

    // Запускаем мониторинг разрешений при открытии модального окна
    if (show) {
      StartPermissionPolling(2000);
    }
  });

  onDestroy(() => {
    EventsOff('permissionsChanged');
  });

  // Проверяем детальный статус
  async function checkDetailedStatus() {
    try {
      const status = await GetDetailedPermissionStatus();
      keyInterceptorRunning = status.keyInterceptorRunning || false;
      needsRestart = status.needsRestart || false;

      // Не закрываем окно автоматически - пользователь сам закроет через кнопки
      if (status.allGranted && !needsRestart) {
        dispatch('granted');
      }
    } catch (e) {
      console.error('Failed to get detailed status:', e);
    }
  }

  async function requestAll() {
    requesting = true;
    permissionRequestedAt = Date.now();
    showRestartHint = false;

    try {
      await RequestAllPermissions();

      // Запускаем мониторинг после запроса разрешений
      await StartPermissionPolling(2000);

      // На macOS показываем подсказку о перезапуске
      if (platform === 'darwin') {
        showRestartHint = true;
      }

      // Проверяем результат через небольшую задержку
      await new Promise(resolve => setTimeout(resolve, 1000));
      await recheckPermissions();

    } catch (e) {
      console.error('Failed to request permissions:', e);
    } finally {
      requesting = false;
    }
  }

  async function recheckPermissions() {
    rechecking = true;
    try {
      const result = await RecheckPermissions();
      permissions = result.permissions || [];

      await checkDetailedStatus();

      // Не закрываем окно автоматически - пользователь сам закроет через кнопки
      if (result.allGranted && !needsRestart) {
        dispatch('granted');
      }
    } catch (e) {
      console.error('Failed to recheck permissions:', e);
    } finally {
      rechecking = false;
    }
  }

  async function requestSingle(type: string) {
    try {
      const granted = await RequestPermission(type);

      // Запускаем мониторинг
      await StartPermissionPolling(2000);

      // На macOS показываем подсказку
      if (platform === 'darwin') {
        showRestartHint = true;
      }

      // Перепроверяем
      await recheckPermissions();
    } catch (e) {
      console.error('Failed to request permission:', e);
    }
  }

  async function openSettings(type: string) {
    try {
      await OpenPermissionSettings(type);

      // Показываем подсказку на macOS
      if (platform === 'darwin') {
        showRestartHint = true;
      }

      // Запускаем мониторинг
      await StartPermissionPolling(2000);
    } catch (e) {
      console.error('Failed to open settings:', e);
    }
  }

  async function restartApplication() {
    try {
      await RestartApp();
    } catch (e) {
      console.error('Failed to restart app:', e);
    }
  }

  function close() {
    show = false;
    dispatch('close');
  }

  function skipForNow() {
    show = false;
    dispatch('skip');
  }

  // Вычисляем есть ли недостающие разрешения
  $: missingPermissions = permissions.filter(p => p.required && p.status !== 'granted');
  $: allGranted = missingPermissions.length === 0;
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
    <div class="bg-dark-800 border border-dark-600 rounded-2xl shadow-2xl max-w-md w-full mx-4 overflow-hidden">
      <!-- Header -->
      <div class="p-6 border-b border-dark-700">
        <div class="flex items-center gap-3">
          <div class="w-12 h-12 rounded-full bg-primary-500/20 flex items-center justify-center">
            <span class="text-2xl">🔐</span>
          </div>
          <div>
            <h2 class="text-xl font-bold text-white">Требуются разрешения</h2>
            <p class="text-sm text-gray-400">
              {platformNames[platform]} требует подтверждения
            </p>
          </div>
        </div>
      </div>

      <!-- Permissions List -->
      <div class="p-6 space-y-4">
        <p class="text-gray-400 text-sm mb-4">
          Для корректной работы приложения необходимы следующие разрешения:
        </p>

        {#each permissions as perm}
          <div class="flex items-center justify-between p-4 rounded-xl bg-dark-900/50 border border-dark-700">
            <div class="flex items-center gap-3">
              <span class="text-2xl">{permissionIcons[perm.type] || '🔒'}</span>
              <div>
                <p class="font-medium text-white">{perm.name}</p>
                <p class="text-xs text-gray-500">{perm.description}</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs {statusColors[perm.status]}">
                {statusLabels[perm.status]}
              </span>
              {#if perm.status !== 'granted'}
                <button
                  class="text-xs px-2 py-1 rounded bg-primary-600 hover:bg-primary-500 text-white transition-colors"
                  on:click={() => openSettings(perm.type)}
                >
                  Настройки
                </button>
              {:else}
                <span class="text-emerald-400">✓</span>
              {/if}
            </div>
          </div>
        {/each}

        <!-- macOS Restart Warning -->
        {#if platform === 'darwin' && showRestartHint && !allGranted}
          <div class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30">
            <div class="flex items-start gap-3">
              <span class="text-2xl">⚠️</span>
              <div class="flex-1">
                <p class="font-medium text-amber-400 mb-1">Требуется перезапуск</p>
                <p class="text-xs text-amber-400/80">
                  На macOS после выдачи разрешений необходимо перезапустить приложение,
                  чтобы изменения вступили в силу. Это ограничение системы безопасности macOS.
                </p>
              </div>
            </div>
          </div>
        {/if}

        <!-- Show restart hint if permissions granted but services not working -->
        {#if allGranted && needsRestart}
          <div class="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30">
            <div class="flex items-start gap-3">
              <span class="text-2xl">✅</span>
              <div class="flex-1">
                <p class="font-medium text-emerald-400 mb-1">Разрешения получены!</p>
                <p class="text-xs text-emerald-400/80">
                  Разрешения выданы, но для их применения нужен перезапуск приложения.
                </p>
              </div>
            </div>
          </div>
        {/if}

        {#if platform === 'darwin' && !showRestartHint && !needsRestart}
          <div class="p-3 rounded-lg bg-blue-500/10 border border-blue-500/30 text-blue-400 text-xs">
            <p class="font-medium mb-1">💡 Подсказка для macOS:</p>
            <p>1. Нажмите "Настройки" рядом с нужным разрешением</p>
            <p>2. Включите галочку для Buffer Sharer</p>
            <p>3. Перезапустите приложение</p>
          </div>
        {/if}
      </div>

      <!-- Actions -->
      <div class="p-6 border-t border-dark-700 space-y-3">
        <button
          class="w-full btn btn-primary flex items-center justify-center gap-2"
          on:click={restartApplication}
        >
          <span>🔄</span>
          Перезапустить приложение
        </button>
        <button
          class="w-full btn btn-secondary text-sm"
          on:click={skipForNow}
        >
          Пропустить (некоторые функции не будут работать)
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .btn {
    padding: 0.5rem 1rem;
    border-radius: 0.5rem;
    font-weight: 500;
    transition: all 0.2s;
  }
  .btn-primary {
    background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
    color: white;
  }
  .btn-primary:hover:not(:disabled) {
    background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  }
  .btn-primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .btn-secondary {
    background: rgba(255, 255, 255, 0.1);
    color: #e2e8f0;
    border: 1px solid rgba(255, 255, 255, 0.1);
  }
  .btn-secondary:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.15);
  }
  .btn-secondary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  /* Light theme support */
  :global(html.light) .bg-dark-800 {
    background-color: white;
  }
  :global(html.light) .bg-dark-900\/50 {
    background-color: #f8fafc;
  }
  :global(html.light) .border-dark-700 {
    border-color: #e2e8f0;
  }
  :global(html.light) .border-dark-600 {
    border-color: #cbd5e1;
  }
</style>
