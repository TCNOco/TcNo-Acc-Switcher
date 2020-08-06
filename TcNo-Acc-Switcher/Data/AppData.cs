using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using ElectronNET.API.Entities;

namespace TcNo_Acc_Switcher.Data
{

    public class AppData
    {
        private string _windowTitle = "Default window title";

        public string WindowTitle
        {
            get => _windowTitle;
            set
            {
                _windowTitle = value;
                NotifyDataChanged();
                Console.WriteLine("Property changed to: " + _windowTitle);
            }
        }

        public event Action OnChange;

        private void NotifyDataChanged() => OnChange?.Invoke();
    }
}
