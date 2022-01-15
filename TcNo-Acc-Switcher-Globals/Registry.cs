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

using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.Versioning;
using System.Text;
using System.Threading.Tasks;
using Microsoft.Win32;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region REGISTRY
        [SupportedOSPlatform("windows")]
        public static RegistryKey ExpandRegistryAbbreviation(string abv)
        {
            return abv switch
            {
                "HKCR" => Registry.ClassesRoot,
                "HKCU" => Registry.CurrentUser,
                "HKLM" => Registry.LocalMachine,
                "HKCC" => Registry.CurrentConfig,
                "HKPD" => Registry.PerformanceData,
                _ => null
            };
        }

        /// <summary>
        /// Break an encoded registry key into it's separate parts
        /// </summary>
        /// <param name="encodedPath">HKXX\\path:SubKey</param>
        private static (RegistryKey, string, string) ExplodeRegistryKey(string encodedPath)
        {
            var rootKey = ExpandRegistryAbbreviation(encodedPath[..4]); // Get HKXX
            encodedPath = encodedPath[5..]; // Remove HKXX\\
            var path = encodedPath.Split(":")[0];
            var subKey = encodedPath.Split(":")[1];

            return (rootKey, path, subKey);
        }

        /// <summary>
        /// Read the value of a Registry key (Requires special path)
        /// </summary>
        /// <param name="encodedPath">HKXX\\path:SubKey</param>
        public static string ReadRegistryKey(string encodedPath)
        {
            var (rootKey, path, subKey) = ExplodeRegistryKey(encodedPath);
            var currentAccountId = "";

            try
            {
                currentAccountId = (string)rootKey.OpenSubKey(path)?.GetValue(subKey);
            }
            catch (Exception)
            {
                // TODO: WRITE ERRORS TO LOGS!
                return "ERROR-READ";
            }
            return currentAccountId ?? "ERROR-NULL";
        }

        /// <summary>
        /// Sets the value of a Registry key (Requires special path)
        /// </summary>
        /// <param name="encodedPath">HKXX\\path:subkey</param>
        /// <param name="value">Value, or empty to "clear"</param>
        [SupportedOSPlatform("windows")]
        public static bool SetRegistryKey(string encodedPath, string value = "")
        {
            var (rootKey, path, subKey) = ExplodeRegistryKey(encodedPath);

            try
            {
                using var key = rootKey.CreateSubKey(path);
                key?.SetValue(subKey, value);
            }
            catch (Exception e)
            {
                WriteToLog("SetRegistryKey failed", e);
                return false;
            }

            return true;
        }
        #endregion
    }
}
