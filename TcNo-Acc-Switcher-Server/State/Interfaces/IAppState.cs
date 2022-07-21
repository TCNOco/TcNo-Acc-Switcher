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
}