<script lang="ts">
  import { fade, fly } from 'svelte/transition';
  import { flip } from 'svelte/animate';

  export let toasts: Array<{
    id: number;
    type: 'success' | 'error' | 'info' | 'warning';
    message: string;
  }> = [];

  const icons = {
    success: 'check',
    error: 'x',
    info: 'info',
    warning: 'alert'
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

<div class="toast-container">
  {#each toasts as toast (toast.id)}
    <div
      class="toast toast-{toast.type}"
      in:fly={{ x: 100, duration: 200 }}
      out:fade={{ duration: 150 }}
      animate:flip={{ duration: 200 }}
    >
      <div class="toast-icon">
        {#if icons[toast.type] === 'check'}
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
        {:else if icons[toast.type] === 'x'}
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <line x1="18" y1="6" x2="6" y2="18"/>
            <line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        {:else if icons[toast.type] === 'info'}
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="16" x2="12" y2="12"/>
            <line x1="12" y1="8" x2="12.01" y2="8"/>
          </svg>
        {:else if icons[toast.type] === 'alert'}
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
            <line x1="12" y1="9" x2="12" y2="13"/>
            <line x1="12" y1="17" x2="12.01" y2="17"/>
          </svg>
        {/if}
      </div>
      <span class="toast-message copyable">{toast.message}</span>
      <button
        class="toast-close"
        on:click={() => removeToast(toast.id)}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: var(--space-6);
    right: var(--space-6);
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    max-width: 380px;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    background: var(--glass-bg);
    backdrop-filter: blur(var(--glass-blur)) saturate(180%);
    -webkit-backdrop-filter: blur(var(--glass-blur)) saturate(180%);
    border: 1px solid var(--glass-border);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-lg);
  }

  .toast-success {
    border-color: rgba(48, 209, 88, 0.4);
    background: rgba(48, 209, 88, 0.15);
  }

  .toast-error {
    border-color: rgba(255, 69, 58, 0.4);
    background: rgba(255, 69, 58, 0.15);
  }

  .toast-info {
    border-color: rgba(10, 132, 255, 0.4);
    background: rgba(10, 132, 255, 0.15);
  }

  .toast-warning {
    border-color: rgba(255, 159, 10, 0.4);
    background: rgba(255, 159, 10, 0.15);
  }

  .toast-icon {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .toast-success .toast-icon {
    color: var(--color-success);
  }

  .toast-error .toast-icon {
    color: var(--color-error);
  }

  .toast-info .toast-icon {
    color: var(--accent-primary);
  }

  .toast-warning .toast-icon {
    color: var(--color-warning);
  }

  .toast-message {
    flex: 1;
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--text-primary);
    line-height: var(--leading-normal);
  }

  .toast-close {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: var(--radius-md);
    color: var(--text-muted);
    cursor: pointer;
    opacity: 0.6;
    transition: all var(--duration-fast) var(--ease-out);
    flex-shrink: 0;
  }

  .toast-close:hover {
    opacity: 1;
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  /* Light theme adjustments */
  :global(html.light) .toast-success {
    background: rgba(52, 199, 89, 0.12);
    border-color: rgba(52, 199, 89, 0.3);
  }

  :global(html.light) .toast-error {
    background: rgba(255, 59, 48, 0.12);
    border-color: rgba(255, 59, 48, 0.3);
  }

  :global(html.light) .toast-info {
    background: rgba(0, 113, 227, 0.1);
    border-color: rgba(0, 113, 227, 0.3);
  }

  :global(html.light) .toast-warning {
    background: rgba(255, 159, 10, 0.12);
    border-color: rgba(255, 159, 10, 0.3);
  }
</style>
