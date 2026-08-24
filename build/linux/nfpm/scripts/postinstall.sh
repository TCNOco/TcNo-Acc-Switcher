#!/bin/sh

# Update desktop database for .desktop file changes
# This makes the application appear in application menus and registers its capabilities.
if command -v update-desktop-database >/dev/null 2>&1; then
  echo "Updating desktop database..."
  update-desktop-database -q /usr/share/applications
else
  echo "Warning: update-desktop-database command not found. Desktop file may not be immediately recognized." >&2
fi

# Refresh the icon theme cache. Without this the icon named by the desktop
# entry's Icon= key can stay unresolvable until something else rebuilds the
# cache, and the window falls back to the compositor's placeholder icon - a
# stale cache is preferred over a directory scan, so newly installed icons are
# simply not seen.
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  echo "Updating icon cache..."
  gtk-update-icon-cache -q -f -t /usr/share/icons/hicolor 2>/dev/null || true
else
  echo "Warning: gtk-update-icon-cache not found. The application icon may not appear until the icon cache is rebuilt." >&2
fi

# Update MIME database for custom URL schemes (x-scheme-handler)
# This ensures the system knows how to handle your custom protocols.
if command -v update-mime-database >/dev/null 2>&1; then
  echo "Updating MIME database..."
  update-mime-database -n /usr/share/mime
else
  echo "Warning: update-mime-database command not found. Custom URL schemes may not be immediately recognized." >&2
fi

exit 0
