using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Clients
{
    public class SoftwareClient
    {
        public SoftwareClient()
        {
            // Defaults
            RunAsAdmin = false;
            ExeLocation = "";
            Exists = false;
        }
        public string Name { get; set; } // Example: Steam
        public bool Exists { get; set; }
        public string IconPath { get; set; }
        public string ExeLocation { get; set; }
        public bool RunAsAdmin { get; set; }
    }
}
