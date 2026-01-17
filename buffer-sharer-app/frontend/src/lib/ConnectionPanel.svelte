<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { Connect, Disconnect, ToggleInvisibility } from '../../wailsjs/go/main/App';

  export let isConnected: boolean;
  export let roomCode: string;
  export let role: string;
  export let settings: any;
  export let isInvisible: boolean = false;

  const dispatch = createEventDispatcher();
  const CONNECTION_TIMEOUT = 15000; // 15 секунд

  let inputRoomCode = '';
  let connecting = false;
  let error = '';
  let cancelled = false;
  let timeoutId: ReturnType<typeof setTimeout> | null = null;
  let countdown = 0;
  let countdownInterval: ReturnType<typeof setInterval> | null = null;

  // Очистка при уничтожении компонента (исправляем баг с утечкой)
  onDestroy(() => {
    clearTimers();
  });

  function log(level: string, message: string) {
    dispatch('log', { level, message });
  }

  function toast(type: 'success' | 'error' | 'info' | 'warning', message: string) {
    dispatch('toast', { type, message });
  }

  function clearTimers() {
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    if (countdownInterval) {
      clearInterval(countdownInterval);
      countdownInterval = null;
    }
  }

  async function connect() {
    connecting = true;
    error = '';
    cancelled = false;
    countdown = CONNECTION_TIMEOUT / 1000;
    log('info', `Подключение как ${role}...`);

    // Countdown timer
    countdownInterval = setInterval(() => {
      countdown--;
      if (countdown <= 0) {
        clearTimers();
      }
    }, 1000);

    // Create timeout promise
    const timeoutPromise = new Promise<never>((_, reject) => {
      timeoutId = setTimeout(() => {
        reject(new Error('Таймаут подключения (15 сек)'));
      }, CONNECTION_TIMEOUT);
    });

    try {
      // Race between connection and timeout
      const result = await Promise.race([
        Connect(
          settings.middlewareHost,
          settings.middlewarePort,
          role,
          role === 'client' ? inputRoomCode.toUpperCase() : ''
        ),
        timeoutPromise
      ]);

      clearTimers();

      if (cancelled) {
        log('info', 'Подключение отменено');
        return;
      }

      if (result.connected) {
        isConnected = true;
        roomCode = result.roomCode;
        log('info', 'Успешно подключено!');
      } else if (result.error) {
        error = result.error;
        log('error', `Ошибка подключения: ${result.error}`);
      }
    } catch (e: any) {
      clearTimers();
      if (!cancelled) {
        error = e.message || 'Неизвестная ошибка';
        log('error', `Ошибка подключения: ${error}`);
      }
    } finally {
      connecting = false;
      clearTimers();
    }
  }

  async function cancelConnection() {
    cancelled = true;
    clearTimers();
    connecting = false;
    error = '';
    log('info', 'Подключение отменено пользователем');

    // Try to disconnect any partial connection
    try {
      await Disconnect();
    } catch (e) {
      // Ignore errors during cancel
    }
  }

  async function disconnect() {
    try {
      await Disconnect();
      isConnected = false;
      roomCode = '';
      log('info', 'Отключено');
    } catch (e: any) {
      log('error', `Ошибка при отключении: ${e.message}`);
    }
  }

  function copyRoomCode() {
    navigator.clipboard.writeText(roomCode);
    toast('success', 'Код скопирован');
  }

  async function toggleInvisibility() {
    try {
      await ToggleInvisibility();
    } catch (e: any) {
      log('error', `Ошибка переключения невидимости: ${e.message}`);
    }
  }

  let justPasted = false;

  function handleRoomCodePaste(e: ClipboardEvent) {
    e.preventDefault();
    e.stopPropagation();
    const pastedText = e.clipboardData?.getData('text') || '';
    // Clean, uppercase, and limit to 6 characters
    const cleaned = pastedText.replace(/[^a-zA-Z0-9]/g, '').toUpperCase().slice(0, 6);
    inputRoomCode = cleaned;
    justPasted = true;
    // Reset flag after a short delay
    setTimeout(() => { justPasted = false; }, 50);
  }

  function handleRoomCodeInput(e: Event) {
    // Skip if we just pasted (to avoid conflict)
    if (justPasted) return;
    const target = e.target as HTMLInputElement;
    // Ensure uppercase and only alphanumeric
    inputRoomCode = target.value.replace(/[^a-zA-Z0-9]/g, '').toUpperCase();
  }
