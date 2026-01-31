<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
  import { SendText, TypeBuffer, ClearKeyboardBuffer, ToggleInputMode, GetInputMode, GetBufferStatus, GetHotkeys } from '../../wailsjs/go/app/App';

  export let isConnected: boolean;
  export let role: string;
  export let receivedText: string = '';

  let text = '';
  let inputMode = false;
  let sending = false;
  let typing = false;

  // Buffer status for progress display
  let bufferLength = 0;
  let bufferPosition = 0;
  let bufferRemaining = 0;

  // Hotkey display - loaded from settings
  let hotkeyToggle = 'Ctrl+Shift+J';
  let hotkeyPaste = 'Ctrl+Shift+V';

  let updateInterval: number;

  onMount(async () => {
    // Load initial input mode state and hotkeys
    try {
      inputMode = await GetInputMode();
      await updateBufferStatus();

      // Load hotkeys from settings
      const hotkeys = await GetHotkeys();
      if (hotkeys.toggle) hotkeyToggle = hotkeys.toggle;
      if (hotkeys.paste) hotkeyPaste = hotkeys.paste;
    } catch (e) {
      console.error('Failed to get input mode:', e);
    }

    // Listen for input mode changes (e.g., from hotkey)
    EventsOn('inputModeChanged', (enabled: boolean) => {
      inputMode = enabled;
      updateBufferStatus();
    });

    // Listen for buffer exhausted
    EventsOn('bufferExhausted', () => {
      inputMode = false;
      bufferRemaining = 0;
      bufferPosition = bufferLength;
    });

    // Listen for text received - simple string
    EventsOn('textReceived', (data: string) => {
      console.log('textReceived:', data, typeof data);
      receivedText = String(data); // Ensure it's a string
      bufferLength = receivedText.length;
      bufferPosition = 0;
      bufferRemaining = receivedText.length;
    });

    // Periodically update buffer status when input mode is active
    updateInterval = setInterval(() => {
      if (inputMode) {
        updateBufferStatus();
      }
    }, 500);
  });

  onDestroy(() => {
    EventsOff('inputModeChanged');
    EventsOff('bufferExhausted');
    EventsOff('textReceived');
    if (updateInterval) clearInterval(updateInterval);
  });

  async function updateBufferStatus() {
    try {
      const status = await GetBufferStatus();
      bufferLength = status.length;
      bufferPosition = status.position;
      bufferRemaining = status.remaining;
    } catch (e) {
      // Ignore errors
    }
  }

  async function sendText() {
    if (!text.trim() || sending) return;

    sending = true;
    try {
      await SendText(text);
      text = '';
    } catch (e) {
      console.error('Failed to send text:', e);
    } finally {
      sending = false;
    }
  }

  async function toggleInputModeHandler() {
    try {
      inputMode = await ToggleInputMode();
      await updateBufferStatus();
    } catch (e) {
      console.error('Failed to toggle input mode:', e);
    }
  }

  async function typeBuffer() {
    if (typing) return;
    typing = true;
    try {
      await TypeBuffer();
      receivedText = '';
    } catch (e) {
      console.error('Failed to type buffer:', e);
    } finally {
      typing = false;
    }
  }

  async function clearBuffer() {
    try {
      await ClearKeyboardBuffer();
      receivedText = '';
      bufferLength = 0;
      bufferPosition = 0;
      bufferRemaining = 0;
    } catch (e) {
      console.error('Failed to clear buffer:', e);
    }
  }

  // Handle Enter key for sending
  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      sendText();
    }
  }

  // Calculate progress percentage
  $: progressPercent = bufferLength > 0 ? Math.round((bufferPosition / bufferLength) * 100) : 0;
</script>

