<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { appBarTitle, previousPage } from "../stores/nav";
  import { t } from "../stores/i18n";
  import {
    confirmationActionsEnabled,
    confirmationStatusMessageKey,
    confirmationUsesOverlay,
    steamConfirmationsWindow,
    type ConfirmationItem,
    type ConfirmationRow,
    type ConfirmationTradeItem,
    type SteamConfirmationsState,
  } from "../lib/steamConfirmations";
  import { flip } from "svelte/animate";
  import { loadSteamGuardSwitcherProfile } from "../lib/steamGuardBridge";
  import { collapse, DUR, flipMotion } from "../lib/animation";
  import { Browser } from "@wailsio/runtime";
  import "../styles/steam-confirmations.scss";

  const confirmations = steamConfirmationsWindow;
  const PROFILE_FALLBACK = "/img/BasicDefault.webp";
  let now = Date.now();
  let clockTimer: ReturnType<typeof setInterval> | null = null;
  let hoveredKey = "";
  let hoveredForHandle: string | null = null;
  let hoveredTrigger: HTMLElement | null = null;
  let overlayEl: HTMLElement | undefined;
  let describedItems: Record<string, ConfirmationItem> = {};
  let accountAvatar = "";
  let accountDisplayName = "";
  let loadedProfileFor = "";

  /**
   * The account's switcher avatar and community name, so this window identifies the
   * account the same way the Steam Guard modal does. Best effort: without it the
   * header still shows the login username and the placeholder image.
   */
  async function loadAccountProfile(accountId: string, username: string): Promise<void> {
    if (!accountId || loadedProfileFor === accountId) return;
    loadedProfileFor = accountId;
    try {
      const profile = await loadSteamGuardSwitcherProfile(accountId, username);
      accountAvatar = profile?.imageUrl?.trim() || profile?.staticImageUrl?.trim() || "";
      accountDisplayName = profile?.displayName?.trim() ?? "";
    } catch (error) {
      console.error("Steam confirmations: account profile unavailable", error);
    }
  }

  $: void loadAccountProfile($confirmations.accountId, $confirmations.accountLabel);
  // A card belongs to the confirmation it was opened from; leaving that one must
  // not leave a card hanging over the list or over the next trade.
  $: if ($confirmations.selectedHandle !== hoveredForHandle) clearHover();

  // The window is frameless, so this is the title the user actually sees. It names
  // the account whenever one is known, which is every state except a refresh that
  // has not returned yet.
  $: appBarTitle.set(
    $confirmations.accountLabel
      ? $t("SteamGuard_Confirmations_TitleFor", { account: $confirmations.accountLabel })
      : $t("SteamGuard_Confirmations_Title"),
  );
  $: selected = $confirmations.rows.find((row) => row.handle === $confirmations.selectedHandle) ?? null;
  $: overlay = selected !== null && confirmationUsesOverlay(selected);
  $: checkedCount = $confirmations.rows.filter((row) => $confirmations.checked[row.handle]).length;
  $: allChecked = checkedCount > 0 && checkedCount === $confirmations.rows.length;
  $: deciding = $confirmations.pendingHandle !== null;
  $: batchEnabled = $confirmations.status === "fresh" && !deciding;

  // Which batch button is doing the deciding, so it can carry the busy state:
  // the store submits row by row, and "the first row says submitting" does not
  // read as "your whole batch is on its way".
  let batchPending: "accept" | "deny" | null = null;

  async function decideBatch(decision: "accept" | "deny"): Promise<void> {
    batchPending = decision;
    try {
      await confirmations.decideChecked(decision);
    } finally {
      batchPending = null;
    }
  }

  /** Esc closes the open confirmation, matching the close button. */
  function onWindowKeydown(event: KeyboardEvent): void {
    if (event.key !== "Escape" || $confirmations.selectedHandle === null) return;
    event.preventDefault();
    confirmations.deselect();
  }

  /**
   * The trade overlay is modal, so opening it moves focus inside — onto the
   * close button — and closing it hands focus back to where it was. Without
   * this, Tab keeps walking the list underneath the dialog. The sheet is not
   * modal and keeps focus where the user left it; a row whose kind only turns
   * out to be a trade once its detail loads is caught by the update.
   */
  function focusOnOpen(node: HTMLElement, modal: boolean) {
    let previous: HTMLElement | null = null;
    const steal = (): void => {
      if (previous) return;
      previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      node.focus();
    };
    if (modal) steal();
    return {
      update(nextModal: boolean) {
        if (nextModal) steal();
      },
      destroy() {
        previous?.focus();
      },
    };
  }

  // A listing has to fit a bar a few lines tall, so it reads as a headline and a
  // footnote rather than as blocks. What the sale pays the seller is the number
  // the answer turns on; everything after it is context, and goes on one line.
  //
  // The wording is ours, not Steam's: the backend hands over amounts and counts,
  // so these read in the user's language rather than in whatever language the
  // Steam account happens to use. Steam's own text appears only where a value
  // could not be read, which is the price labels and market lines of a non-English
  // account.
  $: listing = selected?.listing ?? null;
  $: listingHeadline = !listing
    ? ""
    : listing.receive
      ? $t("SteamGuard_Confirmations_Listing_Receive", { amount: listing.receive })
      : listing.prices.length > 0
        ? `${listing.prices[0].label} ${listing.prices[0].value}`
        : "";
  $: listingFootnote = !listing ? "" : [
    listing.buyerPays ? $t("SteamGuard_Confirmations_Listing_BuyerPays", { amount: listing.buyerPays }) : "",
    ...listing.prices.slice(1).map((price) => `${price.label} ${price.value}`),
    listing.market.forSale > 0
      ? $t("SteamGuard_Confirmations_Listing_ForSale", {
        count: listing.market.forSale,
        price: listing.market.forSalePrice,
      })
      : "",
    listing.market.soldRecently > 0
      ? $t("SteamGuard_Confirmations_Listing_SoldRecently", { count: listing.market.soldRecently })
      : "",
    ...listing.market.text,
  ].filter(Boolean).join(" · ");
  $: retrySeconds = $confirmations.retryAt === null
    ? 0
    : Math.max(0, Math.ceil(($confirmations.retryAt - now) / 1000));

  // Reactive declarations, not a function called from the template: Svelte
  // tracks the dependencies of the expression it sees, so `{#if statusMessage()}`
  // never re-ran and the reauth banner stayed invisible.
  $: statusMessageKey = confirmationStatusMessageKey($confirmations.status);
  $: statusMessage = statusMessageKey ? $t(statusMessageKey, { seconds: retrySeconds }) : "";

  /**
   * Steam describes an item fully only on its own endpoint, so a description is
   * loaded the first time an item is hovered and kept for the rest of the session.
   */
  function itemKey(item: ConfirmationTradeItem): string {
    return `${item.appId}/${item.classId}/${item.instanceId}`;
  }

  async function describe(item: ConfirmationTradeItem, trigger: HTMLElement): Promise<void> {
    const key = itemKey(item);
    hoveredKey = key;
    hoveredForHandle = $confirmations.selectedHandle;
    hoveredTrigger = trigger;
    if (describedItems[key] !== undefined) return;
    const described = await confirmations.describeItem(item);
    if (described) describedItems = { ...describedItems, [key]: described };
  }

  function clearHover(): void {
    hoveredKey = "";
    hoveredForHandle = $confirmations.selectedHandle;
    hoveredTrigger = null;
  }

  /**
   * Places the item card against the hovered tile and keeps it inside the window.
   *
   * Above the tile by preference, then below. A card with many description lines
   * fits neither, and stacking it vertically anyway is what pushed it off the
   * page — so it goes beside the tile instead, where the space it needs is
   * horizontal, and is clamped on both axes.
   */
  function placeCard(node: HTMLElement, _described?: ConfirmationItem) {
    const apply = (): void => {
      const host = overlayEl;
      const trigger = hoveredTrigger;
      if (!host || !trigger) return;
      const hostRect = host.getBoundingClientRect();
      const rect = trigger.getBoundingClientRect();
      const card = node.getBoundingClientRect();
      const gap = 8;
      const edge = 8;

      const top = rect.top - hostRect.top;
      const bottom = rect.bottom - hostRect.top;
      const left = rect.left - hostRect.left;
      const right = rect.right - hostRect.left;
      const clamp = (value: number, max: number): number => Math.min(Math.max(value, edge), Math.max(edge, max));

      const fitsAbove = top - gap - card.height >= edge;
      const fitsBelow = bottom + gap + card.height <= host.clientHeight - edge;
      if (fitsAbove || fitsBelow) {
        const x = left + rect.width / 2 - card.width / 2;
        node.style.left = `${clamp(x, host.clientWidth - card.width - edge)}px`;
        const y = fitsAbove ? top - gap - card.height : bottom + gap;
        node.style.top = `${y + host.scrollTop}px`;
        return;
      }

      // Beside it: whichever side has room, preferring the right.
      const x = right + gap + card.width <= host.clientWidth - edge
        ? right + gap
        : left - gap - card.width;
      node.style.left = `${clamp(x, host.clientWidth - card.width - edge)}px`;
      const centred = top + rect.height / 2 - card.height / 2;
      node.style.top = `${clamp(centred, host.clientHeight - card.height - edge) + host.scrollTop}px`;
    };
    apply();
    return { update: apply };
  }

  /** Opens a Steam profile in the user's own browser, not in this window. */
  function openProfile(url: string): void {
    if (!url) return;
    void Browser.OpenURL(url).catch((error: unknown) => {
      console.error("Steam confirmations: profile could not be opened", error);
    });
  }

  function actionEnabled(state: SteamConfirmationsState, row: ConfirmationRow): boolean {
    return confirmationActionsEnabled(state, row.handle);
  }

  function syncVisibility(): void {
    confirmations.setVisible(document.visibilityState === "visible");
  }

  function refreshWhenOnline(): void {
    if (document.visibilityState === "visible") void confirmations.refresh();
  }

  onMount(() => {
    previousPage.set({ page: "platform", platformName: "Steam" });
    document.addEventListener("visibilitychange", syncVisibility);
    window.addEventListener("online", refreshWhenOnline);
    clockTimer = setInterval(() => { now = Date.now(); }, 1000);
    syncVisibility();
  });

  onDestroy(() => {
    document.removeEventListener("visibilitychange", syncVisibility);
    window.removeEventListener("online", refreshWhenOnline);
    if (clockTimer !== null) clearInterval(clockTimer);
    confirmations.setVisible(false);
  });
