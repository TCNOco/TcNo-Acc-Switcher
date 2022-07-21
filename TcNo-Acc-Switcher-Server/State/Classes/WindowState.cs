using System;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class WindowState
    {
        public event Action OnChange;
        public void NotifyDataChanged() => OnChange?.Invoke();



        // Window stuff
        private string _windowTitle = "TcNo Account Switcher";
        public string WindowTitle
        {
            get => _windowTitle;
            set
            {
                _windowTitle = value;
                NotifyDataChanged();
                Globals.WriteToLog($"{Environment.NewLine}Window Title changed to: {value}");
            }
        }

        public bool FirstMainMenuVisit { get; set; }
    }
}
