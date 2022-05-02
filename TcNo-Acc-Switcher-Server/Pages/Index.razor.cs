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

using System.IO;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;

using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using BattleNetSettings = TcNo_Acc_Switcher_Server.Data.Settings.BattleNet;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        private static readonly Lang Lang = Lang.Instance;

        public void Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            switch (platform)
            {
                case "BattleNet":
                    if (!GeneralFuncs.CanKillProcess(BattleNetSettings.Processes, BattleNetSettings.ClosingMethod)) return;
                    if (Directory.Exists(BattleNetSettings.FolderPath) && File.Exists(BattleNetSettings.Exe())) AppData.ActiveNavMan.NavigateTo("/BattleNet/");
                    else _ = GeneralInvocableFuncs.ShowModal("find:BattleNet:Battle.net.exe:BattleNetSettings");
                    break;

                case "Steam":
                    if (!GeneralFuncs.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
                    if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe()))
                    {
                        _ = GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
                        return;
                    }
                    if (SteamSwitcherFuncs.SteamSettingsValid()) AppData.ActiveNavMan.NavigateTo("/Steam/");
                    else _ = GeneralInvocableFuncs.ShowModal(Lang["Toast_Steam_CantLocateLoginusers"]);
                    break;

                default:
                    if (BasicPlatforms.PlatformExistsFromShort(platform)) // Is a basic platform!
                    {
                        BasicPlatforms.SetCurrentPlatformFromShort(platform);
                        if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, BasicSettings.ClosingMethod)) return;

                        if (Directory.Exists(BasicSettings.FolderPath) && File.Exists(BasicSettings.Exe())) AppData.ActiveNavMan.NavigateTo("/Basic/");
                        else _ = GeneralInvocableFuncs.ShowModal($"find:{CurrentPlatform.SafeName}:{CurrentPlatform.ExeName}:{CurrentPlatform.SafeName}");
                    }
                    break;
            }
        }
    }
}
