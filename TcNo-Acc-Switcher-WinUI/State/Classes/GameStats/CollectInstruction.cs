// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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

namespace TcNo_Acc_Switcher.State.Classes.GameStats;

public class CollectInstruction
{
    public string XPath { get; set; }
    public string Select { get; set; }
    public string DisplayAs { get; set; } = "%x%";
    public string ToggleText { get; set; } = "";

    // Optional
    public string SelectAttribute { get; set; } = ""; // If Select = "attribute", set the attribute to get here.
    public string SpecialType { get; set; } = ""; // Possible types: ImageDownload.
    public string NoDisplayIf { get; set; } = ""; // The DisplayAs text will not display if equal to the default value of this.
    public string Icon { get; set; } = ""; // Icon HTML markup
}