<div class="panel">
  <div class="panel-content">
    <div class="panel-header">
      <h1 class="panel-title">Текст</h1>
      <p class="panel-subtitle">
        {role === 'controller'
          ? 'Отправьте текст для ввода на клиенте'
          : 'Управление буфером клавиатуры'}
      </p>
    </div>

    {#if !isConnected}
      <div class="empty-state card">
        <div class="empty-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <polyline points="4 7 4 4 20 4 20 7"/>
            <line x1="9" y1="20" x2="15" y2="20"/>
            <line x1="12" y1="4" x2="12" y2="20"/>
          </svg>
        </div>
        <p class="empty-title">Сначала подключитесь к комнате</p>
      </div>
    {:else}
      <!-- Input Mode Status (for client) -->
      {#if role === 'client'}
        <div class="card mode-card {inputMode ? 'active' : ''}">
          <div class="mode-content">
            <div class="mode-icon {inputMode ? 'active' : ''}">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                {#if inputMode}
                  <rect x="2" y="4" width="20" height="16" rx="2" ry="2"/>
                  <path d="M6 8h.001"/>
                  <path d="M10 8h.001"/>
                  <path d="M14 8h.001"/>
                  <path d="M18 8h.001"/>
                  <path d="M8 12h.001"/>
                  <path d="M12 12h.001"/>
                  <path d="M16 12h.001"/>
                  <path d="M7 16h10"/>
                {:else}
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/>
                {/if}
              </svg>
            </div>
            <div class="mode-info">
              <span class="mode-title {inputMode ? 'active' : ''}">
                Режим ввода {inputMode ? 'АКТИВЕН' : 'ВЫКЛЮЧЕН'}
              </span>
              <span class="mode-subtitle">
                {#if inputMode}
                  {#if bufferRemaining > 0}
                    Нажимайте любые клавиши для ввода текста
                  {:else}
                    Буфер пуст - дождитесь текста от контроллера
                  {/if}
                {:else}
                  Текст сохраняется в буфер. Нажмите {hotkeyToggle} для активации
                {/if}
              </span>
            </div>
            <button
              class="btn {inputMode ? 'btn-success' : 'btn-secondary'}"
              on:click={toggleInputModeHandler}
            >
              {inputMode ? 'Активен' : 'Включить'}
            </button>
          </div>

          {#if inputMode && bufferRemaining > 0}
            <div class="progress-section">
              <div class="progress-header">
                <span>Прогресс: {bufferPosition} / {bufferLength} символов</span>
                <span>{progressPercent}%</span>
              </div>
              <div class="progress-bar">
                <div class="progress-fill" style="width: {progressPercent}%"></div>
              </div>
              <p class="progress-hint">Нажимайте любые клавиши - они будут заменены на текст из буфера</p>
            </div>
          {/if}
        </div>

        <!-- Received Text Display -->
        {#if receivedText}
          <div class="card">
            <div class="card-header">
              <h2 class="card-title">
                {inputMode ? 'Оставшийся текст в буфере' : 'Полученный текст'}
              </h2>
              <span class="card-badge">{bufferRemaining} символов осталось</span>
            </div>
            <div class="text-display copyable">
              {#if inputMode}
                <span class="text-done">{receivedText.substring(0, bufferPosition)}</span><span class="text-remaining">{receivedText.substring(bufferPosition)}</span>
              {:else}
                {receivedText}
              {/if}
            </div>
            <div class="card-actions">
              {#if !inputMode}
                <button class="btn btn-secondary" on:click={clearBuffer}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="3 6 5 6 21 6"/>
                    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                  </svg>
                  <span>Очистить буфер</span>
                </button>
              {:else}
                <button class="btn btn-secondary" on:click={toggleInputModeHandler}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="6" y="4" width="4" height="16"/>
                    <rect x="14" y="4" width="4" height="16"/>
                  </svg>
                  <span>Остановить</span>
                </button>
                <button class="btn btn-secondary" on:click={clearBuffer}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="3 6 5 6 21 6"/>
                    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                  </svg>
                  <span>Очистить</span>
                </button>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Show hint when no text -->
        {#if !receivedText && !inputMode}
          <div class="card empty-card">
            <div class="empty-icon">
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
                <polyline points="22,6 12,13 2,6"/>
              </svg>
            </div>
            <p class="empty-title">Ожидание текста от контроллера...</p>
            <p class="empty-subtitle">Когда контроллер отправит текст, он появится здесь</p>
          </div>
        {/if}
      {/if}

      <!-- Text Input (for controller) -->
      {#if role === 'controller'}
        <div class="card">
          <h2 class="card-title">Отправить текст</h2>
          <textarea
            class="input text-input"
            bind:value={text}
            on:keydown={handleKeyDown}
            placeholder="Введите текст для отправки на клиент...

Ctrl+Enter для быстрой отправки"
          ></textarea>
          <div class="card-actions">
            <button
              class="btn btn-primary flex-1"
              on:click={sendText}
              disabled={!text.trim() || sending}
            >
              {#if sending}
                <div class="spinner-sm"></div>
                <span>Отправка...</span>
              {:else}
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="22" y1="2" x2="11" y2="13"/>
                  <polygon points="22 2 15 22 11 13 2 9 22 2"/>
                </svg>
                <span>Отправить</span>
              {/if}
            </button>
            <button
              class="btn btn-secondary"
              on:click={() => text = ''}
              disabled={!text}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="3 6 5 6 21 6"/>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
              </svg>
              <span>Очистить</span>
            </button>
          </div>
          <p class="input-hint">На клиенте текст будет вводиться посимвольно при нажатии любых клавиш</p>
        </div>
      {/if}

      <!-- Hotkeys Reference -->
      <div class="hotkeys-card">
        <h3 class="hotkeys-title">Горячие клавиши</h3>
        <div class="hotkeys-grid">
          <div class="hotkey-item">
            <span class="hotkey-label">Переключить режим ввода</span>
            <code class="hotkey-key">{hotkeyToggle}</code>
          </div>
          <div class="hotkey-item">
            <span class="hotkey-label">Ввести весь буфер сразу</span>
            <code class="hotkey-key">{hotkeyPaste}</code>
          </div>
          {#if role === 'controller'}
            <div class="hotkey-item">
              <span class="hotkey-label">Отправить текст</span>
              <code class="hotkey-key">Ctrl+Enter</code>
            </div>
          {/if}
        </div>
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
    max-width: 640px;
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

  /* Mode Card */
  .mode-card {
    border-color: var(--border-primary);
  }

  .mode-card.active {
    border-color: var(--color-success);
    background: var(--color-success-muted);
  }

  .mode-content {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .mode-icon {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    border-radius: var(--radius-lg);
    color: var(--text-muted);
  }

  .mode-icon.active {
    background: var(--color-success);
    color: white;
  }

  .mode-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .mode-title {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-secondary);
  }

  .mode-title.active {
    color: var(--color-success);
  }

  .mode-subtitle {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  /* Progress Section */
  .progress-section {
    margin-top: var(--space-4);
    padding: var(--space-4);
    background: rgba(48, 209, 88, 0.1);
    border: 1px solid rgba(48, 209, 88, 0.2);
    border-radius: var(--radius-lg);
  }

  .progress-header {
    display: flex;
    justify-content: space-between;
    font-size: var(--text-sm);
    color: var(--color-success);
    margin-bottom: var(--space-2);
  }

  .progress-bar {
    height: 6px;
    background: var(--bg-tertiary);
    border-radius: var(--radius-full);
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-success);
    transition: width var(--duration-fast) var(--ease-out);
  }

  .progress-hint {
    font-size: var(--text-xs);
    color: var(--text-tertiary);
    margin: var(--space-2) 0 0 0;
  }

  /* Card Styles */
  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-4);
  }

  .card-title {
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .card-badge {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  .card-actions {
    display: flex;
    gap: var(--space-3);
    margin-top: var(--space-4);
  }

  .flex-1 {
    flex: 1;
  }

  /* Text Display */
  .text-display {
    background: var(--bg-tertiary);
    border-radius: var(--radius-lg);
    padding: var(--space-4);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    color: var(--text-secondary);
    max-height: 160px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .text-done {
    color: var(--text-muted);
  }

  .text-remaining {
    color: var(--color-success);
    border-left: 2px solid var(--color-success);
    padding-left: 2px;
  }

  /* Text Input */
  .text-input {
    min-height: 180px;
    resize: vertical;
    font-family: var(--font-mono);
  }

  .input-hint {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
    margin: var(--space-4) 0 0 0;
  }

  /* Empty Card */
  .empty-card {
    border-style: dashed;
    text-align: center;
    padding: var(--space-8);
  }

  .empty-card .empty-icon {
    width: 64px;
    height: 64px;
    margin: 0 auto var(--space-4);
  }

  /* Hotkeys Card */
  .hotkeys-card {
    padding: var(--space-4);
    background: var(--bg-tertiary);
    border: 1px solid var(--border-secondary);
    border-radius: var(--radius-lg);
  }

  .hotkeys-title {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-secondary);
    margin: 0 0 var(--space-3) 0;
  }

  .hotkeys-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .hotkey-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: var(--text-sm);
  }

  .hotkey-label {
    color: var(--text-tertiary);
  }

  .hotkey-key {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--accent-primary);
    background: var(--accent-primary-muted);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
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
</style>
