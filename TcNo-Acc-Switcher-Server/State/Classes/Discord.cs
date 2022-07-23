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

using System.Threading;
using DiscordRPC;
using DiscordRPC.Logging;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes;

public class Discord
{
    [Inject] public ILang Lang { get; set; }
    [Inject] public IWindowSettings WindowSettings { get; set; }

    public DiscordRpcClient DiscordClient { get; set; }

    public void RefreshDiscordPresenceAsync(bool firstLaunch)
    {
        if (!firstLaunch && DiscordClient.CurrentUser is null) return;
        var dThread = new Thread(RefreshDiscordPresence);
        dThread.Start();
    }

    public void RefreshDiscordPresence()
    {
        Thread.Sleep(1000);

        if (!WindowSettings.DiscordRpcEnabled)
        {
            if (DiscordClient != null)
            {
                if (!DiscordClient.IsInitialized) return;
                DiscordClient.Deinitialize();
                DiscordClient = null;
            }

            return;
        }

        var timestamp = Timestamps.Now;

        DiscordClient ??= new DiscordRpcClient("973188269405765682")
        {
            Logger = new ConsoleLogger { Level = LogLevel.None },
        };
        if (!DiscordClient.IsInitialized) DiscordClient.Initialize();
        else timestamp = DiscordClient.CurrentPresence.Timestamps;

        var state = "";
        if (WindowSettings.CollectStats && WindowSettings.DiscordRpcEnabled)
        {
            AppStats.GenerateTotals();
            state = Lang["Discord_StatusDetails", new { number = AppStats.SwitcherStats["_Total"].Switches }];
        }


        DiscordClient.SetPresence(new RichPresence
        {
            Details = Lang["Discord_Status"],
            State = state,
            Timestamps = timestamp,
            Buttons = new Button[]
            { new() {
                Url = "https://github.com/TcNobo/TcNo-Acc-Switcher/",
                Label = Lang["Website"]
            }},
            Assets = new Assets
            {
                LargeImageKey = "switcher",
                LargeImageText = "TcNo Account Switcher"
            }
        });
    }
}