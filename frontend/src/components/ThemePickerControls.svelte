<script lang="ts">
  import { tick } from "svelte";
  import { viewportDropdown } from "../lib/actions/viewportDropdown";
  import { t } from "../stores/i18n";
  import {
    currentThemeAccentKey,
    currentThemeCustomAccentColor,
    currentThemeId,
    currentWindowsThemeAccentColor,
    listThemes,
    resolveThemeAccent,
    setUserTheme,
    setUserThemeAccentCustom,
    setUserThemeAccentPreset,
    supportsWindowsThemeAccent,
    WINDOWS_THEME_ACCENT_KEY,
  } from "../lib/themes";

  const themes = listThemes();
  const showWindowsAccent = supportsWindowsThemeAccent();

  let themeOpen = false;
  let accentOpen = false;
  let customAccentInput: HTMLInputElement | null = null;

  $: currentTheme = themes.find((theme) => theme.id === $currentThemeId) ?? themes[0];
  $: currentThemeLabel = currentTheme?.label ?? "";
  $: currentAccent = currentTheme
    ? resolveThemeAccent(currentTheme.id, $currentThemeAccentKey, $currentThemeCustomAccentColor)
    : null;
  $: customAccentPreviewColor = $currentThemeCustomAccentColor || currentAccent?.color || "#ffffff";
  $: windowsAccentPreviewColor = $currentWindowsThemeAccentColor || "#0078d4";

  async function pickTheme(id: string): Promise<void> {
    await setUserTheme(id);
    themeOpen = false;
    accentOpen = false;
  }

  async function pickPresetAccent(id: string): Promise<void> {
    await setUserThemeAccentPreset(id);
    accentOpen = false;
  }

  async function pickCustomAccent(): Promise<void> {
    const nextColor = currentAccent?.color || customAccentPreviewColor;
    await setUserThemeAccentCustom(nextColor);
    accentOpen = false;
    await tick();
    if (typeof customAccentInput?.showPicker === "function") {
      customAccentInput.showPicker();
      return;
    }
    customAccentInput?.focus();
  }

  async function pickWindowsAccent(): Promise<void> {
    await setUserThemeAccentPreset(WINDOWS_THEME_ACCENT_KEY);
    accentOpen = false;
  }

  async function updateCustomAccent(event: Event): Promise<void> {
    const input = event.currentTarget as HTMLInputElement | null;
    if (!input) {
      return;
    }
    await setUserThemeAccentCustom(input.value);
  }
</script>

<div class="theme-control-group">
  <div class="rowDropdown">
    <span>{$t("Settings_CurrentStyle")}</span>
    <div class="dropdown" class:show={themeOpen}>
      <button
        type="button"
        class="dropdown-toggle"
        on:click={() => {
          themeOpen = !themeOpen;
          if (themeOpen) {
            accentOpen = false;
          }
        }}
      >
        {currentThemeLabel}
        <span class="caret" aria-hidden="true"></span>
      </button>
      {#if themeOpen}
        <ul class="custom-dropdown-menu dropdown-menu" use:viewportDropdown>
          {#each themes as theme}
            <li role="none">
              <button type="button" class="dropdown-item" on:click={() => void pickTheme(theme.id)}>
                {theme.label}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  {#if currentTheme && currentAccent}
    <div class="rowDropdown">
      <span>{$t("Settings_AccentColor")}</span>
      <div class="dropdown accent-dropdown" class:show={accentOpen}>
        <button
          type="button"
          class="dropdown-toggle accent-toggle"
          on:click={() => {
            accentOpen = !accentOpen;
            if (accentOpen) {
              themeOpen = false;
            }
          }}
        >
          <span class="theme-accent-chip" style={`--accent-chip-color: ${currentAccent.color}`}></span>
          <span class="accent-toggle-label">
            {#if currentAccent.isCustom}
              {$t("Settings_AccentColor_Custom")}
            {:else if currentAccent.id === WINDOWS_THEME_ACCENT_KEY}
              {$t("Settings_WindowsAccent")}
            {:else}
              {currentAccent.label}
            {/if}
          </span>
          <span class="caret" aria-hidden="true"></span>
        </button>

        {#if accentOpen}
          <ul class="custom-dropdown-menu dropdown-menu accent-dropdown-menu" use:viewportDropdown>
            <li role="none">
              <button
                type="button"
                class="dropdown-item accent-dropdown-item"
                class:active={currentAccent.isCustom}
                on:click={() => void pickCustomAccent()}
              >
                <span
                  class="theme-accent-chip"
                  style={`--accent-chip-color: ${customAccentPreviewColor}`}
                ></span>
                <span>{$t("Settings_AccentColor_Custom")}</span>
              </button>
            </li>

            {#if showWindowsAccent}
              <li role="none">
                <button
                  type="button"
                  class="dropdown-item accent-dropdown-item"
                  class:active={$currentThemeAccentKey === WINDOWS_THEME_ACCENT_KEY}
                  on:click={() => void pickWindowsAccent()}
                >
                  <span
                    class="theme-accent-chip"
                    style={`--accent-chip-color: ${windowsAccentPreviewColor}`}
                  ></span>
                  <span>{$t("Settings_WindowsAccent")}</span>
                </button>
              </li>
            {/if}

            {#each currentTheme.accents as accent}
              <li role="none">
                <button
                  type="button"
                  class="dropdown-item accent-dropdown-item"
                  class:active={$currentThemeAccentKey
                    ? $currentThemeAccentKey === accent.id
                    : accent.id === currentTheme.defaultAccentKey}
                  on:click={() => void pickPresetAccent(accent.id)}
                >
                  <span
                    class="theme-accent-chip"
                    style={`--accent-chip-color: ${accent.color}`}
                  ></span>
                  <span>{accent.label}</span>
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
  {/if}

  {#if currentAccent?.isCustom}
    <div class="accent-custom-picker-row">
      <span>{$t("Settings_AccentColor_CustomPicker")}</span>
      <input
        bind:this={customAccentInput}
        type="color"
        class="accent-custom-picker"
        value={currentAccent.color}
        on:input={(event) => void updateCustomAccent(event)}
        aria-label={$t("Settings_AccentColor_CustomPicker")}
      />
    </div>
  {/if}

  <slot name="after-controls"></slot>
</div>
