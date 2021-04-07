// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

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
        /// <param name="ePersonaState">(Optional) Persona State [0: Offline, 1: Online...]</param>
        [JSInvokable]
        public static void SwapTo(string steamId, string accName, int ePersonaState = -1)
        {
            SteamSwitcherFuncs.SwapSteamAccounts(steamId, accName, ePersonaState: ePersonaState);
        }

        /// <summary>
        /// Creates a shortcut to start the Account Switcher, and swap to the account related to provided SteamID.
        /// </summary>
        /// <param name="steamId">SteamID of account to swap to</param>
        /// <param name="accName">Account name of account to swap to</param>
        [JSInvokable]
        public static void CreateShortcut(string steamId, string accName)
        {
            var s = new Shortcut();
            s.Shortcut_Steam(Shortcut.Desktop, $"Switch to {accName}.lnk", $"Switch to {accName} in TcNo Account Switcher", $"+s:{steamId}");
            s.CreateCombinedIcon(
                Path.Join(GeneralFuncs.WwwRoot, "\\prog_icons\\steam.ico"),
                Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\{steamId}.jpg"), 
                $"{steamId}.ico");
            s.TryWrite();
        }

        /// <summary>
        /// JS function handler for swapping to a new Steam account (No inputs)
        /// </summary>
        [JSInvokable]
        public static void NewSteamLogin()
        {
            SteamSwitcherFuncs.SwapSteamAccounts();
        }
    }
}
