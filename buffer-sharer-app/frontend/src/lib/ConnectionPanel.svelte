<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { Connect, Disconnect } from '../../wailsjs/go/main/App';

  export let isConnected: boolean;
  export let roomCode: string;
  export let role: string;
  export let settings: any;

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
    log('info', 'Код комнаты скопирован');
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

<div class="h-full p-8 overflow-auto">
  <div class="max-w-2xl mx-auto">
    <h2 class="text-2xl font-bold text-white mb-2">Подключение</h2>
    <p class="text-gray-400 mb-8">Настройте подключение к middleware серверу</p>

    {#if !isConnected}
      <!-- Error Message -->
      {#if error}
        <div class="mb-6 p-4 bg-red-500/10 border border-red-500/30 rounded-xl text-red-400">
          {error}
        </div>
      {/if}

      <!-- Connecting State -->
      {#if connecting}
        <div class="card mb-6 border-primary-500/30 bg-primary-500/5">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-full bg-primary-500/20 flex items-center justify-center">
                <div class="w-6 h-6 border-2 border-primary-400 border-t-transparent rounded-full animate-spin"></div>
              </div>
              <div>
                <p class="font-semibold text-primary-400">Подключение...</p>
                <p class="text-sm text-gray-400">
                  {settings.middlewareHost}:{settings.middlewarePort} ({countdown} сек)
                </p>
              </div>
            </div>
            <button
              class="btn btn-danger"
              on:click={cancelConnection}
            >
              Отмена
            </button>
          </div>
          <!-- Progress bar -->
          <div class="mt-4 h-1 bg-dark-700 rounded-full overflow-hidden">
            <div
              class="h-full bg-primary-500 transition-all duration-1000 ease-linear"
              style="width: {(countdown / (CONNECTION_TIMEOUT / 1000)) * 100}%"
            ></div>
          </div>
        </div>
      {:else}
        <!-- Server Info -->
        <div class="mb-6 p-3 rounded-lg bg-dark-800 border border-dark-700">
          <p class="text-sm text-gray-400">
            Сервер: <span class="text-white font-mono">{settings.middlewareHost}:{settings.middlewarePort}</span>
            <span class="text-gray-600 ml-2">(изменить в Настройках)</span>
          </p>
        </div>

        <!-- Role Selection -->
        <div class="card mb-6">
          <h3 class="text-lg font-semibold text-white mb-4">Режим работы</h3>
          <div class="grid grid-cols-2 gap-4">
            <button
              class="p-6 rounded-xl border-2 transition-all duration-200 text-left
                     {role === 'controller'
                       ? 'border-primary-500 bg-primary-500/10'
                       : 'border-dark-600 hover:border-dark-500 bg-dark-800'}"
              on:click={() => role = 'controller'}
            >
              <div class="text-3xl mb-3">🎮</div>
              <div class="font-semibold text-white">Controller</div>
              <p class="text-sm text-gray-400 mt-1">
                Создать комнату и управлять клиентом
              </p>
            </button>

            <button
              class="p-6 rounded-xl border-2 transition-all duration-200 text-left
                     {role === 'client'
                       ? 'border-primary-500 bg-primary-500/10'
                       : 'border-dark-600 hover:border-dark-500 bg-dark-800'}"
              on:click={() => role = 'client'}
            >
              <div class="text-3xl mb-3">💻</div>
              <div class="font-semibold text-white">Client</div>
              <p class="text-sm text-gray-400 mt-1">
                Присоединиться к существующей комнате
              </p>
            </button>
          </div>
        </div>

        <!-- Room Code (for client) -->
        {#if role === 'client'}
          <div class="card mb-6">
            <h3 class="text-lg font-semibold text-white mb-4">Код комнаты</h3>
            <input
              type="text"
              class="input text-center text-2xl font-mono tracking-widest uppercase"
              bind:value={inputRoomCode}
              on:paste={handleRoomCodePaste}
              on:input={handleRoomCodeInput}
              placeholder="ABC123"
              maxlength="6"
            />
            <p class="text-sm text-gray-500 mt-2">
              Введите код, полученный от контроллера
            </p>
          </div>
        {/if}

        <!-- Connect Button -->
        <button
          class="btn btn-primary w-full py-4 text-lg font-semibold"
          on:click={connect}
          disabled={role === 'client' && inputRoomCode.length !== 6}
        >
          {role === 'controller' ? '🚀 Создать комнату' : '🔗 Подключиться'}
        </button>
      {/if}
    {:else}
      <!-- Connected State -->
      <div class="card mb-6 border-emerald-500/30 bg-emerald-500/5">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <div class="w-12 h-12 rounded-full bg-emerald-500/20 flex items-center justify-center">
              <span class="text-2xl">✓</span>
            </div>
            <div>
              <p class="font-semibold text-emerald-400">Подключено</p>
              <p class="text-sm text-gray-400">
                {settings.middlewareHost}:{settings.middlewarePort}
              </p>
            </div>
          </div>
          <button
            class="btn btn-danger"
            on:click={disconnect}
          >
            Отключиться
          </button>
        </div>
      </div>

      <!-- Room Code Display -->
      <div class="card">
        <h3 class="text-lg font-semibold text-white mb-4">
          {role === 'controller' ? 'Ваш код комнаты' : 'Комната'}
        </h3>
        <div class="flex items-center gap-4">
          <div class="flex-1 bg-dark-900 rounded-xl p-6 text-center">
            <span class="text-4xl font-mono font-bold text-primary-400 tracking-widest">
              {roomCode}
            </span>
          </div>
          <button
            class="btn btn-secondary p-4"
            on:click={copyRoomCode}
            title="Копировать"
          >
            📋
          </button>
        </div>
        {#if role === 'controller'}
          <p class="text-sm text-gray-500 mt-4">
            Поделитесь этим кодом с клиентом для подключения
          </p>
        {/if}
      </div>
    {/if}
  </div>
</div>
