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
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class Toasts : IToasts
{
    private readonly ILang _lang;
    public Toasts(ILang lang)
    {
        _lang = lang;
    }

    // Toast stuff - This is the first attempt at properly using AppData as a Blazor DI Singleton.
    public ObservableCollection<Toast> ToastQueue { get; set; } = new();

    public void ShowToast(ToastType type, string title = "", string message = "", int duration = 5000)
    {
        var toastItem = new Toast(type, title, message);
        Toast existing;
        // If already exists, increment counter and re-add (Cancelling the task).
        if ((existing = ToastQueue.FirstOrDefault(x => x is not null && x.Type == type && x.Title == title && x.Message == message)) is not null)
        {
            // Cancel existing and remove
            existing.CancellationSource.Cancel();
            ToastQueue.Remove(existing);
            // Then add new, with duplicate counter and fresh timer.
            toastItem.DuplicateCount += 1;
        }
        ToastQueue.Add(toastItem);
        toastItem.RemoveSelf = Task.Run(() =>
        {
            Thread.Sleep(duration);
            if (toastItem.Cancellation.IsCancellationRequested) return;
            try
            {
                ToastQueue.Remove(toastItem);
            }
            catch (Exception)
            {
                //
            }
        }, toastItem.Cancellation);
    }

    public void ShowToast(ToastType type, string message, int duration = 5000) =>
        ShowToast(type, "", message, duration);
    public void ShowToastLang(ToastType type, string titleVar, string messageVar, int duration = 5000) =>
        ShowToast(type, _lang[titleVar], _lang[messageVar], duration);
    public void ShowToastLang(ToastType type, string messageVar, int duration = 5000) =>
        ShowToast(type, "", _lang[messageVar], duration);
    public void ShowToastLang(ToastType type, string titleVar, LangSub message, int duration = 5000) =>
        ShowToast(type, _lang[titleVar], _lang[message.LangKey, message.Variable], duration);
    public void ShowToastLang(ToastType type, LangSub message, int duration = 5000) =>
        ShowToast(type, "", _lang[message.LangKey, message.Variable], duration);
}