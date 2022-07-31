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

using System.Collections.ObjectModel;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IToasts
{
    ObservableCollection<Toast> ToastQueue { get; set; }
    void ShowToast(ToastType type, string title = "", string message = "", int duration = 5000);
    void ShowToast(ToastType type, string message, int duration = 5000);
    void ShowToastLang(ToastType type, string titleVar, string messageVar, int duration = 5000);
    void ShowToastLang(ToastType type, string messageVar, int duration = 5000);
    void ShowToastLang(ToastType type, string titleVar, LangSub message, int duration = 5000);
    void ShowToastLang(ToastType type, LangSub message, int duration = 5000);

    /// <summary>
    /// Update toasts, with a tiny delay to allow UI to update.
    /// </summary>
    Task ShowToastAsync(ToastType type, string message, int duration = 5000);

    /// <summary>
    /// Update toasts, with a tiny delay to allow UI to update.
    /// </summary>
    Task ShowToastLangAsync(ToastType type, string titleVar, string messageVar, int duration = 5000);

    /// <summary>
    /// Update toasts, with a tiny delay to allow UI to update.
    /// </summary>
    Task ShowToastLangAsync(ToastType type, string messageVar, int duration = 5000);

    /// <summary>
    /// Update toasts, with a tiny delay to allow UI to update.
    /// </summary>
    Task ShowToastLangAsync(ToastType type, string titleVar, LangSub message, int duration = 5000);

    /// <summary>
    /// Update toasts, with a tiny delay to allow UI to update.
    /// </summary>
    Task ShowToastLangAsync(ToastType type, LangSub message, int duration = 5000);
}