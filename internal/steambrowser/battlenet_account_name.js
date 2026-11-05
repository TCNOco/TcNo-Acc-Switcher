(() => {
  if (window.top !== window || !/^https:\/\/(?:(?:eu|us|kr|tw|cn)\.)?account\.battle\.net$/.test(location.origin) || !/^\/security\/?$/.test(location.pathname)) return null;
  const roots = [document];
  const navigationRoots = new Set();
  for (let i = 0; i < roots.length && i < 32; i++) {
    for (const node of Array.from(roots[i].querySelectorAll("*")).slice(0, 6000)) {
      if (node.shadowRoot && roots.length < 32) {
        roots.push(node.shadowRoot);
        if (navigationRoots.has(roots[i]) || ["BLZ-NAV", "BLZ-NAV-BATTLENET"].includes(node.tagName)) navigationRoots.add(node.shadowRoot);
      }
    }
  }
  const tags = new Set(), emails = new Set(), names = new Set();
  const tagPattern = /[\p{L}\p{N}][\p{L}\p{N}\p{M}_-]{1,31}#[0-9]{3,12}/gu;
  const emailPattern = /[A-Za-z0-9.!#$%&'*+\/=?^_`{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+/g;
  for (const root of roots) {
    const dedicated = root.querySelectorAll('[data-testid*="battletag" i], [id*="battletag" i], [class*="battletag" i], [class*="battle-tag" i], [slot="username"], [slot="account-name"]');
    const headers = root.querySelectorAll('header, nav, [role="navigation"], blz-nav, blz-nav-battlenet, blz-navbar, blz-account-menu, blz-profile-menu');
    const parts = navigationRoots.has(root) ? root.querySelectorAll('[part="text"], [part="nav-link-content"], [slot="heading"], [slot="subheading"], [title]') : [];
    for (const node of [...dedicated, ...headers, ...parts]) {
      const inMenu = [...parts].includes(node);
      if (!inMenu && !node.getClientRects().length) continue;
      const text = ((inMenu ? node.textContent : node.innerText) || node.textContent || "").trim().slice(0, 4096).replace(/\s*#\s*(\d{3,12})/g, "#$1");
      for (const tag of text.match(tagPattern) || []) tags.add(tag);
      for (const email of text.match(emailPattern) || []) emails.add(email);
      if ([...dedicated].includes(node) && /^[\p{L}\p{N}\p{M}_-]{2,32}$/u.test(text) && !/^(battletag|account|username|battle\.net)$/i.test(text)) names.add(text);
    }
  }
  const name = tags.size === 1 ? [...tags][0] : tags.size > 1 ? "" : emails.size === 1 ? [...emails][0] : emails.size > 1 ? "" : names.size === 1 ? [...names][0] : "";
  return { source: location.origin + location.pathname, name, email: tags.size > 1 || emails.size !== 1 ? "" : [...emails][0] };
})()
