using System.Collections.ObjectModel;
using DiscordRPC;
using TcNo_Acc_Switcher_Server.Shared.Toast;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.StateFuncs;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IAppState
{
    string PasswordCurrent { get; set; }
    ShortcutsState Shortcuts { get; set; }
    Toasts Toasts { get; set; }
    Discord Discord { get; set; }
    Updates Updates { get; set; }
    Stylesheet Stylesheet { get; set; }
    Navigation Navigation { get; set; }
    Switcher Switcher { get; set; }
    WindowState WindowState { get; set; }

    /// <summary>
    /// Is running with the official window, or just the server in a browser.
    /// </summary>
    bool IsTcNoClientApp { get; set; }

    void OpenFolder(string folder);
}