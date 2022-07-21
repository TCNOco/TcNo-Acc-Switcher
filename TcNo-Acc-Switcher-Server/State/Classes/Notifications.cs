using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class Notifications
    {
        [Inject] private NewLang Lang { get; set; }

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
            ShowToast(type, Lang[titleVar], Lang[messageVar], duration);
        public void ShowToastLang(ToastType type, string messageVar, int duration = 5000) =>
            ShowToast(type, "", Lang[messageVar], duration);
        public void ShowToastLang(ToastType type, string titleVar, LangSub message, int duration = 5000) =>
            ShowToast(type, Lang[titleVar], Lang[message.LangKey, message.Variable], duration);
        public void ShowToastLang(ToastType type, LangSub message, int duration = 5000) =>
            ShowToast(type, "", Lang[message.LangKey, message.Variable], duration);
    }
}
