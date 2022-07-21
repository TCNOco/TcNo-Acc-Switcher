using System.Threading;
using DiscordRPC;
using DiscordRPC.Logging;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class Discord
    {
        [Inject] public INewLang Lang { get; set; }
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
}
