using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherBase
    {
        /// <summary>
        /// Converts input SteamID64 into the requested format, then copies it to clipboard.
        /// </summary>
        /// <param name="request">SteamId, SteamId3, SteamId32, SteamId64</param>
        /// <param name="steamId64"></param>
        [JSInvokable]
        public static void CopySteamIdType(string request, string steamId64)
        {
            switch (request)
            {
                case "SteamId":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(steamId64).Id);
                    break;
                case "SteamId3":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(steamId64).Id3);
                    break;
                case "SteamId32":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(steamId64).Id32);
                    break;
                case "SteamId64":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(steamId64).Id64);
                    break;
            }
        }

        /// <summary>
        /// JS function handler for swapping to another Steam account.
        /// </summary>
        /// <param name="steamId">Requested account's SteamID</param>
        /// <param name="accName">Requested account's Login Username</param>
        [JSInvokable]
        public static void SwapTo(string steamId, string accName)
        {
            SteamSwitcherFuncs.SwapSteamAccounts( steamId, accName);
        }

        /// <summary>
        /// JS function handler for swapping to a new Steam account (No inputs)
        /// </summary>
        [JSInvokable]
        public static async void NewSteamLogin()
        {
            SteamSwitcherFuncs.SwapSteamAccounts();
        }
    }
}
