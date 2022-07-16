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

using System.Diagnostics;
using System.IO;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherBase
    {
        private static readonly Lang Lang = Lang.Instance;

        public static void SteamOpenUserdata()
        {
            var steamId32 = new SteamIdConvert(AppData.SelectedAccountId);
            var folder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32.Id32}");
            if (Directory.Exists(folder)) _ = Process.Start("explorer.exe", folder);
            else _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindSteamUserdata"], Lang["Failed"], "toastarea");
        }
    }
}
