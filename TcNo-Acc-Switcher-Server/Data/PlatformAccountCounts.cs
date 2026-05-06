// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
//
// Modern UI helper: counts saved accounts per platform from LoginCache on disk,
// with a short cache so the home-screen render isn't an I/O hot loop.

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Data
{
    public static class PlatformAccountCounts
    {
        private const int CacheSeconds = 5;
        private static readonly object Lock = new();
        private static Dictionary<string, int> _cache = new(StringComparer.OrdinalIgnoreCase);
        private static DateTime _cacheStamp = DateTime.MinValue;

        // Folders under LoginCache/<safe>/ that aren't accounts.
        private static readonly HashSet<string> ReservedDirs = new(StringComparer.OrdinalIgnoreCase)
        {
            "Shortcuts"
        };

        public static int Count(string platformShort)
        {
            if (string.IsNullOrEmpty(platformShort)) return 0;
            EnsureFresh();
            var safe = BasicPlatforms.PlatformSafeName(platformShort);
            return _cache.TryGetValue(safe, out var n) ? n : 0;
        }

        private static void EnsureFresh()
        {
            lock (Lock)
            {
                if ((DateTime.UtcNow - _cacheStamp).TotalSeconds < CacheSeconds && _cache.Count > 0) return;
                _cache = BuildCounts();
                _cacheStamp = DateTime.UtcNow;
            }
        }

        private static Dictionary<string, int> BuildCounts()
        {
            var result = new Dictionary<string, int>(StringComparer.OrdinalIgnoreCase);
            var loginCache = Path.Join(Globals.UserDataFolder, "LoginCache");
            if (!Directory.Exists(loginCache)) return result;

            try
            {
                foreach (var platDir in Directory.EnumerateDirectories(loginCache))
                {
                    var safe = Path.GetFileName(platDir);
                    if (string.IsNullOrEmpty(safe)) continue;
                    int count = 0;
                    try
                    {
                        count = Directory.EnumerateDirectories(platDir)
                            .Count(d => !ReservedDirs.Contains(Path.GetFileName(d) ?? ""));
                    }
                    catch
                    {
                        count = 0;
                    }
                    result[safe] = count;
                }
            }
            catch
            {
                // Disk error — return whatever we got. Empty result is fine.
            }

            return result;
        }
    }
}
