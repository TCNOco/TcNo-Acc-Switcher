import type { Action } from 'svelte/action'

export type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right'

export type TooltipParams =
    | string
    | undefined
    | {
          text: string
          placement?: TooltipPlacement
          delay?: { show?: number; hide?: number }
      }

type Parsed = {
    text: string
    placement: TooltipPlacement
    delayShow: number
    delayHide: number
}

function parseParam(param: TooltipParams): Parsed | null {
    if (param == null || param === '') return null
    if (typeof param === 'string') {
        return { text: param, placement: 'top', delayShow: 0, delayHide: 0 }
    }
    const d = param.delay
    return {
        text: param.text,
        placement: param.placement ?? 'top',
        delayShow: d?.show ?? 0,
        delayHide: d?.hide ?? 0,
    }
}

const GAP = 4
const VIEWPORT_PAD = 15
const Z = 999999

function clamp(n: number, min: number, max: number) {
    return Math.min(Math.max(n, min), max)
}

function placeTooltip(
    node: DOMRect,
    tip: HTMLElement,
    placement: TooltipPlacement
) {
    const vw = window.innerWidth
    const vh = window.innerHeight
    const w = tip.offsetWidth
    const h = tip.offsetHeight

    let top = 0
    let left = 0

    switch (placement) {
        case 'top':
            left = node.left + node.width / 2 - w / 2
            top = node.top - h - GAP
            break
        case 'bottom':
            left = node.left + node.width / 2 - w / 2
            top = node.bottom + GAP
            break
        case 'left':
            left = node.left - w - GAP
            top = node.top + node.height / 2 - h / 2
            break
        case 'right':
            left = node.right + GAP
            top = node.top + node.height / 2 - h / 2
            break
    }

    left = clamp(left, VIEWPORT_PAD, vw - w - VIEWPORT_PAD)
    top = clamp(top, VIEWPORT_PAD, vh - h - VIEWPORT_PAD)

    tip.style.left = `${Math.round(left)}px`
    tip.style.top = `${Math.round(top)}px`
    tip.dataset.placement = placement
}

function buildTooltip(text: string, placement: TooltipPlacement) {
    const root = document.createElement('div')
    root.className = 'tooltip fade'
    root.setAttribute('role', 'tooltip')
    root.dataset.placement = placement

    const arrow = document.createElement('div')
    arrow.className = 'tooltip-arrow'

    const inner = document.createElement('div')
    inner.className = 'tooltip-inner'
    inner.textContent = text

    root.append(arrow, inner)
    return root
}

export const tooltip: Action<HTMLElement, TooltipParams> = (node, param) => {
    let parsed = parseParam(param)
    let tip: HTMLElement | null = null
    let showTimer: ReturnType<typeof setTimeout> | null = null
    let hideTimer: ReturnType<typeof setTimeout> | null = null

    const clearShow = () => {
        if (showTimer != null) {
            clearTimeout(showTimer)
            showTimer = null
        }
    }

    const clearHide = () => {
        if (hideTimer != null) {
            clearTimeout(hideTimer)
            hideTimer = null
        }
    }

    const hide = () => {
        clearShow()
        clearHide()
        if (!tip) return
        tip.classList.remove('show')
        const el = tip
        window.setTimeout(() => {
            el.remove()
            if (tip === el) tip = null
        }, 150)
    }

    const scheduleHide = () => {
        clearHide()
        const ms = parsed?.delayHide ?? 0
        hideTimer = window.setTimeout(hide, ms)
    }

    const show = () => {
        if (!parsed?.text) return
        clearHide()
        if (tip?.classList.contains('show')) return

        if (!tip) {
            tip = buildTooltip(parsed.text, parsed.placement)
            tip.style.position = 'fixed'
            tip.style.zIndex = String(Z)
            tip.style.pointerEvents = 'none'
            document.body.append(tip)
        } else {
            const inner = tip.querySelector('.tooltip-inner')
            if (inner) inner.textContent = parsed.text
            tip.dataset.placement = parsed.placement
        }

        requestAnimationFrame(() => {
            if (!tip || !parsed) return
            placeTooltip(node.getBoundingClientRect(), tip, parsed.placement)
            void tip.offsetWidth
            tip.classList.add('show')
        })
    }

    const scheduleShow = () => {
        if (!parsed?.text) return
        clearShow()
        showTimer = window.setTimeout(show, parsed.delayShow)
    }

    const onMouseEnter = () => scheduleShow()
    const onMouseLeave = () => {
        clearShow()
        scheduleHide()
    }
    const onFocusIn = () => scheduleShow()
    const onFocusOut = () => {
        clearShow()
        scheduleHide()
    }

    const onScrollOrResize = () => {
        if (tip?.classList.contains('show')) hide()
    }

    const onKeydown = (e: KeyboardEvent) => {
        if (e.key === 'Escape') hide()
    }

    node.addEventListener('mouseenter', onMouseEnter)
    node.addEventListener('mouseleave', onMouseLeave)
    node.addEventListener('focusin', onFocusIn)
    node.addEventListener('focusout', onFocusOut)
    window.addEventListener('scroll', onScrollOrResize, true)
    window.addEventListener('resize', onScrollOrResize)
    document.addEventListener('keydown', onKeydown)

    return {
        update(newParam: TooltipParams) {
            parsed = parseParam(newParam)
            if (!parsed?.text) hide()
            else if (tip?.classList.contains('show')) {
                const inner = tip.querySelector('.tooltip-inner')
                if (inner) inner.textContent = parsed.text
                requestAnimationFrame(() => {
                    if (tip && parsed) placeTooltip(node.getBoundingClientRect(), tip, parsed.placement)
                })
            }
        },
        destroy() {
            clearShow()
            clearHide()
            node.removeEventListener('mouseenter', onMouseEnter)
            node.removeEventListener('mouseleave', onMouseLeave)
            node.removeEventListener('focusin', onFocusIn)
            node.removeEventListener('focusout', onFocusOut)
            window.removeEventListener('scroll', onScrollOrResize, true)
            window.removeEventListener('resize', onScrollOrResize)
            document.removeEventListener('keydown', onKeydown)
            tip?.remove()
            tip = null
        },
    }
}
