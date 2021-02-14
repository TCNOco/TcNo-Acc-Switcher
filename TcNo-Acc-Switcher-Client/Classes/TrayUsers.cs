using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Client.Classes
{
    public class TrayUsers
    {
        public class TrayUser
        {
            public string Name { get; set; } = "";
            public string SteamID { get; set; } = "";
            public string AccName { get; set; } = "";
            public string DisplayAs { get; set; } = "";
        }

        public List<TrayUser> ListTrayUsers = new List<TrayUser>();
        public void SaveTrayUsers() => File.WriteAllText("Tray_Users.json", JsonConvert.SerializeObject(ListTrayUsers));

        public void LoadTrayUsers()
        {
            if (!File.Exists("Tray_Users.json"))
                return;
            var json = File.ReadAllText("Tray_Users.json");
            ListTrayUsers = JsonConvert.DeserializeObject<List<TrayUser>>(json);
        }

        public string GetAccName(string steamId)
        {
            // IF AccName == ""  and it's trying to change accounts,
            // Get the account name from the original file --> This is for accounts that are being logged into that aren't in the quick launch ones.
            try
            {
                return ListTrayUsers.Single(r => r.SteamID == steamId)?.AccName;
            }
            catch
            {
                return ""; // Not found
            }
        }

        public string GetName(string steamId) =>
            // IF AccName == ""  and it's trying to change accounts,
            // Get the account name from the original file --> This is for accounts that are being logged into that aren't in the quick launch ones.

            ListTrayUsers.Single(r => r.SteamID == steamId)?.Name;

        public bool AlreadyInList(string steamId) => ListTrayUsers.Count(r => r.SteamID == steamId) != 0;

        public void MoveItemToLast(string steamId)
        {
            var cur = ListTrayUsers.Single(r => r.SteamID == steamId);
            if (ListTrayUsers.IndexOf(cur) == ListTrayUsers.Count - 1) return;
            ListTrayUsers.Remove(cur);
            ListTrayUsers.Add(cur);
        }
    }
}
