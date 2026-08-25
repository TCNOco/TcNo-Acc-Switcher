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
    background: var(--backdrop-dark-20);
    color: inherit;
    font: inherit;
    cursor: pointer;
    box-sizing: border-box;
    transition: background-color var(--dur-fast) var(--ease-out);

    &:hover:not(:disabled) {
      background: var(--overlay-white-08);
    }

    /* Inset ring rather than a border: a real border is a pixel of layout shift
       on every press. The rule this replaces used `::active`, which is not a
       selector that matches anything. */
    &:active:not(:disabled) {
      box-shadow: inset 0 0 0 1px var(--overlay-white-12);
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
