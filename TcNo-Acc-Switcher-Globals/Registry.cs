// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Runtime.Versioning;
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
        [SupportedOSPlatform("windows")]
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
        [SupportedOSPlatform("windows")]
        public static dynamic ReadRegistryKey(string encodedPath)
        {
            var (rootKey, path, subKey) = ExplodeRegistryKey(encodedPath);

            try
            {
                return rootKey.OpenSubKey(path)?.GetValue(subKey) ?? "ERROR-NULL";
            }
            catch (Exception e)
            {
                WriteToLog("ReadRegistryKey failed", e);
                return "ERROR-READ";
            }
        }

        public static string ByteArrayToString(byte[] ba) => BitConverter.ToString(ba).Replace("-", "");
        public static byte[] StringToByteArray(string hex)
        {
            var numberChars = hex.Length;
            var bytes = new byte[numberChars / 2];
            for (var i = 0; i < numberChars; i += 2)
                bytes[i / 2] = Convert.ToByte(hex.Substring(i, 2), 16);
            return bytes;
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
                if (value.StartsWith("(hex)"))
                {
                    value = value[6..];
                    key?.SetValue(subKey, StringToByteArray(value));
                }
                else
                    key?.SetValue(subKey, value);
            }
            catch (Exception e)
            {
                WriteToLog("SetRegistryKey failed", e);
                return false;
            }

            return true;
        }

        [SupportedOSPlatform("windows")]
        public static bool DeleteRegistryKey(string encodedPath)
        {
            var (rootKey, path, subKey) = ExplodeRegistryKey(encodedPath);

            try
            {
                using var key = rootKey.OpenSubKey(path, true);
                key?.DeleteValue(subKey);
            }
            catch (Exception e)
            {
                WriteToLog("DeleteRegistryKey failed", e);
                return false;
            }

            return true;
        }
        #endregion
    }
}
