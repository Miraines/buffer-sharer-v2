<script lang="ts">
  import { fade, fly } from 'svelte/transition';
  import { flip } from 'svelte/animate';

  export let toasts: Array<{
    id: number;
    type: 'success' | 'error' | 'info' | 'warning';
    message: string;
  }> = [];

  const icons = {
    success: '✓',
    error: '✕',
    info: 'ℹ',
    warning: '⚠'
  };

  const colors = {
    success: 'bg-emerald-500/20 border-emerald-500/50 text-emerald-400',
    error: 'bg-red-500/20 border-red-500/50 text-red-400',
    info: 'bg-primary-500/20 border-primary-500/50 text-primary-400',
    warning: 'bg-amber-500/20 border-amber-500/50 text-amber-400'
  };

  export function addToast(type: 'success' | 'error' | 'info' | 'warning', message: string, duration = 3000) {
    const id = Date.now();
    toasts = [...toasts, { id, type, message }];

    if (duration > 0) {
      setTimeout(() => {
        removeToast(id);
      }, duration);
    }
  }

  function removeToast(id: number) {
    toasts = toasts.filter(t => t.id !== id);
  }
</script>

<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
  {#each toasts as toast (toast.id)}
    <div
      class="flex items-center gap-3 px-4 py-3 rounded-xl border backdrop-blur-sm shadow-lg {colors[toast.type]}"
      in:fly={{ x: 100, duration: 200 }}
      out:fade={{ duration: 150 }}
      animate:flip={{ duration: 200 }}
    >
      <span class="text-lg font-bold">{icons[toast.type]}</span>
      <span class="flex-1 text-sm">{toast.message}</span>
      <button
        class="opacity-50 hover:opacity-100 transition-opacity"
        on:click={() => removeToast(toast.id)}
      >
        ✕
      </button>
    </div>
  {/each}
</div>
