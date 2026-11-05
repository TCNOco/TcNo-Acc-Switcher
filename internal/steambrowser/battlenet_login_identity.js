(() => {
  if (window.top !== window || !/^https:\/\/(?:(?:eu|us|kr|tw|cn)\.)?account\.battle\.net$/.test(location.origin) || !/^\/login\/(?:[a-z]{2}(?:-[a-z]{2})?\/)?(?:password\/?)?$/.test(location.pathname)) return null;
  const emails = new Set();
  const pattern = /[A-Za-z0-9.!#$%&'*+\/=?^_`{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+/g;
  for (const input of document.querySelectorAll('input#accountName, input[name="accountName"], input[name="email"], input[type="email"]')) {
    if (!["text", "email", "tel"].includes(input.type)) continue;
    const value = input.value.trim();
    if (value.length <= 320 && value.match(pattern)?.[0] === value && !value.includes("*")) emails.add(value);
  }
  const wrapper = document.querySelector("#login-wrapper, .login-wrapper");
  for (const email of ((wrapper?.innerText || "").slice(0,8192).match(pattern) || [])) {
    if (!email.includes("*")) emails.add(email);
  }
  return {source: location.origin + location.pathname, name: "", email: emails.size === 1 ? [...emails][0] : ""};
})()
