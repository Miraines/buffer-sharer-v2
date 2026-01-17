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
    hotkeyInvisibility: string;
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
      if (field === 'hotkeyInvisibility') settings.hotkeyInvisibility = hotkey;

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
        hotkeyInvisibility: settings.hotkeyInvisibility,
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
      hotkeyInvisibility: isMac ? 'Cmd+Option+I' : 'Ctrl+Alt+I',
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

<div class="panel">
  <div class="panel-content">
    <div class="panel-header">
      <h1 class="panel-title">Настройки</h1>
      <p class="panel-subtitle">Конфигурация приложения</p>
    </div>

    <!-- Server Settings -->
    <div class="card">
      <div class="card-header">
        <div class="card-icon">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <line x1="2" y1="12" x2="22" y2="12"/>
            <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
          </svg>
        </div>
        <h2 class="card-title">Сервер</h2>
      </div>
      <div class="form-grid">
        <div class="form-field">
          <label class="form-label" for="middleware-host">Хост Middleware</label>
          <input
            id="middleware-host"
            type="text"
            class="input"
            bind:value={settings.middlewareHost}
            placeholder="localhost"
          />
        </div>
        <div class="form-field">
          <label class="form-label" for="middleware-port">Порт</label>
          <input
            id="middleware-port"
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
    <div class="card">
      <div class="card-header">
        <div class="card-icon">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
            <circle cx="8.5" cy="8.5" r="1.5"/>
            <polyline points="21 15 16 10 5 21"/>
          </svg>
        </div>
        <h2 class="card-title">Скриншоты</h2>
      </div>
      <div class="form-stack">
        <div class="form-field">
          <div class="slider-header">
            <label class="form-label" for="screenshot-interval">Интервал захвата</label>
            <span class="slider-value">{settings.screenshotInterval} мс</span>
          </div>
          <input
            id="screenshot-interval"
            type="range"
            bind:value={settings.screenshotInterval}
            min="1000"
            max="10000"
            step="500"
          />
          <div class="slider-labels">
            <span>1 сек</span>
            <span>10 сек</span>
          </div>
        </div>
        <div class="form-field">
          <div class="slider-header">
            <label class="form-label" for="screenshot-quality">Качество JPEG</label>
            <span class="slider-value">{settings.screenshotQuality}%</span>
          </div>
          <input
            id="screenshot-quality"
            type="range"
            bind:value={settings.screenshotQuality}
            min="10"
            max="100"
            step="5"
          />
          <div class="slider-labels">
            <span>Меньше размер</span>
            <span>Выше качество</span>
          </div>
        </div>
        <div class="form-field">
          <label class="form-label" for="screenshot-dir">Папка для сохранения</label>
          <div class="input-group">
            <input
              id="screenshot-dir"
              type="text"
              class="input"
              bind:value={settings.screenshotSaveDir}
              placeholder="~/Downloads (по умолчанию)"
              readonly
            />
            <button
              class="btn btn-secondary btn-icon"
              on:click={selectSaveDirectory}
              disabled={selectingDir}
            >
              {#if selectingDir}
                <div class="spinner-sm"></div>
              {:else}
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                </svg>
              {/if}
            </button>
          </div>
          <p class="form-hint">Куда сохранять скриншоты по кнопке "Сохранить"</p>
        </div>
        <div class="form-field">
          <div class="slider-header">
            <label class="form-label" for="screenshot-history-limit">Лимит истории скриншотов</label>
            <span class="slider-value">{settings.screenshotHistoryLimit || 50} шт.</span>
          </div>
          <input
            id="screenshot-history-limit"
            type="range"
            bind:value={settings.screenshotHistoryLimit}
            min="10"
            max="200"
            step="10"
          />
          <div class="slider-labels">
            <span>10</span>
            <span>200</span>
          </div>
          <p class="form-hint">Старые скриншоты автоматически удаляются при превышении лимита</p>
        </div>
      </div>
    </div>

    <!-- Clipboard Settings -->
    <div class="card">
      <div class="card-header">
        <div class="card-icon">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>
            <rect x="8" y="2" width="8" height="4" rx="1" ry="1"/>
          </svg>
        </div>
        <h2 class="card-title">Буфер обмена</h2>
      </div>
      <div class="toggle-row">
        <div class="toggle-info">
          <span class="toggle-label" id="clipboard-sync-label">Синхронизация буфера обмена</span>
          <span class="toggle-hint">Автоматически отправлять содержимое буфера</span>
        </div>
        <button
          type="button"
          class="toggle {settings.clipboardSync ? 'active' : ''}"
          role="switch"
          aria-checked={settings.clipboardSync}
          aria-labelledby="clipboard-sync-label"
          on:click={() => settings.clipboardSync = !settings.clipboardSync}
        ></button>
      </div>
    </div>

    <!-- Theme Settings -->
    <div class="card">
      <div class="card-header">
        <div class="card-icon">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="13.5" cy="6.5" r=".5"/>
            <circle cx="17.5" cy="10.5" r=".5"/>
            <circle cx="8.5" cy="7.5" r=".5"/>
            <circle cx="6.5" cy="12.5" r=".5"/>
            <path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.926 0 1.648-.746 1.648-1.688 0-.437-.18-.835-.437-1.125-.29-.289-.438-.652-.438-1.125a1.64 1.64 0 0 1 1.668-1.668h1.996c3.051 0 5.555-2.503 5.555-5.555C21.965 6.012 17.461 2 12 2z"/>
          </svg>
        </div>
        <h2 class="card-title">Тема оформления</h2>
      </div>
      <div class="theme-grid">
        <button
          class="theme-option {settings.theme === 'dark' ? 'active' : ''}"
          on:click={() => settings.theme = 'dark'}
        >
          <div class="theme-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
          </div>
          <span class="theme-name">Тёмная</span>
        </button>

        <button
          class="theme-option {settings.theme === 'light' ? 'active' : ''}"
          on:click={() => settings.theme = 'light'}
        >
          <div class="theme-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="5"/>
              <line x1="12" y1="1" x2="12" y2="3"/>
              <line x1="12" y1="21" x2="12" y2="23"/>
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
              <line x1="1" y1="12" x2="3" y2="12"/>
              <line x1="21" y1="12" x2="23" y2="12"/>
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
          </div>
          <span class="theme-name">Светлая</span>
        </button>

        <button
          class="theme-option {settings.theme === 'system' ? 'active' : ''}"
          on:click={() => settings.theme = 'system'}
        >
          <div class="theme-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
              <line x1="8" y1="21" x2="16" y2="21"/>
              <line x1="12" y1="17" x2="12" y2="21"/>
            </svg>
          </div>
          <span class="theme-name">Системная</span>
        </button>
      </div>
    </div>

    <!-- Hotkeys Settings -->
    <div class="card">
      <div class="card-header">
        <div class="card-icon">
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
        </div>
        <div>
          <h2 class="card-title">Горячие клавиши</h2>
          <span class="platform-badge">{isMac ? 'macOS' : 'Windows/Linux'}</span>
        </div>
      </div>
      <p class="card-subtitle">Нажмите на поле и введите желаемую комбинацию клавиш</p>
      <div class="form-stack">
        <div class="form-field">
          <label class="form-label" for="hotkey-toggle">Переключить режим ввода</label>
          <div class="hotkey-input-wrapper">
            <input
              id="hotkey-toggle"
              type="text"
              class="input input-mono {recordingHotkey === 'hotkeyToggle' ? 'recording' : ''}"
              value={settings.hotkeyToggle}
              readonly
              on:focus={() => startRecording('hotkeyToggle')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyToggle')}
              placeholder="{modifierKey}+Shift+J"
            />
            {#if recordingHotkey === 'hotkeyToggle'}
              <span class="recording-badge">Запись...</span>
            {/if}
          </div>
        </div>
        <div class="form-field">
          <label class="form-label" for="hotkey-screenshot">Сделать скриншот</label>
          <div class="hotkey-input-wrapper">
            <input
              id="hotkey-screenshot"
              type="text"
              class="input input-mono {recordingHotkey === 'hotkeyScreenshot' ? 'recording' : ''}"
              value={settings.hotkeyScreenshot}
              readonly
              on:focus={() => startRecording('hotkeyScreenshot')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyScreenshot')}
              placeholder="{modifierKey}+Shift+S"
            />
            {#if recordingHotkey === 'hotkeyScreenshot'}
              <span class="recording-badge">Запись...</span>
            {/if}
          </div>
        </div>
        <div class="form-field">
          <label class="form-label" for="hotkey-paste">Вставить из буфера</label>
          <div class="hotkey-input-wrapper">
            <input
              id="hotkey-paste"
              type="text"
              class="input input-mono {recordingHotkey === 'hotkeyPaste' ? 'recording' : ''}"
              value={settings.hotkeyPaste}
              readonly
              on:focus={() => startRecording('hotkeyPaste')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyPaste')}
              placeholder="{modifierKey}+Shift+V"
            />
            {#if recordingHotkey === 'hotkeyPaste'}
              <span class="recording-badge">Запись...</span>
            {/if}
          </div>
        </div>
        <div class="form-field">
          <label class="form-label" for="hotkey-invisibility">
            Режим невидимости
            <span class="hotkey-badge">скрыть от screen share</span>
          </label>
          <div class="hotkey-input-wrapper">
            <input
              id="hotkey-invisibility"
              type="text"
              class="input input-mono {recordingHotkey === 'hotkeyInvisibility' ? 'recording' : ''}"
              value={settings.hotkeyInvisibility}
              readonly
              on:focus={() => startRecording('hotkeyInvisibility')}
              on:blur={stopRecording}
              on:keydown={(e) => handleKeyDown(e, 'hotkeyInvisibility')}
              placeholder="{modifierKey}+Shift+I"
            />
            {#if recordingHotkey === 'hotkeyInvisibility'}
              <span class="recording-badge">Запись...</span>
            {/if}
          </div>
        </div>
      </div>
      <p class="form-hint" style="margin-top: var(--space-4);">
        Подсказка: используйте {modifierKey} + Shift + буква для глобальных хоткеев
      </p>
    </div>

    <!-- Additional Settings -->
    <div class="card">
      <div class="card-header">
        <div class="card-icon">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="3"/>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
        </div>
        <h2 class="card-title">Дополнительно</h2>
      </div>
      <div class="toggle-row">
        <div class="toggle-info">
          <span class="toggle-label" id="sound-enabled-label">Звуковые уведомления</span>
          <span class="toggle-hint">Звук при получении текста и подключении</span>
        </div>
        <button
          type="button"
          class="toggle {settings.soundEnabled ? 'active' : ''}"
          role="switch"
          aria-checked={settings.soundEnabled}
          aria-labelledby="sound-enabled-label"
          on:click={() => settings.soundEnabled = !settings.soundEnabled}
        ></button>
      </div>
    </div>

    <!-- Actions -->
    <div class="action-buttons">
      <button
        class="btn {saved ? 'btn-success' : 'btn-primary'} action-btn-main"
        on:click={saveSettings}
        disabled={saving}
      >
        {#if saving}
          <div class="spinner-sm"></div>
          <span>Сохранение...</span>
        {:else if saved}
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          <span>Сохранено!</span>
        {:else}
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
            <polyline points="17 21 17 13 7 13 7 21"/>
            <polyline points="7 3 7 8 15 8"/>
          </svg>
          <span>Сохранить</span>
        {/if}
      </button>
      <button
        class="btn btn-secondary"
        on:click={resetToDefaults}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="1 4 1 10 7 10"/>
          <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/>
        </svg>
        <span>Сбросить</span>
      </button>
    </div>
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

  /* Card Header */
  .card-header {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-bottom: var(--space-5);
  }

  .card-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-primary-muted);
    border-radius: var(--radius-lg);
    color: var(--accent-primary);
  }

  .card-title {
    font-size: var(--text-lg);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .card-subtitle {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
    margin: 0 0 var(--space-4) 0;
  }

  .platform-badge {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  /* Form Styles */
  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }

  .form-stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .form-field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .form-label {
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--text-secondary);
  }

  .form-hint {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: 0;
  }

  /* Slider Styles */
  .slider-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .slider-value {
    font-size: var(--text-sm);
    color: var(--accent-primary);
    font-weight: 500;
  }

  .slider-labels {
    display: flex;
    justify-content: space-between;
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin-top: var(--space-1);
  }

  /* Input Group */
  .input-group {
    display: flex;
    gap: var(--space-2);
  }

  .input-group .input {
    flex: 1;
  }

  /* Input Mono */
  .input-mono {
    font-family: var(--font-mono);
  }

  .input.recording {
    border-color: var(--accent-primary);
    box-shadow: 0 0 0 3px var(--accent-primary-muted);
  }

  /* Hotkey Input */
  .hotkey-input-wrapper {
    position: relative;
  }

  .recording-badge {
    position: absolute;
    right: var(--space-3);
    top: 50%;
    transform: translateY(-50%);
    font-size: var(--text-xs);
    color: var(--accent-primary);
    animation: pulse 1s infinite;
  }

  .hotkey-badge {
    font-size: var(--text-xs);
    font-weight: 400;
    color: var(--accent-success);
    background: var(--accent-success-muted);
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    margin-left: var(--space-2);
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  /* Toggle Row */
  .toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
  }

  .toggle-info {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .toggle-label {
    font-size: var(--text-base);
    font-weight: 500;
    color: var(--text-primary);
  }

  .toggle-hint {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  .toggle {
    position: relative;
    width: 44px;
    height: 24px;
    background: var(--bg-active);
    border: none;
    border-radius: var(--radius-full);
    cursor: pointer;
    transition: background var(--duration-normal) var(--ease-out);
    flex-shrink: 0;
    padding: 0;
  }

  .toggle::after {
    content: '';
    position: absolute;
    top: 2px;
    left: 2px;
    width: 20px;
    height: 20px;
    background: white;
    border-radius: var(--radius-full);
    box-shadow: var(--shadow-sm);
    transition: transform var(--duration-normal) var(--ease-spring);
  }

  .toggle.active {
    background: var(--accent-primary);
  }

  .toggle.active::after {
    transform: translateX(20px);
  }

  /* Theme Grid */
  .theme-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--space-3);
  }

  .theme-option {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-4);
    background: var(--bg-tertiary);
    border: 2px solid var(--border-primary);
    border-radius: var(--radius-xl);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .theme-option:hover {
    border-color: var(--border-hover);
  }

  .theme-option.active {
    border-color: var(--accent-primary);
    background: var(--accent-primary-muted);
  }

  .theme-icon {
    color: var(--text-secondary);
  }

  .theme-option.active .theme-icon {
    color: var(--accent-primary);
  }

  .theme-name {
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--text-primary);
  }

  /* Action Buttons */
  .action-buttons {
    display: flex;
    gap: var(--space-4);
    margin-top: var(--space-2);
  }

  .action-btn-main {
    flex: 1;
    padding: var(--space-4);
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
