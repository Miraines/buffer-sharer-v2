<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { SaveSettings, SelectScreenshotDirectory, GetScreenshotSaveDir } from '../../wailsjs/go/main/App';
  import { main } from '../../wailsjs/go/models';

  export let settings: {
    middlewareHost: string;
    middlewarePort: number;
    screenshotInterval: number;
    screenshotQuality: number;
    clipboardSync: boolean;
    hotkeyToggle: string;
    hotkeyScreenshot: string;
    hotkeyPaste: string;
    autoConnect: boolean;
    lastRole: string;
    lastRoomCode: string;
    soundEnabled: boolean;
    theme: string;
    screenshotSaveDir: string;
    screenshotHistoryLimit: number;
  };

  // Определяем платформу
  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
  const modifierKey = isMac ? '⌘' : 'Ctrl';
  const altKey = isMac ? '⌥' : 'Alt';

  // Состояние записи хоткея
  let recordingHotkey: string | null = null;
  let recordedKeys: string[] = [];

  function startRecording(field: string) {
    recordingHotkey = field;
    recordedKeys = [];
  }

  function stopRecording() {
    recordingHotkey = null;
    recordedKeys = [];
  }

  function handleKeyDown(event: KeyboardEvent, field: string) {
    if (recordingHotkey !== field) return;

    event.preventDefault();
    event.stopPropagation();

    const keys: string[] = [];

    // On macOS: metaKey = Cmd, ctrlKey = Ctrl
    // On Windows/Linux: ctrlKey = Ctrl, metaKey = Win
    if (isMac) {
      if (event.metaKey) keys.push('Cmd');
      if (event.ctrlKey) keys.push('Ctrl');
    } else {
      if (event.ctrlKey) keys.push('Ctrl');
      if (event.metaKey) keys.push('Win');
    }

    if (event.altKey) keys.push(isMac ? 'Option' : 'Alt');
    if (event.shiftKey) keys.push('Shift');

    // Добавляем основную клавишу если это не модификатор
    const key = event.key.toUpperCase();
    if (!['CONTROL', 'META', 'ALT', 'SHIFT'].includes(key)) {
      keys.push(key);

      // Записываем хоткей
      const hotkey = keys.join('+');
      if (field === 'hotkeyToggle') settings.hotkeyToggle = hotkey;
      if (field === 'hotkeyScreenshot') settings.hotkeyScreenshot = hotkey;
      if (field === 'hotkeyPaste') settings.hotkeyPaste = hotkey;

      stopRecording();
    }
  }

  const dispatch = createEventDispatcher();

  let saved = false;
  let saving = false;
  let selectingDir = false;

  function log(level: string, message: string) {
    dispatch('log', { level, message });
  }

  async function selectSaveDirectory() {
    selectingDir = true;
    try {
      const dir = await SelectScreenshotDirectory();
      if (dir) {
        settings.screenshotSaveDir = dir;
        log('info', `Папка для скриншотов: ${dir}`);
      }
    } catch (e: any) {
      log('error', `Ошибка выбора папки: ${e.message}`);
    } finally {
      selectingDir = false;
    }
  }

  async function saveSettings() {
    saving = true;
    try {
      const s = new main.Settings({
        middlewareHost: settings.middlewareHost,
        middlewarePort: settings.middlewarePort,
        screenshotInterval: settings.screenshotInterval,
        screenshotQuality: settings.screenshotQuality,
        clipboardSync: settings.clipboardSync,
        hotkeyToggle: settings.hotkeyToggle,
        hotkeyScreenshot: settings.hotkeyScreenshot,
        hotkeyPaste: settings.hotkeyPaste,
        autoConnect: settings.autoConnect,
        lastRole: settings.lastRole,
        lastRoomCode: settings.lastRoomCode,
        soundEnabled: settings.soundEnabled,
        theme: settings.theme,
        screenshotSaveDir: settings.screenshotSaveDir,
        screenshotHistoryLimit: settings.screenshotHistoryLimit || 50
      });
      await SaveSettings(s);
      saved = true;
      log('info', 'Настройки сохранены');
      setTimeout(() => saved = false, 2000);
    } catch (e: any) {
      log('error', `Ошибка сохранения настроек: ${e.message}`);
    } finally {
      saving = false;
    }
  }

  function resetToDefaults() {
    // Use platform-specific hotkeys that don't conflict with system shortcuts
    // Cmd+Option on macOS, Ctrl+Alt on Windows/Linux
    settings = {
      middlewareHost: 'localhost',
      middlewarePort: 8080,
      screenshotInterval: 4000,
      screenshotQuality: 80,
      clipboardSync: true,
      hotkeyToggle: isMac ? 'Cmd+Option+J' : 'Ctrl+Alt+J',
      hotkeyScreenshot: isMac ? 'Cmd+Option+S' : 'Ctrl+Alt+S',
      hotkeyPaste: isMac ? 'Cmd+Option+V' : 'Ctrl+Alt+V',
      autoConnect: false,
      lastRole: settings.lastRole,
      lastRoomCode: settings.lastRoomCode,
      soundEnabled: true,
      theme: 'dark',
      screenshotSaveDir: '',
      screenshotHistoryLimit: 50
    };
    log('info', 'Настройки сброшены к значениям по умолчанию');
  }
