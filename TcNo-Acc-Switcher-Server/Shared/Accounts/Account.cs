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

using System.Collections.Generic;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts;

public class Account
{
    public bool IsChecked { get; set; } = false;
    public bool IsCurrent { get; set; } = false;
    public string TitleText { get; set; } = "";
    public string Platform { get; set; } = "";
    public string ImagePath { get; set; } = "";
    public string DisplayName { get; set; } = "";
    public string LoginUsername { get; set; } = "";
    public string SafeDisplayName => GeneralFuncs.EscapeText(DisplayName);
    public string AccountId { get; set; } = "";
    public string Line0 { get; set; } = "";
    public string Line2 { get; set; } = "";
    public string Line3 { get; set; } = "";
    public Dictionary<string, Dictionary<string, BasicStats.StatValueAndIcon>> UserStats { get; set; }

    public void SetUserStats(string platform)
    {
        UserStats = BasicStats.GetUserStatsAllGamesMarkup(platform, AccountId);
    }
    public ClassCollection Classes { get; set; } = new();

    // Optional
    public class ClassCollection
    {
        public string Label { get; set; } = "";
        public string Image { get; set; } = "";
        public string Line0 { get; set; } = "";
        public string Line2 { get; set; } = "";
        public string Line3 { get; set; } = "";
    }
}