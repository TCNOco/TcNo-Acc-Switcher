// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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

using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Settings
    {
        private static readonly Lang Lang = Lang.Instance;
        protected override void OnInitialized()
        {
            AppData.Instance.WindowTitle = Lang["Title_Settings"];
            Globals.DebugWriteLine(@"[Auto:Settings.razor.cs.OnInitializedAsync]");
        }
    }
}
