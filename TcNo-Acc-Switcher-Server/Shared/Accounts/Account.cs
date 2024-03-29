﻿using System;
using System.Collections.Generic;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts;

public class Account
{
    private static readonly Lang Lang = Lang.Instance;
    public string Platform { get; set; } = "";
    public string ImagePath { get; set; } = "";
    public string DisplayName { get; set; } = "";
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