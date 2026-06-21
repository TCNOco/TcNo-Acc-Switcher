<script lang="ts">
  import { t } from "../stores/i18n";

  export let count = 3;
</script>

<div class="acc_list acc_list--skeleton" aria-busy="true" aria-label={$t("Button_Loading")}>
  {#each Array.from({ length: count }) as _, i (i)}
    <div class="acc_list_item acc_list_item--skeleton" aria-hidden="true">
      <div class="acc_skeleton_avatar"></div>
      <div class="acc_skeleton_lines">
        <div class="acc_skeleton_line acc_skeleton_line--name"></div>
      </div>
    </div>
  {/each}
</div>
