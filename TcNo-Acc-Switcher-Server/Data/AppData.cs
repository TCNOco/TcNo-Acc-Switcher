using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection.Metadata.Ecma335;
using System.Threading.Tasks;
using Microsoft.JSInterop;

namespace TcNo_Acc_Switcher_Server.Data
{

    public class AppData
    {
        // Window stuff
        private string _windowTitle = "Default window title";

        public string WindowTitle
        {
            get => _windowTitle;
            set
            {
                _windowTitle = value;
                NotifyDataChanged();
                Console.WriteLine("Window Title changed to: " + _windowTitle);
            }
        }

        private string _currentStatus = "Status: Initializing";
        public string CurrentStatus
        {
            get => _currentStatus;
            set
            {
                _currentStatus = value;
                NotifyDataChanged();
                Console.WriteLine("CurrentStatus changed to: " + _windowTitle);
            }
        }
        public void SetCurrentStatus(string status) => CurrentStatus = status;

        public event Action OnChange;

        private void NotifyDataChanged() => OnChange?.Invoke();
    }
}
