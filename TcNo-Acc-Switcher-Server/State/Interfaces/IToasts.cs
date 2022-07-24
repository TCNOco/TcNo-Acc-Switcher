using System.Collections.ObjectModel;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State;

public interface IToasts
{
    ObservableCollection<Toast> ToastQueue { get; set; }
    void ShowToast(ToastType type, string title = "", string message = "", int duration = 5000);
    void ShowToast(ToastType type, string message, int duration = 5000);
    void ShowToastLang(ToastType type, string titleVar, string messageVar, int duration = 5000);
    void ShowToastLang(ToastType type, string messageVar, int duration = 5000);
    void ShowToastLang(ToastType type, string titleVar, LangSub message, int duration = 5000);
    void ShowToastLang(ToastType type, LangSub message, int duration = 5000);
}