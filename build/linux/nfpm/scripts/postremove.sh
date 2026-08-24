#!/bin/sh

# The icon files are gone by now, but the caches still list them. Leaving them
# listed makes menus draw a broken icon until something else rebuilds them.
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q -f -t /usr/share/icons/hicolor 2>/dev/null || true
fi

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications
fi

exit 0
