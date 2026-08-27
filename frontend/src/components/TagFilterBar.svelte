<script lang="ts">
  import { collapse, DUR } from "../lib/animation";

  export let label = "";
  export let onClick: (ev: Event) => void = () => {};
  export let disabled = false;
</script>

<!-- The bar only exists while a tag filter is active, and it sits directly above
     the account grid: collapsing its height moves the grid instead of jolting it. -->
<button
  type="button"
  class="tag-filter-bar"
  class:tag-filter-bar--disabled={disabled}
  on:click={disabled ? undefined : onClick}
  disabled={disabled}
  transition:collapse={{ duration: DUR.normal }}
>
  <span class="tag-filter-bar__label">{label}</span>
  <span class="tag-filter-bar__icon" aria-hidden="true">
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="currentColor"
    >
      <path d="M10 18h4v-2h-4v2zM3 6v2h18V6H3zm3 7h12v-2H6v2z" />
    </svg>
  </span>
</button>

<style lang="scss">
  .tag-filter-bar {
    display: flex;
    align-items: center;
    width: 100%;
    margin: 0 0 0.5rem;
    padding: 0.4rem 0.5rem;
    border: 0px solid transparent;
    /* Opaque, not a scrim: `--backdrop-dark-20` is rgba(0,0,0,0.2) in all 22
       themes, so it leaves the bar a shade of whatever is behind it, 1.69:1 on
       the WinVista theme and at the mercy of the wallpaper on the themes that
       ship one. `--surface-row-dark` is the pairing
       `.acc_list_actionbar` uses for the other bar in this list, opaque in all 22
       themes, worst case 5.17:1 under `--whiteSecondary`. */
    background: var(--surface-row-dark);
    color: var(--whiteSecondary);
    font: inherit;
    cursor: pointer;
    box-sizing: border-box;
    /* The Windows themes bevel every `button`; this bar is a band in a list, not
       a control to press, so it declines the bevel. */
    box-shadow: none;
    transition:
      box-shadow var(--dur-fast) var(--ease-out),
      filter var(--dur-fast) var(--ease-out);

    /* An accent ring rather than a fill change: a fill has to move in a
       different direction on light themes than on dark ones, and the accent is
       the one colour every theme guarantees will read against its own surfaces.
       Inset, so neither state costs a pixel of layout shift. */
    &:hover:not(:disabled) {
      box-shadow: inset 0 0 0 1px var(--accent);
    }

    &:active:not(:disabled) {
      box-shadow: inset 0 0 0 1px var(--accent);
      filter: brightness(0.92);
    }
  }

  .tag-filter-bar--disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .tag-filter-bar__label {
    flex: 1;
    text-align: center;
    font-size: 0.85rem;
  }

  .tag-filter-bar__icon {
    display: flex;
    flex-shrink: 0;
    opacity: 0.85;
  }
</style>
