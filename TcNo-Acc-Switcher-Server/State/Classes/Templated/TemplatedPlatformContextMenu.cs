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


using System;
using System.Collections.ObjectModel;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes.Templated
{
    public class TemplatedPlatformContextMenu
    {
        [Inject] private NewLang Lang { get; set; }
        [Inject] private Modals Modals { get; set; }
        [Inject] IAppState AppState { get; set; }
        [Inject] Toasts Toasts { get; set; }
        [Inject] TemplatedPlatformState TemplatedPlatformState { get; set; }


        public readonly ObservableCollection<MenuItem> ContextMenuItems = new();
        private void BuildContextMenu()
        {
            ContextMenuItems.Clear();
            ContextMenuItems.AddRange(new MenuBuilder(
                new[]
                {
                    new ("Context_SwapTo", new Action(async () => await TemplatedPlatformState.SwapToAccount())),
                    new ("Context_CopyUsername", new Action(async () => await AppFuncs.CopyText(AppState.Switcher.SelectedAccount.DisplayName))),
                    new ("Context_ChangeName", new Action(Modals.ShowChangeUsernameModal)),
                    new ("Context_CreateShortcut", new Action(async () => await GeneralInvocableFuncs.CreateShortcut())),
                    new ("Context_ChangeImage", new Action(Modals.ShowChangeAccImageModal)),
                    new ("Forget", new Action(async () => await AppFuncs.ForgetAccount())),
                    new ("Notes", new Action(() => Modals.ShowModal("notes"))),
                    BasicStats.PlatformHasAnyGames(CurrentPlatform.SafeName) ?
                        new Tuple<string, object>("Context_ManageGameStats", new Action(ModalData.ShowGameStatsSelectorModal)) : null,
                }).Result());
        }

        public readonly ObservableCollection<MenuItem> ContextMenuShortcutItems = new MenuBuilder(
            new Tuple<string, object>[]
            {
                new ("Context_RunAdmin", "shortcut('admin')"),
                new ("Context_Hide", "shortcut('hide')"),
            }).Result();

        public readonly ObservableCollection<MenuItem> ContextMenuPlatformItems = new MenuBuilder(
            new Tuple<string, object>("Context_RunAdmin", "shortcut('admin')")
        ).Result();


    }
}
