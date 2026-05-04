<script lang="ts">
  import { textColorForTagBackground, type TagDefRow } from "../lib/accountTagsContext";

  export let tags: TagDefRow[] = [];
</script>

{#if tags.length > 0}
  <div class="acc_tag_bubbles">
    {#each tags as tg (tg.id)}
      <span
        class="acc_tag_bubble"
        style:background-color={tg.color || "#555"}
        style:color={textColorForTagBackground(tg.color || "#555")}>{tg.name}</span>
    {/each}
  </div>
{/if}

<style lang="scss">
  .acc_tag_bubbles {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin: 0.2rem 0 0;
    max-width: 100%;
    flex-direction: row;
    justify-content: center;
  }

  .acc_tag_bubble {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border-radius: 999px;
    font-size: 0.65rem;
    font-weight: 600;
    line-height: 1.3;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
