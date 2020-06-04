using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_SteamTray
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
        public void SaveTrayUsers()
        {
            File.WriteAllText("Tray_Users.json", JsonConvert.SerializeObject(ListTrayUsers));
        }

        public void LoadTrayUsers()
        {
            if (!File.Exists("Tray_Users.json"))
                return;
            string json = File.ReadAllText("Tray_Users.json");
            ListTrayUsers = JsonConvert.DeserializeObject<List<TrayUser>>(json);
        }

        public string GetAccName(string SteamID)
        {
            // IF AccName == ""  and it's trying to change accounts,
            // Get the account name from the original file --> This is for accounts that are being logged into that aren't in the quick launch ones.

            return ListTrayUsers.Single(r => r.SteamID == SteamID)?.AccName;
        }

        public bool AlreadyInList(string SteamID)
        {
            return ListTrayUsers.Count(r => r.SteamID == SteamID) != 0;
        }

        public void MoveItemToLast(string SteamID)
        {
            TrayUser cur = ListTrayUsers.Single(r => r.SteamID == SteamID);
            if (ListTrayUsers.IndexOf(cur) != ListTrayUsers.Count - 1)
            {
                ListTrayUsers.Remove(cur);
                ListTrayUsers.Add(cur);
            }
        }
    }

}