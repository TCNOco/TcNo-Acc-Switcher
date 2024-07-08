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

using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region TRAY
        /// <summary>
        /// Adds a user to the tray cache
        /// </summary>
        /// <param name="platform">Platform to switch account on</param>
        /// <param name="arg">Argument to launch and switch</param>
        /// <param name="name">Name to be displayed in the Tray</param>
        /// <param name="maxAccounts">(Optional) Number of accounts to keep and show in tray</param>
        public static void AddTrayUser(string platform, string arg, string name, int maxAccounts)
        {
            var trayUsers = TrayUser.ReadTrayUsers();
            TrayUser.AddUser(ref trayUsers, platform, new TrayUser { Arg = arg, Name = name }, maxAccounts);
            TrayUser.SaveUsers(trayUsers);
        }

        /// <summary>
        /// Removes a user to the tray cache
        /// </summary>
        /// <param name="platform">Platform to switch account on</param>
        /// <param name="name">Name to be displayed in the Tray</param>
        public static void RemoveTrayUser(string platform, string name)
        {
            var trayUsers = TrayUser.ReadTrayUsers();
            TrayUser.RemoveUser(ref trayUsers, platform, name);
            TrayUser.SaveUsers(trayUsers);
        }

        /// <summary>
        /// Removes a user to the tray cache (By argument)
        /// </summary>
        /// <param name="platform">Platform to switch account on</param>
        /// <param name="arg">Argument this account uses to switch</param>
        public static void RemoveTrayUserByArg(string platform, string arg)
        {
            var trayUsers = TrayUser.ReadTrayUsers();
            TrayUser.RemoveUserByArg(ref trayUsers, platform, arg);
            TrayUser.SaveUsers(trayUsers);
        }
        #endregion
    }
    public class TrayUser
    {
        public string Name { get; set; } = ""; // Name to display on list
        public string Arg { get; set; } = "";  // Argument used to switch to this account

        /// <summary>
        /// Reads Tray_Users.json, and returns a dictionary of strings, with a list of TrayUsers attached to them.
        /// </summary>
        /// <returns>Dictionary of keys, and associated lists of tray users</returns>
        public static Dictionary<string, List<TrayUser>> ReadTrayUsers()
        {
            if (!File.Exists("Tray_Users.json")) return new Dictionary<string, List<TrayUser>>();
            var json = Globals.ReadAllText("Tray_Users.json");
            return JsonConvert.DeserializeObject<Dictionary<string, List<TrayUser>>>(json) ?? new Dictionary<string, List<TrayUser>>();
        }

        /// <summary>
        /// Adds a user to the beginning of the [Key]s list of TrayUsers. Moves to position 0 if exists.
        /// </summary>
        /// <param name="trayUsers">Reference to Dictionary of keys & list of TrayUsers</param>
        /// <param name="key">Key to add user to</param>
        /// <param name="newUser">user to add to aforementioned key in dictionary</param>
        /// <param name="maxAccounts">(Optional) Number of accounts to keep and show in tray</param>
        public static void AddUser(ref Dictionary<string, List<TrayUser>> trayUsers, string key, TrayUser newUser, int maxAccounts)
        {
            // Create key and add item if doesn't exist already
            if (!trayUsers.ContainsKey(key))
            {
                trayUsers.Add(key, new List<TrayUser>(new[] { newUser }));
                return;
            }

            // If key contains -> Remove it
            trayUsers[key] = trayUsers[key].Where(x => x.Arg != newUser.Arg).ToList();
            // Add item into first slot
            trayUsers[key].Insert(0, newUser);
            // Shorten list to be a max of 3 (default)
            while (trayUsers[key].Count > maxAccounts) trayUsers[key].RemoveAt(trayUsers[key].Count - 1);
        }

        /// <summary>
        /// Remove user from the list of tray users
        /// </summary>
        /// <param name="trayUsers">Reference to list of TrayUsers to modify</param>
        /// <param name="platform">Platform to switch account on</param>
        /// <param name="name">Name to be displayed in the Tray</param>
        public static void RemoveUser(ref Dictionary<string, List<TrayUser>> trayUsers, string platform, string name)
        {
            // Return if does not have requested platform
            if (!trayUsers.ContainsKey(platform)) return;
            var toRemove = trayUsers[platform].Where(x => x.Name == name).ToList();
            if (toRemove.Count == 0) return;
            foreach (var tu in toRemove)
            {
                _ = trayUsers[platform].Remove(tu);
            }
        }

        /// <summary>
        /// Remove user from the list of tray users (By argument)
        /// </summary>
        /// <param name="trayUsers">Reference to list of TrayUsers to modify</param>
        /// <param name="platform">Platform to switch account on</param>
        /// <param name="arg">Argument this account uses to switch</param>
        public static void RemoveUserByArg(ref Dictionary<string, List<TrayUser>> trayUsers, string platform, string arg)
        {
            // Return if does not have requested platform
            if (!trayUsers.ContainsKey(platform)) return;
            var toRemove = trayUsers[platform].Where(x => x.Arg == arg).ToList();
            if (toRemove.Count == 0) return;
            foreach (var tu in toRemove)
            {
                _ = trayUsers[platform].Remove(tu);
            }
        }

        /// <summary>
        /// Saves trayUsers list to file.
        /// </summary>
        public static void SaveUsers(Dictionary<string, List<TrayUser>> trayUsers) => File.WriteAllText("Tray_Users.json", JsonConvert.SerializeObject(new SortedDictionary<string, List<TrayUser>>(trayUsers)));
    }

}
