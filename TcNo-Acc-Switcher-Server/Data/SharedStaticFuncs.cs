using System.Linq;
using System.Runtime.Versioning;
using Microsoft.Win32;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class SharedStaticFuncs
    {

        /// <summary>
        /// Check for existence of protocol key in registry (tcno:\\)
        /// </summary>
        [SupportedOSPlatform("windows")]
        public static bool Protocol_IsEnabled()
        {
            var key = Registry.ClassesRoot.OpenSubKey(@"tcno");
            return key != null && (key.GetValueNames().Contains("URL Protocol"));
        }
    }
}
