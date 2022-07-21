using System;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using DiscordRPC;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared.Toast;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Interfaces;
using TcNo_Acc_Switcher_Server.StateFuncs;

namespace TcNo_Acc_Switcher_Server.State
{
    public class AppState : IAppState
    {
        public string PasswordCurrent { get; set; }

        public ShortcutsState Shortcuts { get; set; } = new();

        public Notifications Notifications { get; set; } = new();

        public Discord Discord { get; set; } = new();

        public AppState()
        {
            // Discord integration
            Discord.RefreshDiscordPresenceAsync(true);

        }
    }
}
