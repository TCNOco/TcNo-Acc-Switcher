<script lang="ts">
    import { route, navigateBackLikeButton } from '../stores/nav'
    import { t } from "../stores/i18n";
    import { onMount } from 'svelte'
    import { appBarTitle } from '../stores/nav'
    import { activeModal } from '../stores/modal'
    import { Events, Window } from "@wailsio/runtime";

    let minimised = false
    let maximised = false

    const common = Events.Types.Common

    async function refreshWindowState() {
        const [min, max] = await Promise.all([
            Window.IsMinimised(),
            Window.IsMaximised(),
        ])
        minimised = min
        maximised = max
    }

    onMount(() => {
        void refreshWindowState()

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
                void refreshWindowState()
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
    $: titleLabel = (() => {
        if ($route.page === "manage-platforms") {
            return $t("Title_Platforms_Settings")
        }
        if ($route.page === "preview-css") {
            return $t("Title_Settings_TestCss")
        }
        if ($route.page === "settings") {
            return $t("Title_Settings")
        }
        if ($route.page === "platform-settings") {
            return $t("Title_Template_Settings", { platformName: $route.platformName })
        }
        if ($route.page === "platform") {
            return $t("Title_AccountsList", { platform: $route.platformName })
        }
        return $appBarTitle
    })()

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
            navigateBackLikeButton()
        }
    }
</script>

<header class="headerbar">
    <span class="title-left">
        <button
            type="button"
            class="win-btn win-btn-back"
            title={$t("Aria_WindowBack")}
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
    <span id="title-label" class="title-drag">
        {titleLabel}
    </span>
    <span class="window-controls" role="toolbar">
        <button type="button" class="win-btn win-btn-min" aria-label={$t("Aria_WindowMinimize")} on:click={() => Window.Minimise()}>
            <svg class="win-btn__glyph win-btn__glyph--min" viewBox="0 0 10 10" aria-hidden="true">
                <path d="M1 5h8" />
            </svg>
        </button>
        {#if maximised}
            <button type="button" class="win-btn win-btn-max" aria-label={$t("Aria_WindowRestore")} on:click={() => Window.Restore()}>
                <svg class="win-btn__glyph win-btn__glyph--restore" viewBox="0 0 10 10" aria-hidden="true">
                    <path d="M2.5 1.5h5v5h-5z" />
                    <path d="M4.5 3.5h4v5h-5v-4" />
                </svg>
            </button>
        {:else}
            <button type="button" class="win-btn win-btn-max" aria-label={$t("Aria_WindowMaximize")} on:click={() => Window.Maximise()}>
                <svg class="win-btn__glyph win-btn__glyph--max" viewBox="0 0 10 10" aria-hidden="true">
                    <path d="M1.5 1.5h7v7h-7z" />
                </svg>
            </button>
        {/if}
        <button type="button" class="win-btn win-btn-close" aria-label={$t("Aria_WindowClose")} on:click={() => Window.Close()}>
            <svg class="win-btn__glyph win-btn__glyph--close" viewBox="0 0 10 10" aria-hidden="true">
                <path d="M2 2l6 6" />
                <path d="M8 2L2 8" />
            </svg>
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
        color: var(--whiteSecondary);
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
        z-index: 1;
        height: 100%;
        display: flex;
        flex-direction: row;
    }
    .title-drag {
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        max-width: calc(100% - 300px);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        pointer-events: none;
    }
    .win-btn-back {
        --wails-draggable: no-drag;
        svg {
            fill: var(--whiteSecondary);
            height: 0.8rem;
            display: block;
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
        fill: var(--whiteSecondary);
    }
    .window-controls {
        --wails-draggable: no-drag;
        z-index: 1;
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
            background: var(--window-control-hover-bg);
        }
    }
    .win-btn__glyph {
        width: 10px;
        height: 10px;
        display: block;
        overflow: visible;
        color: currentColor;
        fill: none;
        stroke: currentColor;
        stroke-width: 1.2;
        stroke-linecap: square;
        stroke-linejoin: miter;
        vector-effect: non-scaling-stroke;
        forced-color-adjust: auto;
    }
    .win-btn__glyph--min {
        stroke-linecap: butt;
    }
    .win-btn-close:hover {background: var(--window-close-hover);}
</style>
