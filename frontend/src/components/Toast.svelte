<script lang="ts">
  import { onMount } from 'svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import { Events } from '@wailsio/runtime'
  import { dismissToastById, pushToast, toastStore } from '../stores/toast'
  import ToastTypeIcon from './ToastTypeIcon.svelte'

  type WailsToastPayload = {
    type: string
    title?: string
    message: string
    duration?: number
  }

  type WindowToastOpts = {
    type: string
    title?: string
    message: string
    renderTo?: string
    duration?: number
  }

  function fromWailsPayload(data: unknown): void {
    if (!data || typeof data !== 'object') return
    const p = data as WailsToastPayload
    if (typeof p.message !== 'string' || typeof p.type !== 'string') return
    pushToast({
      type: p.type,
      title: typeof p.title === 'string' ? p.title : '',
      message: p.message,
      duration: typeof p.duration === 'number' ? p.duration : undefined,
    })
  }

  onMount(() => {
    const offToast = Events.On('toast', (ev) => {
      fromWailsPayload(ev.data)
    })

    window.notification = {
      new: (opts: WindowToastOpts): void => {
        if (!opts || typeof opts.message !== 'string' || typeof opts.type !== 'string') {
          return
        }
        pushToast({
          type: opts.type,
          title: typeof opts.title === 'string' ? opts.title : '',
          message: opts.message,
          duration: typeof opts.duration === 'number' ? opts.duration : undefined,
        })
      },
    }

    return () => {
      offToast()
      delete window.notification
    }
  })

  function typeClass(type: string): string {
    const t = type.toLowerCase()
    if (t === 'success' || t === 'warning' || t === 'error' || t === 'info') {
      return `toast--${t}`
    }
    return 'toast--info'
  }
</script>

<!-- Type icons: edit `ToastTypeIcon.svelte` (per-type {#if} branches + HTML comment there). -->
<div class="toast-root" aria-live="polite" aria-relevant="additions text">
  <div class="toast-stack">
    {#each $toastStore as t (t.id)}
      <div
        class="toast {typeClass(t.type)}"
        in:fly={{ y: -14, duration: 240, opacity: 0, easing: cubicOut }}
        out:fly={{ y: -10, duration: 200, opacity: 0, easing: cubicOut }}
        role="status"
      >
        {#if t.count > 1}
          <span class="toast__count" aria-label={`Repeated ${t.count} times`}>{t.count}</span>
        {/if}
        <button
          type="button"
          class="toast__close"
          aria-label="Dismiss notification"
          on:click={() => dismissToastById(t.id)}
        >
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
            <path
              fill="currentColor"
              d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
            />
          </svg>
        </button>
        <div class="toast__row">
          <div class="toast__icon" aria-hidden="true">
            <ToastTypeIcon type={t.type} />
          </div>
          <div class="toast__text">
            {#if t.title}
              <div class="toast__title">{t.title}</div>
            {/if}
            <div class="toast__message">{t.message}</div>
          </div>
        </div>
      </div>
    {/each}
  </div>
</div>

<style lang="scss">
  .toast-root {
    position: absolute;
    inset: 0;
    z-index: 1000;
    pointer-events: none;
    display: flex;
    justify-content: flex-end;
    align-items: stretch;
    padding: var(--main-padding, 0.75rem);
    box-sizing: border-box;
  }

  .toast-stack {
    display: flex;
    flex-direction: column;
    justify-content: flex-start;
    align-items: flex-end;
    gap: 0.5rem;
    max-width: 22rem;
    width: 100%;
    min-height: 0;
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    position: relative;
    width: 100%;
    padding: 0.65rem 2.25rem 0.65rem 0.65rem;

    border-radius: 2px;
    border: 1px solid transparent;
    box-shadow: var(--shadow, 0 4px 14px rgba(0, 0, 0, 0.35));
    background: var(--darker-code-background, #0f151a);
    color: var(--white, #f8f8f2);
    text-align: left;

    user-select: text;
    -webkit-user-select: text;
    -moz-user-select: text;
    -ms-user-select: text;
  }

  .toast--success {
    border-color: var(--green, #8aff80);
    box-shadow:
      var(--shadow, 0 4px 14px rgba(0, 0, 0, 0.35)),
      0 0 0 1px rgba(138, 255, 128, 0.12);

    .toast__icon, .toast__count, svg {
      color: var(--green, #8aff80);
      fill: var(--green, #8aff80);
    }
  }

  .toast--warning {
    border-color: var(--orange, #ffca80);
    box-shadow:
      var(--shadow, 0 4px 14px rgba(0, 0, 0, 0.35)),
      0 0 0 1px rgba(255, 202, 128, 0.12);

    .toast__icon, .toast__count, svg {
      color: var(--orange, #ffca80);
      fill: var(--orange, #ffca80);
    }
  }

  .toast--error {
    border-color: var(--red, #ff9580);
    box-shadow:
      var(--shadow, 0 4px 14px rgba(0, 0, 0, 0.35)),
      0 0 0 1px rgba(255, 149, 128, 0.12);

    .toast__icon, .toast__count, svg {
      color: var(--red, #ff9580);
      fill: var(--red, #ff9580);
    }
  }

  .toast--info {
    border-color: var(--cyan, #80ffea);
    box-shadow:
      var(--shadow, 0 4px 14px rgba(0, 0, 0, 0.35)),
      0 0 0 1px rgba(128, 255, 234, 0.12);

    .toast__icon, .toast__count, svg {
      color: var(--cyan, #80ffea);
      fill: var(--cyan, #80ffea);
    }
  }

  .toast__row {
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }

  .toast__icon {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    justify-content: center;
    align-self: center;
  }

  .toast__text {
    flex: 1;
    min-width: 0;
  }

  .toast__count {
    position: absolute;
    left: -1rem;
    top: -1rem;
    min-width: 2rem;
    height: 2rem;
    padding: 0.5rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-weight: 700;
    line-height: 1;
    border-radius: 999px;
    background: var(--button-bg, #274560);
    color: var(--whiteSecondary, #fff);
    border: 1px solid var(--blackSecondary, #414558);
    z-index: 1;
  }

  .toast__close {
    position: absolute;
    right: 0.2rem;
    top: 0.2rem;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    margin: 0;
    padding: 0;
    border: none;
    border-radius: 2px;
    background: transparent;
    cursor: pointer;
    transition:
      color 80ms ease,
      background 80ms ease;

    &:hover {
      color: var(--white, #f8f8f2);
      background: rgba(255, 255, 255, 0.06);
    }
  }

  .toast__title {
    font-weight: 600;
    font-size: 1.1rem;
    margin-bottom: 0.2rem;
    padding-right: 0.25rem;
  }

  .toast__message {
    font-size: 1rem;
    line-height: 1.35;
    color: var(--blackTernary, #a7abbe);
    word-break: break-word;
  }

  .toast--success .toast__message,
  .toast--warning .toast__message,
  .toast--error .toast__message,
  .toast--info .toast__message {
    color: var(--whiteSecondary, #fff);
    opacity: 0.92;
  }
</style>
