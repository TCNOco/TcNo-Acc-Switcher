<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import SettingsField from "./settings/SettingsField.svelte";

  let scale = 0;
  let saving = false;
  let loaded = false;
  const scales = [0.75, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2];

  onMount(() => {
    void PlatformService.GetInterfaceScale().then(value => { scale = value; })
      .catch(() => {}).finally(() => { loaded = true; });
  });

  async function save(next: number): Promise<void> {
    if (saving || !loaded) return;
    saving = true;
    const previous = scale;
    scale = next;
    try {
      await PlatformService.SetInterfaceScale(next);
    } catch (error) {
      scale = previous;
      pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SaveFailed"), error), duration: 8000 });
    } finally { saving = false; }
  }
</script>

<SettingsField label={$t("Settings_InterfaceScale")} forId="interface-scale" note={$t("Settings_InterfaceScale_Hint")}>
  <select id="interface-scale" value={scale} disabled={saving || !loaded}
    on:change={(event) => void save(Number(event.currentTarget.value))}>
    <option value={0}>{$t("Settings_InterfaceScale_Automatic")}</option>
    {#each scales as value}<option {value}>{Math.round(value * 100)}%</option>{/each}
  </select>
</SettingsField>

<style>
  select { padding: 0.5em; color: var(--whiteSecondary); background: var(--even-darker-code-background); border: 1px solid var(--input-number-border); border-radius: 4px; }
</style>
