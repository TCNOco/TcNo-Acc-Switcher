<script lang="ts">
    import { get } from 'svelte/store'
    import { route, previousPage } from '../stores/nav'
    import { onMount } from 'svelte'
    import { appBarTitle } from '../stores/nav'
    import { activeModal } from '../stores/modal'
    import { Events, Window } from "@wailsio/runtime";

    let minimised = false
    let maximised = false

    const common = Events.Types.Common

    async function refreshWindowState(reason: string) {
        const [min, max] = await Promise.all([
            Window.IsMinimised(),
            Window.IsMaximised(),
        ])
        minimised = min
        maximised = max
        console.log('[TitleBar] synced', { reason, minimised, maximised })
    }

    onMount(() => {
        void refreshWindowState('onMount')

        const tracked = [
            common.WindowMinimise,
            common.WindowUnMinimise,
            common.WindowMaximise,
            common.WindowUnMaximise,
            common.WindowRestore,
            common.WindowFullscreen,
            common.WindowUnFullscreen,
        ] as const

        const unsubs = tracked.map((subscribedAs) =>
            Events.On(subscribedAs, (ev) => {
                console.log('[TitleBar] event', {
                    subscribedAs,
                    name: ev.name,
                    sender: ev.sender,
                    data: ev.data,
                })
                void refreshWindowState(String(ev.name))
            })
        )

        return () => {
            for (const off of unsubs) off()
        }
    })

    const possibleAnimations = ['X', 'Y', 'Z']
    let backSpin = ''
    let backTransition = ''
    $: backDisabled = !!$activeModal

    function backClick() {
        if ($route.page === 'home') {
            const axis = possibleAnimations[Math.floor(Math.random() * possibleAnimations.length)]
            backSpin = `rotate${axis}(360deg)`
            backTransition = 'transform 500ms ease-in-out'
            setTimeout(() => {
                backSpin = ''
                backTransition = 'transform 0ms ease-in-out'
            }, 500)
        } else {
            const prev = get(previousPage)
            if (prev) {
                route.set(prev)
            } else {
                route.set({ page: 'home' })
            }
        }
    }
</script>

<header class="headerbar">
    <span class="title-left">
        <button
            type="button"
            class="win-btn win-btn-back"
            title="Back"
            disabled={backDisabled}
            aria-disabled={backDisabled}
            on:click={backClick}
        >
            <svg 
            style:transform={backSpin}
            style:transition={backTransition}
            style:transform-origin="center"
            xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 512"><!--!Font Awesome Free v5.15.4 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2026 Fonticons, Inc.--><path d="M34.52 239.03L228.87 44.69c9.37-9.37 24.57-9.37 33.94 0l22.67 22.67c9.36 9.36 9.37 24.52.04 33.9L131.49 256l154.02 154.75c9.34 9.38 9.32 24.54-.04 33.9l-22.67 22.67c-9.37 9.37-24.57 9.37-33.94 0L34.52 272.97c-9.37-9.37-9.37-24.57 0-33.94z"/></svg>
        </button>
        <svg class="header_icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 768 264" fill-rule="evenodd" stroke-linejoin="round" stroke-miterlimit="2">
            <use href="img/TcNo_Logo_Flat.svg#logo"></use>
        </svg>
    </span>
    <span id="title-label" class="title-drag">{$appBarTitle}</span>
    <span class="window-controls" role="toolbar">
        <button type="button" class="win-btn win-btn-min" aria-label="Minimize" on:click={backClick}>
            <img class="icon" srcset="img/icons/min-w-10.webp 1x, img/icons/min-w-12.webp 1.25x, img/icons/min-w-15.webp 1.5x, img/icons/min-w-15.webp 1.75x, img/icons/min-w-20.webp 2x, img/icons/min-w-20.webp 2.25x, img/icons/min-w-24.webp 2.5x, img/icons/min-w-30.webp 3x, img/icons/min-w-30.webp 3.5x" draggable="false" alt="-">
        </button>
        {#if maximised}
            <button type="button" class="win-btn win-btn-max" aria-label="Restore" on:click={() => Window.Restore()}>
                <img class="icon" srcset="img/icons/restore-w-10.webp 1x, img/icons/restore-w-12.webp 1.25x, img/icons/restore-w-15.webp 1.5x, img/icons/restore-w-15.webp 1.75x, img/icons/restore-w-20.webp 2x, img/icons/restore-w-20.webp 2.25x, img/icons/restore-w-24.webp 2.5x, img/icons/restore-w-30.webp 3x, img/icons/restore-w-30.webp 3.5x" draggable="false" alt="R">
            </button>
        {:else}
            <button type="button" class="win-btn win-btn-max" aria-label="Restore" on:click={() => Window.Maximise()}>
                <img class="icon" srcset="img/icons/max-w-10.webp 1x, img/icons/max-w-12.webp 1.25x, img/icons/max-w-15.webp 1.5x, img/icons/max-w-15.webp 1.75x, img/icons/max-w-20.webp 2x, img/icons/max-w-20.webp 2.25x, img/icons/max-w-24.webp 2.5x, img/icons/max-w-30.webp 3x, img/icons/max-w-30.webp 3.5x" draggable="false" alt="M">
            </button>
        {/if}
        <button type="button" class="win-btn win-btn-close" aria-label="Close" on:click={() => Window.Close()}>
            <img class="icon" srcset="img/icons/close-w-10.webp 1x, img/icons/close-w-12.webp 1.25x, img/icons/close-w-15.webp 1.5x, img/icons/close-w-15.webp 1.75x, img/icons/close-w-20.webp 2x, img/icons/close-w-20.webp 2.25x, img/icons/close-w-24.webp 2.5x, img/icons/close-w-30.webp 3x, img/icons/close-w-30.webp 3.5x" draggable="false" alt="X">
        </button>
    </span>
</header>

<style lang="scss">
    .headerbar {
        --webkit-app-region: drag;
        --wails-draggable: drag;
        -moz-user-select: none;
        -ms-user-select: none;
        -webkit-user-select: none;
        user-select: none;
        z-index: 5;
        background: var(--border-bar-bg);
        position: relative;
        height: 32px;
        min-height: 32px;
        width: 100%;
        -webkit-app-region: drag;
        color: #fff;
        grid-column: 1;
        display: flex;
        justify-content: space-between;
        align-items: center;
        overflow: hidden;
        font-family: "Segoe UI", sans-serif;
        font-size: 12px;
        font-weight: 500;
    }
    .title-left {
        height: 100%;
        display: flex;
        flex-direction: row;
    }
    .win-btn-back {
        --wails-draggable: no-drag;
        svg {
            fill: white;
            height: 0.8rem;
            display: block;
            fill: white;
        }
        &:disabled {
            opacity: 0.35;
            cursor: not-allowed;
        }
    }
    .header_icon {
        height: 10px;
        margin: 12px;
        display: block;
        fill: white;
    }
    .window-controls {
        --wails-draggable: no-drag;
        display: grid;
        grid-template-columns: repeat(3, 46px);
        top: 0;
        right: 0;
        height: 100%;
    }
    .win-btn {
        border-radius: 0;
        background: none;
        border: 0;
        margin: 0;
        display: flex;
        justify-content: center;
        align-items: center;
        width: 46px;
        height: 100%;

        &:hover {
            background: #3B4853;
        }
    }
    .win-btn-close:hover {background: #d51426;}
</style>