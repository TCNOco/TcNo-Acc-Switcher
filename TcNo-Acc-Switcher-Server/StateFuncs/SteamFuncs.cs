using System.IO;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.StateFuncs
{
    public class SteamFuncs
    {
        [Inject] private SteamSettings SteamSettings { get; set; }
        [Inject] private Modals Modals { get; set; }
        [Inject] private IAppState AppState { get; set; }
        [Inject] private Toasts Toasts { get; set; }
        [Inject] private NewLang Lang { get; set; }


    }

}
