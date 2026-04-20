import type { Action } from 'svelte/action'

export type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right'

export type TooltipParams =
    | string
    | undefined
    | {
          text: string
          placement?: TooltipPlacement
          delay?: { show?: number; hide?: number }
          /** Clamp position inside this element’s rect (e.g. scroll pane), not only the window. */
          boundary?: HTMLElement | null
      }

type Parsed = {
    text: string
    placement: TooltipPlacement
    delayShow: number
    delayHide: number
    boundary?: HTMLElement | null
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
        boundary: param.boundary,
    }
}

const GAP = 4
const VIEWPORT_PAD = 15
const Z = 999999

function clamp(n: number, min: number, max: number) {
    return Math.min(Math.max(n, min), max)
}

function clampBounds(
    vw: number,
    vh: number,
    w: number,
    h: number,
    boundary?: HTMLElement | null
) {
    let minL = VIEWPORT_PAD
    let minT = VIEWPORT_PAD
    let maxL = vw - w - VIEWPORT_PAD
    let maxT = vh - h - VIEWPORT_PAD
    if (boundary) {
        const br = boundary.getBoundingClientRect()
        const pad = 4
        minL = Math.max(minL, br.left + pad)
        minT = Math.max(minT, br.top + pad)
        maxL = Math.min(maxL, br.right - w - pad)
        maxT = Math.min(maxT, br.bottom - h - pad)
    }
    if (maxL < minL) {
        minL = VIEWPORT_PAD
        maxL = vw - w - VIEWPORT_PAD
    }
    if (maxT < minT) {
        minT = VIEWPORT_PAD
        maxT = vh - h - VIEWPORT_PAD
    }
    return { minL, minT, maxL, maxT }
}

function placeTooltip(
    node: DOMRect,
    tip: HTMLElement,
    placement: TooltipPlacement,
    boundary?: HTMLElement | null
) {
    const vw = window.innerWidth
    const vh = window.innerHeight
    const w = tip.offsetWidth
    const h = tip.offsetHeight

    const { minL, minT, maxL, maxT } = clampBounds(vw, vh, w, h, boundary)

    let usePlacement: TooltipPlacement = placement
    if (boundary && (placement === 'right' || placement === 'left')) {
        const br = boundary.getBoundingClientRect()
        const pad = 4
        const canRight = node.right + GAP + w <= br.right - pad
        const canLeft = node.left - GAP - w >= br.left + pad
        if (placement === 'right' && !canRight && canLeft) {
            usePlacement = 'left'
        } else if (placement === 'left' && !canLeft && canRight) {
            usePlacement = 'right'
        }
    }

    let top = 0
    let left = 0

    switch (usePlacement) {
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

    left = clamp(left, minL, maxL)
    top = clamp(top, minT, maxT)

    tip.style.left = `${Math.round(left)}px`
    tip.style.top = `${Math.round(top)}px`
    tip.dataset.placement = usePlacement
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
            placeTooltip(
                node.getBoundingClientRect(),
                tip,
                parsed.placement,
                parsed.boundary
            )
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
                    if (tip && parsed)
                        placeTooltip(
                            node.getBoundingClientRect(),
                            tip,
                            parsed.placement,
                            parsed.boundary
                        )
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
