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
    screen_capture: 'screen',
    accessibility: 'keyboard',
    microphone: 'mic'
  };

  const statusColors: Record<string, string> = {
    granted: 'success',
    denied: 'error',
    not_asked: 'warning',
    restricted: 'muted',
    unknown: 'muted'
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
  <div class="modal-overlay">
    <div class="modal animate-scale-in">
      <!-- Header -->
      <div class="modal-header">
        <div class="header-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
          </svg>
        </div>
        <div class="header-text">
          <h2 class="modal-title">Требуются разрешения</h2>
          <p class="modal-subtitle">{platformNames[platform]} требует подтверждения</p>
        </div>
      </div>

      <!-- Content -->
      <div class="modal-content">
        <p class="content-intro">Для корректной работы приложения необходимы следующие разрешения:</p>

        <div class="permissions-list">
          {#each permissions as perm}
            <div class="permission-item">
              <div class="permission-icon">
                {#if permissionIcons[perm.type] === 'screen'}
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
                    <line x1="8" y1="21" x2="16" y2="21"/>
                    <line x1="12" y1="17" x2="12" y2="21"/>
                  </svg>
                {:else if permissionIcons[perm.type] === 'keyboard'}
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="2" y="4" width="20" height="16" rx="2" ry="2"/>
                    <path d="M6 8h.001"/>
                    <path d="M10 8h.001"/>
                    <path d="M14 8h.001"/>
                    <path d="M18 8h.001"/>
                    <path d="M8 12h.001"/>
                    <path d="M12 12h.001"/>
                    <path d="M16 12h.001"/>
                    <path d="M7 16h10"/>
                  </svg>
                {:else}
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                    <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                  </svg>
                {/if}
              </div>
              <div class="permission-info">
                <span class="permission-name">{perm.name}</span>
                <span class="permission-desc">{perm.description}</span>
              </div>
              <div class="permission-actions">
                <span class="permission-status status-{statusColors[perm.status]}">
                  {statusLabels[perm.status]}
                </span>
                {#if perm.status !== 'granted'}
                  <button
                    class="btn btn-secondary btn-sm"
                    on:click={() => openSettings(perm.type)}
                  >
                    Настройки
                  </button>
                {:else}
                  <span class="check-icon">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                      <polyline points="20 6 9 17 4 12"/>
                    </svg>
                  </span>
                {/if}
              </div>
            </div>
          {/each}
        </div>

        <!-- Warnings / Hints -->
        {#if platform === 'darwin' && showRestartHint && !allGranted}
          <div class="alert alert-warning">
            <svg class="alert-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
              <line x1="12" y1="9" x2="12" y2="13"/>
              <line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
            <div class="alert-content">
              <span class="alert-title">Требуется перезапуск</span>
              <span class="alert-text">На macOS после выдачи разрешений необходимо перезапустить приложение, чтобы изменения вступили в силу.</span>
            </div>
          </div>
        {/if}

        {#if allGranted && needsRestart}
          <div class="alert alert-success">
            <svg class="alert-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
            <div class="alert-content">
              <span class="alert-title">Разрешения получены!</span>
              <span class="alert-text">Разрешения выданы, но для их применения нужен перезапуск приложения.</span>
            </div>
          </div>
        {/if}

        {#if platform === 'darwin' && !showRestartHint && !needsRestart}
          <div class="hint-box">
            <span class="hint-title">Подсказка для macOS:</span>
            <ol class="hint-steps">
              <li>Нажмите "Настройки" рядом с нужным разрешением</li>
              <li>Включите галочку для Buffer Sharer</li>
              <li>Перезапустите приложение</li>
            </ol>
          </div>
        {/if}
      </div>

      <!-- Footer -->
      <div class="modal-footer">
        <button class="btn btn-primary action-btn" on:click={restartApplication}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="1 4 1 10 7 10"/>
            <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/>
          </svg>
          <span>Перезапустить приложение</span>
        </button>
        <button class="btn btn-ghost skip-btn" on:click={skipForNow}>
          Пропустить (некоторые функции не будут работать)
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    z-index: var(--z-modal);
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }

  .modal {
    width: 100%;
    max-width: 480px;
    margin: var(--space-4);
    background: var(--bg-elevated);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-2xl);
    box-shadow: var(--shadow-xl);
    overflow: hidden;
  }

  /* Header */
  .modal-header {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-6);
    border-bottom: 1px solid var(--border-secondary);
  }

  .header-icon {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-primary-muted);
    border-radius: var(--radius-xl);
    color: var(--accent-primary);
  }

  .header-text {
    flex: 1;
  }

  .modal-title {
    font-size: var(--text-xl);
    font-weight: 700;
    color: var(--text-primary);
    margin: 0 0 var(--space-1) 0;
  }

  .modal-subtitle {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
    margin: 0;
  }

  /* Content */
  .modal-content {
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .content-intro {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    margin: 0;
  }

  /* Permissions List */
  .permissions-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .permission-item {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
    background: var(--bg-tertiary);
    border: 1px solid var(--border-secondary);
    border-radius: var(--radius-xl);
  }

  .permission-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-hover);
    border-radius: var(--radius-lg);
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .permission-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .permission-name {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-primary);
  }

  .permission-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .permission-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .permission-status {
    font-size: var(--text-xs);
    font-weight: 500;
  }

  .status-success { color: var(--color-success); }
  .status-error { color: var(--color-error); }
  .status-warning { color: var(--color-warning); }
  .status-muted { color: var(--text-muted); }

  .check-icon {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-success);
  }

  .btn-sm {
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-xs);
  }

  /* Alerts */
  .alert {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-4);
    border-radius: var(--radius-xl);
  }

  .alert-warning {
    background: var(--color-warning-muted);
    border: 1px solid rgba(255, 159, 10, 0.3);
  }

  .alert-success {
    background: var(--color-success-muted);
    border: 1px solid rgba(48, 209, 88, 0.3);
  }

  .alert-icon {
    flex-shrink: 0;
    margin-top: 2px;
  }

  .alert-warning .alert-icon { color: var(--color-warning); }
  .alert-success .alert-icon { color: var(--color-success); }

  .alert-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .alert-title {
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .alert-warning .alert-title { color: var(--color-warning); }
  .alert-success .alert-title { color: var(--color-success); }

  .alert-text {
    font-size: var(--text-xs);
    opacity: 0.9;
  }

  .alert-warning .alert-text { color: var(--color-warning); }
  .alert-success .alert-text { color: var(--color-success); }

  /* Hint Box */
  .hint-box {
    padding: var(--space-4);
    background: var(--color-info-muted);
    border: 1px solid rgba(100, 210, 255, 0.3);
    border-radius: var(--radius-lg);
    font-size: var(--text-xs);
    color: var(--color-info);
  }

  .hint-title {
    font-weight: 600;
    display: block;
    margin-bottom: var(--space-2);
  }

  .hint-steps {
    margin: 0;
    padding-left: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  /* Footer */
  .modal-footer {
    padding: var(--space-6);
    border-top: 1px solid var(--border-secondary);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .action-btn {
    width: 100%;
    padding: var(--space-4);
  }

  .skip-btn {
    width: 100%;
    font-size: var(--text-sm);
  }

  /* Animation */
  .animate-scale-in {
    animation: scale-in var(--duration-normal) var(--ease-out);
  }

  @keyframes scale-in {
    from {
      opacity: 0;
      transform: scale(0.95);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  /* Light theme */
  :global(html.light) .modal {
    background: white;
  }

  :global(html.light) .permission-item {
    background: var(--bg-secondary);
  }
</style>
