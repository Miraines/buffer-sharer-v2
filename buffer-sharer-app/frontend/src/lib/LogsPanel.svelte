<script lang="ts">
  export let logs: Array<{time: string, level: string, message: string}>;

  function getLevelColor(level: string): string {
    switch (level) {
      case 'error': return 'text-red-400';
      case 'warn': return 'text-yellow-400';
      case 'info': return 'text-blue-400';
      case 'debug': return 'text-gray-500';
      default: return 'text-gray-400';
    }
  }

  function getLevelBg(level: string): string {
    switch (level) {
      case 'error': return 'bg-red-500/10';
      case 'warn': return 'bg-yellow-500/10';
      case 'info': return 'bg-blue-500/10';
      default: return 'bg-dark-800';
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

<div class="h-full p-8 flex flex-col">
  <div class="flex items-center justify-between mb-6">
    <div>
      <h2 class="text-2xl font-bold text-white">Логи</h2>
      <p class="text-gray-400">История действий приложения</p>
    </div>
    <div class="flex gap-2">
      <button class="btn btn-secondary text-sm" on:click={exportLogs}>
        📥 Экспорт
      </button>
      <button class="btn btn-secondary text-sm" on:click={clearLogs}>
        🗑️ Очистить
      </button>
    </div>
  </div>

  <div class="flex-1 card overflow-hidden flex flex-col">
    {#if logs.length === 0}
      <div class="flex-1 flex flex-col items-center justify-center py-12">
        <span class="text-5xl mb-4 opacity-50">📋</span>
        <p class="text-gray-500">Логи пусты</p>
      </div>
    {:else}
      <div class="flex-1 overflow-auto font-mono text-sm space-y-1 p-2">
        {#each logs as log, i (i)}
          <div class="flex items-start gap-3 px-3 py-2 rounded-lg {getLevelBg(log.level)} hover:bg-dark-700/50 transition-colors">
            <span class="text-gray-600 shrink-0">{log.time}</span>
            <span class="uppercase text-xs font-bold w-12 shrink-0 {getLevelColor(log.level)}">
              {log.level}
            </span>
            <span class="text-gray-300 break-all">{log.message}</span>
          </div>
        {/each}
      </div>
    {/if}

    <!-- Stats -->
    <div class="border-t border-dark-700 px-4 py-3 flex items-center justify-between text-sm text-gray-500">
      <span>{logs.length} записей</span>
      <div class="flex gap-4">
        <span class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-red-500"></span>
          {logs.filter(l => l.level === 'error').length}
        </span>
        <span class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-yellow-500"></span>
          {logs.filter(l => l.level === 'warn').length}
        </span>
        <span class="flex items-center gap-1">
          <span class="w-2 h-2 rounded-full bg-blue-500"></span>
          {logs.filter(l => l.level === 'info').length}
        </span>
      </div>
    </div>
  </div>
</div>
