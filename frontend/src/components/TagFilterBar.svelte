<script lang="ts">
  export let label = "";
  export let onClick: (ev: Event) => void = () => {};
</script>

<button type="button" class="tag-filter-bar" on:click={onClick}>
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
    border: 1px solid rgba(255, 255, 255, 0.12);
    background: rgba(0, 0, 0, 0.2);
    color: inherit;
    font: inherit;
    cursor: pointer;
    box-sizing: border-box;
    &::active {
      border: 1px solid rgba(255, 255, 255, 0.12)!important;
    }
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
