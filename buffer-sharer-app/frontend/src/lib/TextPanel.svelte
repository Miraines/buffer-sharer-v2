<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
  import { SendText, TypeBuffer, ClearKeyboardBuffer, ToggleInputMode, GetInputMode, GetBufferStatus, GetHotkeys } from '../../wailsjs/go/main/App';

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

<div class="h-full p-8 overflow-auto">
  <div class="max-w-3xl mx-auto">
    <h2 class="text-2xl font-bold text-white mb-2">Текст</h2>
    <p class="text-gray-400 mb-8">
      {role === 'controller'
        ? 'Отправьте текст для ввода на клиенте'
        : 'Управление буфером клавиатуры'}
    </p>

    {#if !isConnected}
      <div class="card flex flex-col items-center justify-center py-20">
        <span class="text-6xl mb-4 opacity-50">📝</span>
        <p class="text-gray-400 text-lg">Сначала подключитесь к комнате</p>
      </div>
    {:else}
      <!-- Input Mode Status (for client) -->
      {#if role === 'client'}
        <div class="card mb-6 {inputMode ? 'border-green-500/30 bg-green-500/5' : 'border-dark-600'}">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-full {inputMode ? 'bg-green-500/20' : 'bg-dark-700'} flex items-center justify-center">
                <span class="text-2xl">{inputMode ? '⌨️' : '💤'}</span>
              </div>
              <div>
                <p class="font-semibold {inputMode ? 'text-green-400' : 'text-gray-400'}">
                  Режим ввода {inputMode ? 'АКТИВЕН' : 'ВЫКЛЮЧЕН'}
                </p>
                <p class="text-sm text-gray-500">
                  {#if inputMode}
                    {#if bufferRemaining > 0}
                      Нажимайте любые клавиши для ввода текста
                    {:else}
                      Буфер пуст - дождитесь текста от контроллера
                    {/if}
                  {:else}
                    Текст сохраняется в буфер. Нажмите {hotkeyToggle} для активации
                  {/if}
                </p>
              </div>
            </div>
            <button
              class="btn {inputMode ? 'bg-green-600 hover:bg-green-700 text-white' : 'btn-secondary'}"
              on:click={toggleInputModeHandler}
            >
              {inputMode ? '✓ Активен' : 'Включить'}
            </button>
          </div>

          {#if inputMode && bufferRemaining > 0}
            <div class="mt-4 p-3 bg-green-500/10 rounded-lg border border-green-500/20">
              <div class="flex items-center justify-between mb-2">
                <p class="text-sm text-green-400">
                  Прогресс: {bufferPosition} / {bufferLength} символов
                </p>
                <span class="text-sm text-green-400">{progressPercent}%</span>
              </div>
              <div class="w-full bg-dark-700 rounded-full h-2">
                <div
                  class="bg-green-500 h-2 rounded-full transition-all duration-200"
                  style="width: {progressPercent}%"
                ></div>
              </div>
              <p class="text-xs text-gray-500 mt-2">
                💡 Нажимайте любые клавиши - они будут заменены на текст из буфера
              </p>
            </div>
          {/if}
        </div>

        <!-- Received Text Display -->
        {#if receivedText}
          <div class="card mb-6">
            <div class="flex items-center justify-between mb-3">
              <h3 class="font-semibold text-white">
                {inputMode ? 'Оставшийся текст в буфере' : 'Полученный текст'}
              </h3>
              <span class="text-sm text-gray-500">{bufferRemaining} символов осталось</span>
            </div>
            <div class="bg-dark-900 rounded-lg p-4 font-mono text-sm text-gray-300 max-h-40 overflow-auto whitespace-pre-wrap">
              {#if inputMode}
                <span class="text-gray-600">{receivedText.substring(0, bufferPosition)}</span><span class="text-green-400 border-l-2 border-green-400">{receivedText.substring(bufferPosition)}</span>
              {:else}
                {receivedText}
              {/if}
            </div>
            {#if !inputMode}
              <div class="flex gap-4 mt-4">
                <button class="btn btn-secondary" on:click={clearBuffer}>
                  🗑️ Очистить буфер
                </button>
              </div>
            {:else}
              <div class="flex gap-4 mt-4">
                <button
                  class="btn btn-secondary flex-1"
                  on:click={toggleInputModeHandler}
                >
                  ⏹️ Остановить
                </button>
                <button class="btn btn-secondary" on:click={clearBuffer}>
                  🗑️ Очистить
                </button>
              </div>
            {/if}
          </div>
        {/if}

        <!-- Show hint when no text -->
        {#if !receivedText && !inputMode}
          <div class="card mb-6 border-dashed border-2 border-dark-600">
            <div class="flex flex-col items-center justify-center py-8 text-center">
              <span class="text-4xl mb-3 opacity-50">📨</span>
              <p class="text-gray-400">Ожидание текста от контроллера...</p>
              <p class="text-sm text-gray-600 mt-2">Когда контроллер отправит текст, он появится здесь</p>
            </div>
          </div>
        {/if}
      {/if}

      <!-- Text Input (for controller) -->
      {#if role === 'controller'}
        <div class="card">
          <h3 class="font-semibold text-white mb-4">Отправить текст</h3>
          <textarea
            class="input min-h-[200px] resize-none font-mono"
            bind:value={text}
            on:keydown={handleKeyDown}
            placeholder="Введите текст для отправки на клиент...&#10;&#10;Ctrl+Enter для быстрой отправки"
          ></textarea>
          <div class="flex gap-4 mt-4">
            <button
              class="btn btn-primary flex-1"
              on:click={sendText}
              disabled={!text.trim() || sending}
            >
              {sending ? '⏳ Отправка...' : '📤 Отправить'}
            </button>
            <button
              class="btn btn-secondary"
              on:click={() => text = ''}
              disabled={!text}
            >
              🗑️ Очистить
            </button>
          </div>
          <p class="text-sm text-gray-500 mt-4">
            💡 На клиенте текст будет вводиться посимвольно при нажатии любых клавиш
          </p>
        </div>
      {/if}

      <!-- Hotkeys Reference -->
      <div class="mt-6 p-4 bg-dark-800/50 rounded-lg border border-dark-700">
        <h4 class="text-sm font-semibold text-gray-400 mb-3">Горячие клавиши</h4>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-2 text-sm">
          <div class="flex justify-between">
            <span class="text-gray-500">Переключить режим ввода</span>
            <code class="text-primary-400">{hotkeyToggle}</code>
          </div>
          <div class="flex justify-between">
            <span class="text-gray-500">Ввести весь буфер сразу</span>
            <code class="text-primary-400">{hotkeyPaste}</code>
          </div>
          {#if role === 'controller'}
            <div class="flex justify-between">
              <span class="text-gray-500">Отправить текст</span>
              <code class="text-primary-400">Ctrl+Enter</code>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</div>
