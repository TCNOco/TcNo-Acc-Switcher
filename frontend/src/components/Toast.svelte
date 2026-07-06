<script lang="ts">
  import { onMount } from 'svelte'
  import { fly } from 'svelte/transition'
  import { cubicOut } from 'svelte/easing'
  import { Events } from '@wailsio/runtime'
  import { get } from 'svelte/store'
  import { dismissToastById, pushToast, toastStore } from '../stores/toast'
  import { t as translate } from '../stores/i18n'
  import { controllerSpatialNavigation } from '../lib/actions/controllerSpatialNavigation'
  import ToastTypeIcon from './ToastTypeIcon.svelte'
  import "../styles/toast.scss";

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

  function translateI18nToken(value: string): string {
    if (!value.startsWith('i18n:')) return value
    return get(translate)(value.slice(5))
  }

  function fromWailsPayload(data: unknown): void {
    if (!data || typeof data !== 'object') return
    const p = data as WailsToastPayload
    if (typeof p.message !== 'string' || typeof p.type !== 'string') return
    pushToast({
      type: p.type,
      title: typeof p.title === 'string' ? translateI18nToken(p.title) : '',
      message: translateI18nToken(p.message),
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
  <div class="toast-stack" use:controllerSpatialNavigation>
    {#each $toastStore as t (t.id)}
      <div
        class="toast {typeClass(t.type)}"
        in:fly={{ y: -14, duration: 240, opacity: 0, easing: cubicOut }}
        out:fly={{ y: -10, duration: 200, opacity: 0, easing: cubicOut }}
        role="status"
      >
        {#if t.count > 1}
          <span class="toast__count" aria-label={$translate("Aria_RepeatedTimes", { count: t.count })}>{t.count}</span>
        {/if}
        <button
          type="button"
          class="toast__close"
          aria-label={$translate("Aria_DismissNotification")}
          on:click|stopPropagation={() => dismissToastById(t.id)}
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