</script>

<div class="h-full p-8 overflow-auto">
  <div class="max-w-2xl mx-auto">
    <h2 class="text-2xl font-bold text-white mb-2">Настройки</h2>
    <p class="text-gray-400 mb-8">Конфигурация приложения</p>

    <!-- Server Settings -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <span>🌐</span> Сервер
      </h3>
      <div class="space-y-4">
        <div>
          <label class="block text-sm text-gray-400 mb-2">Хост Middleware</label>
          <input
            type="text"
            class="input"
            bind:value={settings.middlewareHost}
            placeholder="localhost"
          />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-2">Порт</label>
          <input
            type="number"
            class="input"
            bind:value={settings.middlewarePort}
            min="1"
            max="65535"
          />
        </div>
      </div>
    </div>

    <!-- Screenshot Settings -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <span>📷</span> Скриншоты
      </h3>
      <div class="space-y-4">
        <div>
          <div class="flex justify-between mb-2">
            <label class="text-sm text-gray-400">Интервал захвата</label>
            <span class="text-sm text-primary-400">{settings.screenshotInterval} мс</span>
          </div>
          <input
            type="range"
            class="w-full h-2 bg-dark-700 rounded-lg appearance-none cursor-pointer accent-primary-500"
            bind:value={settings.screenshotInterval}
            min="1000"
            max="10000"
            step="500"
          />
          <div class="flex justify-between text-xs text-gray-600 mt-1">
            <span>1 сек</span>
            <span>10 сек</span>
          </div>
        </div>
        <div>
          <div class="flex justify-between mb-2">
            <label class="text-sm text-gray-400">Качество JPEG</label>
            <span class="text-sm text-primary-400">{settings.screenshotQuality}%</span>
          </div>
          <input
            type="range"
            class="w-full h-2 bg-dark-700 rounded-lg appearance-none cursor-pointer accent-primary-500"
            bind:value={settings.screenshotQuality}
            min="10"
            max="100"
            step="5"
          />
          <div class="flex justify-between text-xs text-gray-600 mt-1">
            <span>Меньше размер</span>
            <span>Выше качество</span>
          </div>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-2">Папка для сохранения</label>
          <div class="flex gap-2">
            <input
              type="text"
              class="input flex-1 text-sm"
              bind:value={settings.screenshotSaveDir}
              placeholder="~/Downloads (по умолчанию)"
              readonly
            />
            <button
              class="btn btn-secondary px-4"
              on:click={selectSaveDirectory}
              disabled={selectingDir}
            >
              {#if selectingDir}
                ⏳
              {:else}
                📁
              {/if}
            </button>
          </div>
          <p class="text-xs text-gray-600 mt-1">
            Куда сохранять скриншоты по кнопке "Сохранить"
          </p>
        </div>
        <div>
          <div class="flex justify-between mb-2">
            <label class="text-sm text-gray-400">Лимит истории скриншотов</label>
            <span class="text-sm text-primary-400">{settings.screenshotHistoryLimit || 50} шт.</span>
          </div>
          <input
            type="range"
            class="w-full h-2 bg-dark-700 rounded-lg appearance-none cursor-pointer accent-primary-500"
            bind:value={settings.screenshotHistoryLimit}
            min="10"
            max="200"
            step="10"
          />
          <div class="flex justify-between text-xs text-gray-600 mt-1">
            <span>10</span>
            <span>200</span>
          </div>
          <p class="text-xs text-gray-600 mt-1">
            Старые скриншоты автоматически удаляются при превышении лимита
          </p>
        </div>
      </div>
    </div>

    <!-- Clipboard Settings -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <span>📋</span> Буфер обмена
      </h3>
      <label class="flex items-center justify-between cursor-pointer">
        <div>
          <p class="font-medium text-gray-200">Синхронизация буфера обмена</p>
          <p class="text-sm text-gray-500">Автоматически отправлять содержимое буфера</p>
        </div>
        <div class="relative">
          <input
            type="checkbox"
            class="sr-only peer"
            bind:checked={settings.clipboardSync}
          />
          <div class="w-11 h-6 bg-dark-600 rounded-full peer peer-checked:bg-primary-600
                      peer-focus:ring-2 peer-focus:ring-primary-500/50
                      after:content-[''] after:absolute after:top-0.5 after:left-0.5
                      after:bg-white after:rounded-full after:h-5 after:w-5
                      after:transition-all peer-checked:after:translate-x-5"></div>
        </div>
      </label>
    </div>

    <!-- Theme Settings -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <span>🎨</span> Тема оформления
      </h3>
      <div class="grid grid-cols-3 gap-3">
        <button
          class="p-4 rounded-xl border-2 transition-all duration-200 text-center
                 {settings.theme === 'dark'
                   ? 'border-primary-500 bg-primary-500/10'
                   : 'border-dark-600 hover:border-dark-500'}"
          on:click={() => settings.theme = 'dark'}
        >
          <div class="text-2xl mb-2">🌙</div>
          <div class="text-sm font-medium">Тёмная</div>
        </button>

        <button
          class="p-4 rounded-xl border-2 transition-all duration-200 text-center
                 {settings.theme === 'light'
                   ? 'border-primary-500 bg-primary-500/10'
                   : 'border-dark-600 hover:border-dark-500'}"
          on:click={() => settings.theme = 'light'}
        >
          <div class="text-2xl mb-2">☀️</div>
          <div class="text-sm font-medium">Светлая</div>
        </button>

        <button
          class="p-4 rounded-xl border-2 transition-all duration-200 text-center
                 {settings.theme === 'system'
                   ? 'border-primary-500 bg-primary-500/10'
                   : 'border-dark-600 hover:border-dark-500'}"
          on:click={() => settings.theme = 'system'}
        >
          <div class="text-2xl mb-2">💻</div>
          <div class="text-sm font-medium">Системная</div>
        </button>
      </div>
    </div>

    <!-- Hotkeys Settings -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <span>⌨️</span> Горячие клавиши
        <span class="text-xs text-gray-500 font-normal ml-2">
          ({isMac ? 'macOS' : 'Windows/Linux'})
        </span>
      </h3>
      <p class="text-sm text-gray-500 mb-4">
        Нажмите на поле и введите желаемую комбинацию клавиш
      </p>
      <div class="space-y-4">
        <div>
          <label class="block text-sm text-gray-400 mb-2">Переключить режим ввода</label>
          <div class="relative">
            <input
              type="text"
              class="input font-mono pr-20 {recordingHotkey === 'hotkeyToggle' ? 'ring-2 ring-primary-500' : ''}"
              value={settings.hotkeyToggle}
              readonly
              on:focus={() => startRecording('hotkeyToggle')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyToggle')}
              placeholder="{modifierKey}+Shift+J"
            />
            {#if recordingHotkey === 'hotkeyToggle'}
              <span class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-primary-400 animate-pulse">
                Запись...
              </span>
            {/if}
          </div>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-2">Сделать скриншот</label>
          <div class="relative">
            <input
              type="text"
              class="input font-mono pr-20 {recordingHotkey === 'hotkeyScreenshot' ? 'ring-2 ring-primary-500' : ''}"
              value={settings.hotkeyScreenshot}
              readonly
              on:focus={() => startRecording('hotkeyScreenshot')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyScreenshot')}
              placeholder="{modifierKey}+Shift+S"
            />
            {#if recordingHotkey === 'hotkeyScreenshot'}
              <span class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-primary-400 animate-pulse">
                Запись...
              </span>
            {/if}
          </div>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-2">Вставить из буфера</label>
          <div class="relative">
            <input
              type="text"
              class="input font-mono pr-20 {recordingHotkey === 'hotkeyPaste' ? 'ring-2 ring-primary-500' : ''}"
              value={settings.hotkeyPaste}
              readonly
              on:focus={() => startRecording('hotkeyPaste')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyPaste')}
              placeholder="{modifierKey}+Shift+V"
            />
            {#if recordingHotkey === 'hotkeyPaste'}
              <span class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-primary-400 animate-pulse">
                Запись...
              </span>
            {/if}
          </div>
        </div>
      </div>
      <p class="text-xs text-gray-600 mt-4">
        💡 Подсказка: используйте {modifierKey} + Shift + буква для глобальных хоткеев
      </p>
    </div>

    <!-- Additional Settings -->
    <div class="card mb-6">
      <h3 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <span>🔧</span> Дополнительно
      </h3>
      <div class="space-y-4">
        <label class="flex items-center justify-between cursor-pointer">
          <div>
            <p class="font-medium text-gray-200">Звуковые уведомления</p>
            <p class="text-sm text-gray-500">Звук при получении текста и подключении</p>
          </div>
          <div class="relative">
            <input
              type="checkbox"
              class="sr-only peer"
              bind:checked={settings.soundEnabled}
            />
            <div class="w-11 h-6 bg-dark-600 rounded-full peer peer-checked:bg-primary-600
                        peer-focus:ring-2 peer-focus:ring-primary-500/50
                        after:content-[''] after:absolute after:top-0.5 after:left-0.5
                        after:bg-white after:rounded-full after:h-5 after:w-5
                        after:transition-all peer-checked:after:translate-x-5"></div>
          </div>
        </label>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex gap-4">
      <button
        class="btn btn-primary flex-1"
        on:click={saveSettings}
        disabled={saving}
      >
        {#if saving}
          ⏳ Сохранение...
        {:else if saved}
          ✓ Сохранено!
        {:else}
          💾 Сохранить
        {/if}
      </button>
      <button
        class="btn btn-secondary"
        on:click={resetToDefaults}
      >
        🔄 Сбросить
      </button>
    </div>
  </div>
</div>
