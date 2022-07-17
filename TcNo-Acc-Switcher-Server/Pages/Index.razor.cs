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
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared.Modal;
using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        private static readonly Lang Lang = Lang.Instance;

        public async Task Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            switch (platform)
            {
                case "Steam":
                    if (!await GeneralFuncs.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
                    if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe()))
                    {
                        AppData.CurrentSwitcher = "Steam";
                        ModalFuncs.ShowUpdatePlatformFolderModal();
                        return;
                    }
                    if (SteamSwitcherFuncs.SteamSettingsValid()) AppData.ActiveNavMan.NavigateTo("/Steam/");
                    else await GeneralInvocableFuncs.ShowModal(Lang["Toast_Steam_CantLocateLoginusers"]);
                    break;

                default:
                    if (BasicPlatforms.PlatformExistsFromShort(platform)) // Is a basic platform!
                    {
                        BasicPlatforms.SetCurrentPlatformFromShort(platform);
                        AppData.CurrentSwitcher = CurrentPlatform.FullName;
                        if (!await GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, BasicSettings.ClosingMethod)) return;

                        if (Directory.Exists(BasicSettings.FolderPath) && File.Exists(BasicSettings.Exe()))
                            AppData.ActiveNavMan.NavigateTo("/Basic/");
                        else
                            ModalFuncs.ShowUpdatePlatformFolderModal();
                    }
                    break;
            }
        }
    }
}
