using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Steam
{
    class SteamFuncs
    {
        public void StartSteam()
        {

        }
        public void CloseSteam()
        {
            // This is what Administrator permissions are required for.
            var process = new System.Diagnostics.Process();
            var startInfo = new System.Diagnostics.ProcessStartInfo
            {
                WindowStyle = System.Diagnostics.ProcessWindowStyle.Hidden,
                FileName = "cmd.exe",
                Arguments = "/C TASKKILL /F /T /IM steam*"
            };
            process.StartInfo = startInfo;
            process.Start();
            process.WaitForExit();
        }
    }
}