</script>

<div class="panel">
  <div class="panel-content">
    <div class="panel-header">
      <h1 class="panel-title">Подключение</h1>
      <p class="panel-subtitle">Настройте подключение к middleware серверу</p>
    </div>

    {#if !isConnected}
      <!-- Error Message -->
      {#if error}
        <div class="alert alert-error animate-slide-down">
          <svg class="alert-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          <span class="copyable">{error}</span>
        </div>
      {/if}

      <!-- Connecting State -->
      {#if connecting}
        <div class="card connecting-card animate-scale-in">
          <div class="connecting-content">
            <div class="connecting-spinner">
              <div class="spinner"></div>
            </div>
            <div class="connecting-info">
              <span class="connecting-title">Подключение...</span>
              <span class="connecting-detail">
                {settings.middlewareHost}:{settings.middlewarePort} ({countdown} сек)
              </span>
            </div>
            <button class="btn btn-danger" on:click={cancelConnection}>
              Отмена
            </button>
          </div>
          <div class="progress-bar">
            <div
              class="progress-fill"
              style="width: {(countdown / (CONNECTION_TIMEOUT / 1000)) * 100}%"
            ></div>
          </div>
        </div>
      {:else}
        <!-- Server Info -->
        <div class="server-info">
          <svg class="server-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
            <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
            <line x1="6" y1="6" x2="6.01" y2="6"/>
            <line x1="6" y1="18" x2="6.01" y2="18"/>
          </svg>
          <span class="server-address copyable">{settings.middlewareHost}:{settings.middlewarePort}</span>
          <span class="server-hint">(изменить в Настройках)</span>
        </div>

        <!-- Role Selection -->
        <div class="card">
          <h2 class="card-title">Режим работы</h2>
          <div class="role-grid">
            <button
              class="role-card {role === 'controller' ? 'active' : ''}"
              on:click={() => role = 'controller'}
            >
              <div class="role-icon">
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                  <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
                  <line x1="8" y1="21" x2="16" y2="21"/>
                  <line x1="12" y1="17" x2="12" y2="21"/>
                  <circle cx="12" cy="10" r="3"/>
                </svg>
              </div>
              <div class="role-info">
                <span class="role-name">Controller</span>
                <span class="role-desc">Создать комнату и управлять клиентом</span>
              </div>
              {#if role === 'controller'}
                <div class="role-check">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                </div>
              {/if}
            </button>

            <button
              class="role-card {role === 'client' ? 'active' : ''}"
              on:click={() => role = 'client'}
            >
              <div class="role-icon">
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                  <rect x="5" y="2" width="14" height="20" rx="2" ry="2"/>
                  <line x1="12" y1="18" x2="12.01" y2="18"/>
                </svg>
              </div>
              <div class="role-info">
                <span class="role-name">Client</span>
                <span class="role-desc">Присоединиться к существующей комнате</span>
              </div>
              {#if role === 'client'}
                <div class="role-check">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                </div>
              {/if}
            </button>
          </div>
        </div>

        <!-- Room Code (for client) -->
        {#if role === 'client'}
          <div class="card animate-slide-up">
            <h2 class="card-title">Код комнаты</h2>
            <input
              type="text"
              class="input room-input"
              bind:value={inputRoomCode}
              on:paste={handleRoomCodePaste}
              on:input={handleRoomCodeInput}
              placeholder="ABC123"
              maxlength="6"
            />
            <p class="input-hint">Введите код, полученный от контроллера</p>
          </div>
        {/if}

        <!-- Connect Button -->
        <button
          class="btn btn-primary connect-btn"
          on:click={connect}
          disabled={role === 'client' && inputRoomCode.length !== 6}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            {#if role === 'controller'}
              <polygon points="5 3 19 12 5 21 5 3"/>
            {:else}
              <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
              <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
            {/if}
          </svg>
          <span>{role === 'controller' ? 'Создать комнату' : 'Подключиться'}</span>
        </button>
      {/if}
    {:else}
      <!-- Connected State -->
      <div class="card connected-card animate-scale-in">
        <div class="connected-content">
          <div class="connected-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          </div>
          <div class="connected-info">
            <span class="connected-title">Подключено</span>
            <span class="connected-detail copyable">
              {settings.middlewareHost}:{settings.middlewarePort}
            </span>
          </div>
          <button class="btn btn-danger" on:click={disconnect}>
            Отключиться
          </button>
        </div>
      </div>

      <!-- Invisibility Mode -->
      <div class="card invisibility-card {isInvisible ? 'invisibility-active' : 'invisibility-inactive'} animate-scale-in">
        <div class="invisibility-content">
          <div class="invisibility-icon {isInvisible ? 'active' : ''}">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              {#if isInvisible}
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/>
                <line x1="1" y1="1" x2="23" y2="23"/>
              {:else}
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                <circle cx="12" cy="12" r="3"/>
              {/if}
            </svg>
          </div>
          <div class="invisibility-info">
            <span class="invisibility-title {isInvisible ? 'active' : ''}">
              {isInvisible ? 'Невидим' : 'Режим невидимости'}
            </span>
            <span class="invisibility-detail">
              {isInvisible ? 'Скрыто от захвата экрана' : 'Видимо при screen share'}
            </span>
          </div>
          <button
            class="btn {isInvisible ? 'btn-warning-outline' : 'btn-warning'}"
            on:click={toggleInvisibility}
          >
            {isInvisible ? 'Выключить' : 'Включить'}
          </button>
        </div>
        <p class="invisibility-hint">
          Горячая клавиша: <kbd>{settings.hotkeyInvisibility || 'Ctrl+Shift+I'}</kbd>
        </p>
      </div>

      <!-- Room Code Display -->
      <div class="card">
        <h2 class="card-title">
          {role === 'controller' ? 'Ваш код комнаты' : 'Комната'}
        </h2>
        <div class="room-display">
          <div class="room-code-box">
            <span class="room-code-text copyable">{roomCode}</span>
          </div>
          <button class="btn btn-secondary btn-icon copy-btn" on:click={copyRoomCode} title="Копировать">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
            </svg>
          </button>
        </div>
        {#if role === 'controller'}
          <p class="input-hint">Поделитесь этим кодом с клиентом для подключения</p>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .panel {
    height: 100%;
    padding: var(--space-8);
    overflow: auto;
  }

  .panel-content {
    max-width: 560px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .panel-header {
    margin-bottom: var(--space-2);
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

  /* Alert */
  .alert {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4);
    border-radius: var(--radius-lg);
  }

  .alert-error {
    background: var(--color-error-muted);
    border: 1px solid rgba(255, 69, 58, 0.3);
    color: var(--color-error);
  }

  .alert-icon {
    flex-shrink: 0;
  }

  /* Server Info */
  .server-info {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--bg-tertiary);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
  }

  .server-icon {
    color: var(--text-tertiary);
  }

  .server-address {
    font-family: var(--font-mono);
    color: var(--text-primary);
  }

  .server-hint {
    color: var(--text-muted);
    margin-left: auto;
  }

  /* Card Title */
  .card-title {
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 var(--space-4) 0;
  }

  /* Role Grid */
  .role-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }

  .role-card {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-6) var(--space-4);
    background: var(--bg-tertiary);
    border: 2px solid var(--border-primary);
    border-radius: var(--radius-xl);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
    text-align: center;
  }

  .role-card:hover {
    border-color: var(--border-hover);
    background: var(--bg-hover);
  }

  .role-card.active {
    border-color: var(--accent-primary);
    background: var(--accent-primary-muted);
  }

  .role-icon {
    width: 56px;
    height: 56px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-hover);
    border-radius: var(--radius-lg);
    color: var(--text-secondary);
    transition: all var(--duration-fast) var(--ease-out);
  }

  .role-card.active .role-icon {
    background: var(--accent-primary);
    color: white;
  }

  .role-info {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .role-name {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
  }

  .role-desc {
    font-size: var(--text-xs);
    color: var(--text-tertiary);
  }

  .role-check {
    position: absolute;
    top: var(--space-3);
    right: var(--space-3);
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-primary);
    color: white;
    border-radius: var(--radius-full);
  }

  /* Room Input */
  .room-input {
    text-align: center;
    font-size: var(--text-2xl);
    font-family: var(--font-mono);
    letter-spacing: 0.3em;
    text-transform: uppercase;
    padding: var(--space-5);
  }

  .input-hint {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
    margin: var(--space-3) 0 0 0;
    text-align: center;
  }

  /* Connect Button */
  .connect-btn {
    width: 100%;
    padding: var(--space-4) var(--space-6);
    font-size: var(--text-base);
    font-weight: 600;
  }

  /* Connecting State */
  .connecting-card {
    border-color: var(--accent-primary);
    background: var(--accent-primary-muted);
    padding: 0;
    overflow: hidden;
  }

  .connecting-content {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-5);
  }

  .connecting-spinner {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-primary-muted);
    border-radius: var(--radius-lg);
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--accent-primary);
    border-top-color: transparent;
    border-radius: var(--radius-full);
    animation: spin 1s linear infinite;
  }

  .connecting-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .connecting-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--accent-primary);
  }

  .connecting-detail {
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }

  .progress-bar {
    height: 3px;
    background: var(--bg-tertiary);
  }

  .progress-fill {
    height: 100%;
    background: var(--accent-primary);
    transition: width 1s linear;
  }

  /* Connected State */
  .connected-card {
    border-color: var(--color-success);
    background: var(--color-success-muted);
  }

  .connected-content {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .connected-icon {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--color-success);
    color: white;
    border-radius: var(--radius-lg);
  }

  .connected-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .connected-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--color-success);
  }

  .connected-detail {
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }

  /* Room Display */
  .room-display {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .room-code-box {
    flex: 1;
    padding: var(--space-5);
    background: var(--bg-tertiary);
    border-radius: var(--radius-xl);
    text-align: center;
  }

  .room-code-text {
    font-size: var(--text-3xl);
    font-family: var(--font-mono);
    font-weight: 700;
    color: var(--accent-primary);
    letter-spacing: 0.2em;
  }

  .copy-btn {
    height: 60px;
    width: 60px;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* Invisibility Mode */
  .invisibility-card {
    transition: all var(--duration-fast) var(--ease-out);
  }

  .invisibility-card.invisibility-inactive {
    border-color: var(--border-primary);
    background: var(--bg-secondary);
  }

  .invisibility-card.invisibility-active {
    border-color: var(--color-warning);
    background: rgba(255, 159, 10, 0.1);
  }

  .invisibility-content {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .invisibility-icon {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
    border-radius: var(--radius-lg);
    transition: all var(--duration-fast) var(--ease-out);
  }

  .invisibility-icon.active {
    background: var(--color-warning);
    color: white;
  }

  .invisibility-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .invisibility-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-primary);
    transition: color var(--duration-fast) var(--ease-out);
  }

  .invisibility-title.active {
    color: var(--color-warning);
  }

  .invisibility-detail {
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }

  .invisibility-hint {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: var(--space-3) 0 0 0;
    text-align: center;
  }

  .invisibility-hint kbd {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    background: var(--bg-tertiary);
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-primary);
  }

  .btn-warning {
    background: var(--color-warning);
    color: white;
    border: 2px solid var(--color-warning);
  }

  .btn-warning:hover {
    background: var(--color-warning-hover, #e68a00);
    border-color: var(--color-warning-hover, #e68a00);
  }

  .btn-warning-outline {
    background: transparent;
    color: var(--color-warning);
    border: 2px solid var(--color-warning);
  }

  .btn-warning-outline:hover {
    background: rgba(255, 159, 10, 0.1);
  }
</style>
