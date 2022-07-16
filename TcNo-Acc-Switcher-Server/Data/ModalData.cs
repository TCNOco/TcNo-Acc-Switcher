using System;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class ModalData
    {
        private static readonly Lang Lang = Lang.Instance;
        private static ModalData _instance = new();

        private static readonly object LockObj = new();

        public static ModalData Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new ModalData();
                }
            }
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }
        public event Action OnChange;
        public void NotifyDataChanged() => OnChange?.Invoke();

        private bool _isShown;
        public static bool IsShown
        {
            get => Instance._isShown;
            set
            {
                Instance._isShown = value;
                _ = AppData.InvokeVoidAsync(value ? "showModal" : "hideModal");
                Instance.NotifyDataChanged();
            }
        }

        private string _type;

        public static string Type
        {
            get => Instance._type;
            set
            {
                Instance._type = value;
                Instance.NotifyDataChanged();
            }
        }

        private string _extraArgs;

        public static string ExtraArgs { get => Instance._extraArgs; set => Instance._extraArgs = value; }

        private string _title;

        public static string Title
        {
            get => Instance._title;
            set
            {
                Instance._title = value;
                Instance.NotifyDataChanged();
            }
        }




        public static void ShowModal(string type)
        {
            if (type.Contains(":"))
            {
                Type = type.Split(":")[0];
                ExtraArgs = type.Split(":")[1];
                IsShown = true;
                return;
            }

            Type = type;
            IsShown = true;
        }
    }
}
