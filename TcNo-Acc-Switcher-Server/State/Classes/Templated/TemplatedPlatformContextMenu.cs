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
using System.IO;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes.Templated;

public class TemplatedPlatformContextMenu
{
    [Inject] private Lang Lang { get; set; }
    [Inject] private Modals Modals { get; set; }
    [Inject] IAppState AppState { get; set; }
    [Inject] Toasts Toasts { get; set; }
    [Inject] TemplatedPlatformState TemplatedPlatformState { get; set; }
    [Inject] TemplatedPlatformFuncs TemplatedPlatformFuncs { get; set; }
    [Inject] State.GameStats GameStats { get; set; }
    [Inject] IWindowSettings WindowSettings { get; set; }


    public readonly ObservableCollection<MenuItem> ContextMenuItems = new();
    private void BuildContextMenu()
    {
        ContextMenuItems.Clear();
        ContextMenuItems.AddRange(new MenuBuilder(
            new[]
            {
                new ("Context_SwapTo", new Action(() => TemplatedPlatformFuncs.SwapToAccount())),
                new ("Context_CopyUsername", new Action(async () => await StaticFuncs.CopyText(AppState.Switcher.SelectedAccount.DisplayName))),
                new ("Context_ChangeName", new Action(Modals.ShowChangeUsernameModal)),
                new ("Context_CreateShortcut", new Action(() => CreateShortcut())),
                new ("Context_ChangeImage", new Action(Modals.ShowChangeAccImageModal)),
                new ("Forget", new Action(async () => await TemplatedPlatformFuncs.ForgetAccount())),
                new ("Notes", new Action(() => Modals.ShowModal("notes"))),
                GameStats.PlatformHasAnyGames(TemplatedPlatformState.CurrentPlatform.SafeName) ?
                    new Tuple<string, object>("Context_ManageGameStats", new Action(Modals.ShowGameStatsSelectorModal)) : null,
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



    /// <summary>
    /// Creates a shortcut to start the Account Switcher, and swap to the account related.
    /// </summary>
    /// <param name="args">(Optional) arguments for shortcut</param>
    public void CreateShortcut(string args = "")
    {
        if (!OperatingSystem.IsWindows()) return;
        Globals.DebugWriteLine(@"[JSInvoke:General\GeneralInvocableFuncs.CreateShortcut]");
        if (args.Length > 0 && args[0] != ':') args = $" {args}"; // Add a space before arguments if doesn't start with ':'
        var bgImg = Path.Join(Globals.WwwRoot, $"\\img\\platform\\{AppState.Switcher.CurrentSwitcherSafe}.svg");
        var currentPlatformImgPath = Path.Join(Globals.WwwRoot, $"\\img\\platform\\{TemplatedPlatformState.CurrentPlatform.SafeName}.svg");
        var currentPlatformImgPathOverride = Path.Join(Globals.WwwRoot, $"\\img\\platform\\{TemplatedPlatformState.CurrentPlatform.SafeName}.png");
        var primaryPlatformId = TemplatedPlatformState.CurrentPlatform.PrimaryId;
        var platformName = $"Switch to {AppState.Switcher.SelectedAccount.DisplayName} [{AppState.Switcher.CurrentSwitcher}]";

        if (File.Exists(currentPlatformImgPathOverride))
            bgImg = currentPlatformImgPathOverride;
        else if (File.Exists(currentPlatformImgPath))
            bgImg = currentPlatformImgPath;
        else if (File.Exists(Path.Join(Globals.WwwRoot, "\\img\\BasicDefault.png")))
            bgImg = Path.Join(Globals.WwwRoot, "\\img\\BasicDefault.png");


        var fgImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{AppState.Switcher.CurrentSwitcherSafe}\\{AppState.Switcher.SelectedAccountId}.jpg");
        if (!File.Exists(fgImg)) fgImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{AppState.Switcher.CurrentSwitcherSafe}\\{AppState.Switcher.SelectedAccountId}.png");
        if (!File.Exists(fgImg))
        {
            Toasts.ShowToastLang(ToastType.Error, "Toast_CantCreateShortcut", "Toast_CantFindImage");
            return;
        }

        var s = new Shortcut();
        _ = s.Shortcut_Platform(
            Shortcut.Desktop,
            platformName,
            $"+{primaryPlatformId}:{AppState.Switcher.SelectedAccountId}{args}",
            $"Switch to {AppState.Switcher.SelectedAccount.DisplayName} [{AppState.Switcher.CurrentSwitcher}] in TcNo Account Switcher",
            true);
        if (s.CreateCombinedIcon(bgImg, fgImg, $"{AppState.Switcher.SelectedAccountId}.ico"))
        {
            s.TryWrite();

            if (AppState.Stylesheet.StreamerModeTriggered)
                Toasts.ShowToastLang(ToastType.Success, "Success", "Toast_ShortcutCreated");
            else
                Toasts.ShowToastLang(ToastType.Success, "Toast_ShortcutCreated", new LangSub("ForName", new { name = AppState.Switcher.SelectedAccount.DisplayName }));
        }
        else
            Toasts.ShowToastLang(ToastType.Error, "Toast_FailedCreateIcon");
    }
}