</script>

<svelte:window on:keydown={onWindowKeydown} />

<section class="confirmations-page" aria-labelledby="confirmations-title">
  <header class="confirmations-toolbar">
    <!-- The window title already says which account and that this is Confirmations,
         so the header only identifies the account. -->
    <span class="confirmations-identity-avatar">
      <img src={accountAvatar || PROFILE_FALLBACK} alt="" draggable="false" />
    </span>
    <span class="confirmations-identity" id="confirmations-title">
      <strong>{$confirmations.accountLabel || $t("SteamGuard_Confirmations_Title")}</strong>
      {#if accountDisplayName && accountDisplayName !== $confirmations.accountLabel}
        <small>{accountDisplayName}</small>
      {/if}
    </span>
    {#if $confirmations.fetchedAt !== null}
      <small class="confirmations-updated">
        {$t("SteamGuard_Confirmations_Updated", { time: new Date($confirmations.fetchedAt).toLocaleTimeString() })}
      </small>
    {/if}
    <button
      type="button"
      class="confirmations-refresh"
      disabled={$confirmations.refreshing || $confirmations.pendingHandle !== null || ($confirmations.status === "rate-limit" && retrySeconds > 0)}
      aria-busy={$confirmations.refreshing}
      aria-label={$t("SteamGuard_Confirmations_Refresh")}
      title={$confirmations.refreshing ? $t("SteamGuard_Confirmations_Refreshing") : $t("SteamGuard_Confirmations_Refresh")}
      on:click={() => confirmations.refresh()}
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7" /></svg>
    </button>
  </header>

  {#if statusMessage}
    <div class="confirmations-status confirmations-status--{$confirmations.status}" role="status" aria-live="polite">
      <span>{statusMessage}</span>
      {#if $confirmations.message}<small>{$confirmations.message}</small>{/if}
      {#if $confirmations.status === "reauth"}
        <button type="button" class="btnicontext" disabled={$confirmations.refreshing} on:click={() => confirmations.loginAgain()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M1 9.2L10 9.2L10 5.2L16.5 12L10 18.8L10 14.8L1 14.8ZM13.5 2L22.5 2L22.5 22L13.5 22L13.5 18.6L19.2 18.6L19.2 5.4L13.5 5.4Z"/></svg>
          {$t("SteamGuard_LoginAgain")}
        </button>
      {:else if $confirmations.status === "canceled"}
        <button type="button" class="btnicontext" disabled={$confirmations.refreshing} on:click={() => confirmations.refresh()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"/></svg>
          {$t("SteamGuard_Confirmations_Retry")}
        </button>
      {:else if $confirmations.status === "stale-error"}
        <!-- Steam refusing the list without saying why is often a session that
             needs replacing, so the dead end offers the way out of it too. -->
        <button type="button" class="btnicontext" disabled={$confirmations.refreshing} on:click={() => confirmations.refresh()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"/></svg>
          {$t("SteamGuard_Confirmations_Retry")}
        </button>
        <button type="button" class="btnicontext" disabled={$confirmations.refreshing} on:click={() => confirmations.loginAgain()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M1 9.2L10 9.2L10 5.2L16.5 12L10 18.8L10 14.8L1 14.8ZM13.5 2L22.5 2L22.5 22L13.5 22L13.5 18.6L19.2 18.6L19.2 5.4L13.5 5.4Z"/></svg>
          {$t("SteamGuard_LoginAgain")}
        </button>
      {/if}
    </div>
  {/if}

  {#if $confirmations.status === "loading" && $confirmations.rows.length === 0}
    <div class="confirmations-loading" aria-live="polite">
      <span class="confirmations-spinner" aria-hidden="true"></span>
      <p>{$t("SteamGuard_Confirmations_Loading")}</p>
    </div>
  {:else if $confirmations.status === "empty"}
    <div class="confirmations-empty">
      <div class="confirmations-empty__mark" aria-hidden="true">✓</div>
      <h2>{$t("SteamGuard_Confirmations_EmptyTitle")}</h2>
      <p>{$t("SteamGuard_Confirmations_EmptyBody")}</p>
    </div>
  {:else if $confirmations.rows.length > 0}
    <div class="confirmations-workspace">
      <nav class="confirmations-list" aria-label={$t("SteamGuard_Confirmations_ListLabel")}>
        {#each $confirmations.rows as row (row.handle)}
          {@const rowChecked = Boolean($confirmations.checked[row.handle])}
          <!-- Two controls, the way the Steam app splits them: the image marks the
               row for a batch decision, the rest of the row opens it. -->
          <!-- A decided confirmation collapses out of the list and the rows below
               it slide up, so the queue visibly shortens instead of blinking. -->
          <div
            class="confirmations-row"
            class:active={row.handle === $confirmations.selectedHandle}
            class:checked={rowChecked}
            animate:flip={flipMotion()}
            out:collapse={{ duration: DUR.fast }}
          >
            <button
              type="button"
              class="confirmations-row__icon"
              role="checkbox"
              aria-checked={rowChecked}
              disabled={deciding}
              aria-label={$t("SteamGuard_Confirmations_MarkRow", { title: row.title })}
              title={$t("SteamGuard_Confirmations_MarkRow", { title: row.title })}
              on:click={() => confirmations.toggleChecked(row.handle)}
            >
              {#if row.icon}
                <img src={row.icon} alt="" draggable="false" />
              {/if}
              <span class="confirmations-row__check" aria-hidden="true">
                <svg viewBox="0 0 24 24"><path d="M5 13l4 4L19 7" /></svg>
              </span>
            </button>
            <button
              type="button"
              class="confirmations-row__body"
              aria-current={row.handle === $confirmations.selectedHandle ? "true" : undefined}
              on:click={() => confirmations.select(row.handle)}
            >
              <span class="confirmations-row__type">
                <!-- Keyed on kind, not the label: the label is display text the
                     backend may reword. Trade is a swap, a listing is a price
                     tag, and the account changes share a shield. -->
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  {#if row.kind === "trade"}
                    <path d="M8 3L4 7l4 4M4 7h16M16 21l4-4-4-4M20 17H4" />
                  {:else if row.kind === "market"}
                    <!-- Filled, unlike its outlined neighbours: the tag's outline
                         alone reads as a square at this size. The hole is punched
                         by the even-odd rule. -->
                    <path
                      class="confirmations-row__type-fill"
                      fill-rule="evenodd"
                      d="M20.59 13.41 12 4.83A2 2 0 0 0 10.59 4H6a2 2 0 0 0-2 2v4.59A2 2 0 0 0 4.59 12l8.58 8.59a2 2 0 0 0 2.83 0l4.59-4.59a2 2 0 0 0 0-2.83zM7.5 9A1.5 1.5 0 1 1 7.5 6a1.5 1.5 0 0 1 0 3z"
                    />
                  {:else}
                    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                  {/if}
                </svg>
                {row.typeLabel}
              </span>
              <strong>{row.title}</strong>
              <span>{row.summary}</span>
              {#if $confirmations.pendingHandle === row.handle}
                <small role="status">{$t("SteamGuard_Confirmations_Submitting")}</small>
              {/if}
              {#if $confirmations.rowErrors[row.handle]}
                <small role="alert">{$confirmations.rowErrors[row.handle]}</small>
              {/if}
            </button>
          </div>
        {/each}
      </nav>

    </div>
  {/if}

  <!-- A trade takes the whole window: it needs room for both sides of the
       exchange, and reviewing one is the only thing being done at that moment.
       A listing is a few lines of text, so it opens in a bar at the foot of the
       list instead of leaving most of a window empty. Closing either clears the
       selection. -->
  {#if selected}
    <article
      class="confirmations-detail"
      class:confirmations-detail--overlay={overlay}
      class:confirmations-detail--sheet={!overlay}
      bind:this={overlayEl}
      aria-live="polite"
      aria-modal={overlay ? "true" : undefined}
      role={overlay ? "dialog" : "group"}
    >
      <button
        type="button"
        class="confirmations-detail__close"
        aria-label={$t("SteamGuard_Confirmations_Close")}
        title={$t("SteamGuard_Confirmations_Close")}
        use:focusOnOpen={overlay}
        on:click={() => confirmations.deselect()}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18" /></svg>
      </button>
      <header>
        {#if selected.icon}
          <img class="confirmations-detail__icon" src={selected.icon} alt="" draggable="false" />
        {/if}
        <div>
          <!-- A listing is always one item, and the header already carries its
               image and name. What it is — the game and kind of item — belongs
               beside the type rather than in a block repeating both. -->
          <span class="confirmations-detail__type">
            <span>{selected.typeLabel}</span>
            {#if selected.listing?.item?.type}
              <span class="confirmations-detail__source">{selected.listing.item.type}</span>
            {/if}
          </span>
          <h2>
            {#if selected.trade?.partner?.profileUrl}
              <button
                type="button"
                class="fancyLink confirmations-detail__profile"
                title={$t("SteamGuard_Confirmations_ViewProfile")}
                on:click={() => openProfile(selected.trade?.partner?.profileUrl ?? "")}
              >{selected.title}</button>
            {:else}
              {selected.title}
            {/if}
            {#if listingHeadline}
              <span class="confirmations-detail__headline-price">{listingHeadline}</span>
            {/if}
          </h2>
          {#if selected.trade?.partner && (selected.trade.partner.level || selected.trade.partner.yearsBadge)}
            <div class="confirmations-detail__partner-meta">
              {#if selected.trade.partner.level}
                <span>{$t("SteamGuard_Confirmations_Level", { level: selected.trade.partner.level })}</span>
              {/if}
              {#if selected.trade.partner.yearsBadge}
                <img src={selected.trade.partner.yearsBadge} alt="" draggable="false" />
              {/if}
            </div>
          {/if}
          <p>{selected.summary}</p>
          {#if listingFootnote}
            <p class="confirmations-detail__footnote">{listingFootnote}</p>
          {/if}
        </div>
      </header>
      <!-- The detail is fetched when the confirmation is opened, so this is the gap
           between the header and the buttons; without it the wait reads as an
           empty trade. -->
      {#if $confirmations.detailLoadingHandle === selected.handle}
        <div class="confirmations-detail__loading" aria-live="polite">
          <span class="confirmations-spinner" aria-hidden="true"></span>
          <p>{$t("SteamGuard_Confirmations_DetailLoading")}</p>
        </div>
      {/if}
      {#if selected.trade}
        <div class="confirmations-trade">
          {#each [selected.trade.give, selected.trade.receive] as side}
            {#if side.items.length > 0}
              <section class="confirmations-trade__side">
                <h3>{side.header}</h3>
                <ul class="confirmations-trade__grid">
                  <!-- Keyed by position: a trade can hold the same item twice, and
                       two identical ids in a keyed each is a fatal render error. -->
                  {#each side.items as item, index (index)}
                    <li>
                      <button
                        type="button"
                        class="confirmations-trade__item"
                        on:mouseenter={(event) => void describe(item, event.currentTarget)}
                        on:focus={(event) => void describe(item, event.currentTarget)}
                        on:mouseleave={clearHover}
                        on:blur={clearHover}
                      >
                        {#if item.icon}
                          <img src={item.icon} alt="" draggable="false" />
                        {/if}
                      </button>
                    </li>
                  {/each}
                </ul>
              </section>
            {/if}
          {/each}
        </div>
      {/if}
      {#if selected.details.length > 0}
        <dl>
          {#each selected.details as field}
            <div><dt>{field.label}</dt><dd>{field.value}</dd></div>
          {/each}
        </dl>
      {/if}
      {#if hoveredKey}
        {@const described = describedItems[hoveredKey]}
        <div class="confirmations-item-card" role="tooltip" use:placeCard={described}>
          {#if described}
            <strong style={described.nameColor ? `color:${described.nameColor}` : ""}>{described.name}</strong>
            {#if described.type}<span class="confirmations-item-card__type">{described.type}</span>{/if}
            {#each described.descriptions as line}
              <span style={line.color ? `color:${line.color}` : ""}>{line.value}</span>
            {/each}
            {#if described.tags.length > 0}
              <ul class="confirmations-item-card__tags">
                {#each described.tags as tag}
                  <li><em>{tag.category}</em> <span style={tag.color ? `color:${tag.color}` : ""}>{tag.name}</span></li>
                {/each}
              </ul>
            {/if}
          {:else}
            <span class="confirmations-item-card__type">{$t("SteamGuard_Confirmations_ItemLoading")}</span>
          {/if}
        </div>
      {/if}

      <div class="confirmations-actions">
        <button
          type="button"
          class="confirmations-deny"
          disabled={!actionEnabled($confirmations, selected)}
          on:click={() => confirmations.decide(selected.handle, "deny")}
        >{selected.denyLabel}</button>
        <button
          type="button"
          class="confirmations-accept"
          disabled={!actionEnabled($confirmations, selected)}
          on:click={() => confirmations.decide(selected.handle, "accept")}
        >{selected.acceptLabel}</button>
      </div>
    </article>
  {/if}

  <!-- Appears only while something is marked, under the detail bar so an open
       confirmation stays clearly its own thing above it. Accept and Deny act on
       exactly the marked rows. -->
  {#if checkedCount > 0}
    <div class="confirmations-batch" role="toolbar" aria-label={$t("SteamGuard_Confirmations_SelectedActions")}>
      <button type="button" disabled={deciding} on:click={() => allChecked ? confirmations.clearChecked() : confirmations.checkAll()}>
        {$t(allChecked ? "SteamGuard_Confirmations_SelectNone" : "SteamGuard_Confirmations_SelectAll")}
      </button>
      <button type="button" disabled={deciding} on:click={() => confirmations.clearChecked()}>
        {$t("SteamGuard_Confirmations_ClearSelection")}
      </button>
      <button
        type="button"
        class="confirmations-deny"
        disabled={!batchEnabled}
        aria-busy={batchPending === "deny"}
        on:click={() => decideBatch("deny")}
      >
        {#if batchPending === "deny"}<span class="confirmations-spinner" aria-hidden="true"></span>{/if}
        {$t("SteamGuard_Confirmations_DenySelected", { count: checkedCount })}
      </button>
      <button
        type="button"
        class="confirmations-accept"
        disabled={!batchEnabled}
        aria-busy={batchPending === "accept"}
        on:click={() => decideBatch("accept")}
      >
        {#if batchPending === "accept"}<span class="confirmations-spinner" aria-hidden="true"></span>{/if}
        {$t("SteamGuard_Confirmations_AcceptSelected", { count: checkedCount })}
      </button>
    </div>
  {/if}
</section>
