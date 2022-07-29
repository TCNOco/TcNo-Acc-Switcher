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
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes;

public class Discord
{
    private readonly ILang _lang;
    private readonly IStatistics _statistics;
    private readonly IWindowSettings _windowSettings;

    public Discord(ILang lang, IStatistics statistics, IWindowSettings windowSettings)
    {
        _lang = lang;
        _statistics = statistics;
        _windowSettings = windowSettings;
    }

    public DiscordRpcClient DiscordClient { get; set; }

    public void RefreshDiscordPresenceBackground(bool firstLaunch)
    {
        if (!firstLaunch && DiscordClient.CurrentUser is null) return;
        var dThread = new Thread(RefreshDiscordPresence);
        dThread.Start();
    }

    public void RefreshDiscordPresence()
    {
        Thread.Sleep(1000);

        if (!_windowSettings.DiscordRpcEnabled)
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
        if (_windowSettings.CollectStats && _windowSettings.DiscordRpcEnabled)
        {
            _statistics.GenerateTotals();
            state = _lang["Discord_StatusDetails", new { number = _statistics.SwitcherStats["_Total"].Switches }];
        }


        DiscordClient.SetPresence(new RichPresence
        {
            Details = _lang["Discord_Status"],
            State = state,
            Timestamps = timestamp,
            Buttons = new Button[]
            { new() {
                Url = "https://github.com/TcNobo/TcNo-Acc-Switcher/",
                Label = _lang["Website"]
            }},
            Assets = new Assets
            {
                LargeImageKey = "switcher",
                LargeImageText = "TcNo Account Switcher"
            }
        });

        Thread.Sleep(10000);
        RefreshDiscordPresence();
    }
}