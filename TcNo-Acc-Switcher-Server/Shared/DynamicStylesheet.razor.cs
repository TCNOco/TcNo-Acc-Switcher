// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Special thanks to iR3turnZ for contributing to this platform's account switcher
// iR3turnZ: https://github.com/HoeblingerDaniel

using System;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Shared
{
    public partial class DynamicStylesheet
    {
        public string GetStylesheetMarkupString()
        {
            var style = AppSettings.Stylesheet;

            if (OperatingSystem.IsWindows() && AppSettings.WindowsAccent)
            {
                var start = style.IndexOf("--accent:", StringComparison.Ordinal);
                var end = style.IndexOf(";", start, StringComparison.Ordinal) - start;
                style = style.Replace(style.Substring(start, end), "");

                var (h, s, l) = AppSettings.WindowsAccentColorHsl;
                var (r, g, b) = AppSettings.WindowsAccentColorInt;
                style = $":root {{ --accentHS: {h}, {s}%; --accentL: {l}%; --accent: {AppSettings.WindowsAccentColor}; --accentInt: {r}, {g}, {b}}}\n\n; " + style;
            }

            if (AppSettings.Rtl)
                style = "@import url(css/rtl.min.css);\n" + style;

            if (AppSettings.Background != "")
                style += ".programMain {background: url(" + AppSettings.Background + ")!important;background-size:cover!important;}";

            return style;
        }
    }
}
