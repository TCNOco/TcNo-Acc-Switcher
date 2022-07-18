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
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared.Modal;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        [Inject] private IBasicPlatforms BasicPlatforms { get; }
        [Inject] private ISteam Steam { get; }
        [Inject] private IBasic Basic { get; }
        [Inject] private ICurrentPlatform CurrentPlatform { get; }

        public async Task Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            if (platform == "Steam")
            {
                if (!await GeneralFuncs.CanKillProcess(Steam.Processes, Steam.ClosingMethod)) return;
                if (!Directory.Exists(Steam.FolderPath) || !File.Exists(Steam.Exe()))
                {
                    AppData.CurrentSwitcher = "Steam";
                    ModalData.ShowUpdatePlatformFolderModal();
                    return;
                }
                if (Steam.LoginUsersVdf() != "RESET_PATH") AppData.NavigateTo("/Steam/");
                else await GeneralFuncs.ShowModal(Lang["Toast_Steam_CantLocateLoginusers"]);
                return;
            }

            AppData.CurrentSwitcher = platform;
            BasicPlatforms.SetCurrentPlatform(platform);
            if (!await GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, Basic.ClosingMethod)) return;

            if (Directory.Exists(Basic.FolderPath) && File.Exists(Basic.Exe()))
                AppData.NavigateTo("/Basic/");
            else
                ModalData.ShowUpdatePlatformFolderModal();
        }
    }
}
