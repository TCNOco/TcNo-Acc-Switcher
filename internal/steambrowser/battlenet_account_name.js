(() => {
  if (window.top !== window || location.origin !== "https://account.battle.net" || !/^\/security\/?$/.test(location.pathname)) return null;
  const roots = [document];
  for (let i = 0; i < roots.length && i < 32; i++) {
    for (const node of Array.from(roots[i].querySelectorAll("*")).slice(0, 6000)) {
      if (node.shadowRoot && roots.length < 32) roots.push(node.shadowRoot);
    }
  }
  const tags = new Set(), emails = new Set(), names = new Set();
  const tagPattern = /[\p{L}\p{N}][\p{L}\p{N}\p{M}_-]{1,31}#[0-9]{3,12}/gu;
  const emailPattern = /[A-Za-z0-9.!#$%&'*+\/=?^_`{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+/g;
  for (const root of roots) {
    for (const nav of root.querySelectorAll("blz-nav")) {
      if (!nav.getClientRects().length) continue;
      const tag = typeof nav.user?.battletag === "string" ? nav.user.battletag.trim() : "";
      const email = typeof nav.user?.email === "string" ? nav.user.email.trim() : "";
      if (tag.length <= 64 && tag.match(tagPattern)?.[0] === tag) tags.add(tag);
      if (email.length <= 320 && email.match(emailPattern)?.[0] === email) emails.add(email);
    }
  }
  if (tags.size || emails.size) {
    const name = tags.size === 1 ? [...tags][0] : tags.size > 1 ? "" : emails.size === 1 ? [...emails][0] : "";
    return { source: location.origin + location.pathname, name };
  }
  for (const root of roots) {
    const dedicated = root.querySelectorAll('[data-testid*="battletag" i], [id*="battletag" i], [class*="battletag" i], [class*="battle-tag" i], [slot="username"], [slot="account-name"]');
    const headers = root.querySelectorAll('header, nav, [role="navigation"], blz-nav, blz-navbar, blz-account-menu, blz-profile-menu');
    for (const node of [...dedicated, ...headers]) {
      if (!node.getClientRects().length) continue;
      const text = (node.innerText || node.textContent || "").trim().slice(0, 4096).replace(/\s*#\s*(\d{3,12})/g, "#$1");
      for (const tag of text.match(tagPattern) || []) tags.add(tag);
      for (const email of text.match(emailPattern) || []) emails.add(email);
      if ([...dedicated].includes(node) && /^[\p{L}\p{N}\p{M}_-]{2,32}$/u.test(text) && !/^(battletag|account|username|battle\.net)$/i.test(text)) names.add(text);
    }
  }
  const name = tags.size === 1 ? [...tags][0] : tags.size > 1 ? "" : emails.size === 1 ? [...emails][0] : emails.size > 1 ? "" : names.size === 1 ? [...names][0] : "";
  return { source: location.origin + location.pathname, name };
})()
