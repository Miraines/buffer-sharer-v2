<script lang="ts">
  export let logs: Array<{time: string, level: string, message: string}>;

  function getLevelColor(level: string): string {
    switch (level) {
      case 'error': return 'error';
      case 'warn': return 'warning';
      case 'info': return 'info';
      case 'debug': return 'muted';
      default: return 'muted';
    }
  }

  function clearLogs() {
    logs = [];
  }

  function exportLogs() {
    const content = logs.map(l => `[${l.time}] [${l.level.toUpperCase()}] ${l.message}`).join('\n');
    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `buffer-sharer-logs-${new Date().toISOString().slice(0, 10)}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  }
</script>

<div class="panel">
  <div class="panel-content">
    <div class="panel-header">
      <div>
        <h1 class="panel-title">Логи</h1>
        <p class="panel-subtitle">История действий приложения</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-secondary btn-sm" on:click={exportLogs}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="7 10 12 15 17 10"/>
            <line x1="12" y1="15" x2="12" y2="3"/>
          </svg>
          <span>Экспорт</span>
        </button>
        <button class="btn btn-secondary btn-sm" on:click={clearLogs}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="3 6 5 6 21 6"/>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
          </svg>
          <span>Очистить</span>
        </button>
      </div>
    </div>

    <div class="card logs-card">
      {#if logs.length === 0}
        <div class="empty-state">
          <div class="empty-icon">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
              <polyline points="14 2 14 8 20 8"/>
              <line x1="16" y1="13" x2="8" y2="13"/>
              <line x1="16" y1="17" x2="8" y2="17"/>
              <polyline points="10 9 9 9 8 9"/>
            </svg>
          </div>
          <p class="empty-title">Логи пусты</p>
        </div>
      {:else}
        <div class="logs-list">
          {#each logs as log, i (i)}
            <div class="log-item level-{getLevelColor(log.level)}">
              <span class="log-time copyable">{log.time}</span>
              <span class="log-level">{log.level.toUpperCase()}</span>
              <span class="log-message copyable">{log.message}</span>
            </div>
          {/each}
        </div>

        <!-- Stats Footer -->
        <div class="logs-footer">
          <span class="logs-count">{logs.length} записей</span>
          <div class="logs-stats">
            <span class="stat-item error">
              <span class="stat-dot"></span>
              {logs.filter(l => l.level === 'error').length}
            </span>
            <span class="stat-item warning">
              <span class="stat-dot"></span>
              {logs.filter(l => l.level === 'warn').length}
            </span>
            <span class="stat-item info">
              <span class="stat-dot"></span>
              {logs.filter(l => l.level === 'info').length}
            </span>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .panel {
    height: 100%;
    padding: var(--space-8);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .panel-content {
    max-width: 900px;
    margin: 0 auto;
    width: 100%;
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: var(--space-6);
    flex-shrink: 0;
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

  .header-actions {
    display: flex;
    gap: var(--space-2);
  }

  .btn-sm {
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-xs);
  }

  /* Logs Card */
  .logs-card {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 0;
    min-height: 0;
  }

  /* Empty State */
  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: var(--space-12);
  }

  .empty-icon {
    width: 64px;
    height: 64px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-tertiary);
    border-radius: var(--radius-xl);
    color: var(--text-muted);
    margin-bottom: var(--space-4);
  }

  .empty-title {
    font-size: var(--text-base);
    color: var(--text-tertiary);
    margin: 0;
  }

  /* Logs List */
  .logs-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-height: 0;
  }

  .log-item {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    transition: background var(--duration-fast) var(--ease-out);
  }

  .log-item:hover {
    background: var(--bg-hover);
  }

  .log-item.level-error {
    background: var(--color-error-muted);
  }

  .log-item.level-warning {
    background: var(--color-warning-muted);
  }

  .log-item.level-info {
    background: var(--color-info-muted);
  }

  .log-time {
    color: var(--text-muted);
    flex-shrink: 0;
    width: 80px;
  }

  .log-level {
    text-transform: uppercase;
    font-size: var(--text-xs);
    font-weight: 600;
    width: 48px;
    flex-shrink: 0;
  }

  .level-error .log-level {
    color: var(--color-error);
  }

  .level-warning .log-level {
    color: var(--color-warning);
  }

  .level-info .log-level {
    color: var(--color-info);
  }

  .level-muted .log-level {
    color: var(--text-muted);
  }

  .log-message {
    color: var(--text-secondary);
    word-break: break-all;
    flex: 1;
  }

  /* Footer */
  .logs-footer {
    padding: var(--space-4);
    border-top: 1px solid var(--border-primary);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .logs-count {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  .logs-stats {
    display: flex;
    gap: var(--space-4);
  }

  .stat-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }

  .stat-dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
  }

  .stat-item.error .stat-dot {
    background: var(--color-error);
  }

  .stat-item.warning .stat-dot {
    background: var(--color-warning);
  }

  .stat-item.info .stat-dot {
    background: var(--color-info);
  }
</style>
