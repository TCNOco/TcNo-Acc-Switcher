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
            public string SteamId { get; set; } = "";
            // Never used
            // => public string AccName { get; set; } = "";
            public string DisplayAs { get; set; } = "";
        }

        public List<TrayUser> ListTrayUsers = new List<TrayUser>();

        public void LoadTrayUsers()
        {
            if (!File.Exists("Tray_Users.json"))
                return;
            var json = File.ReadAllText("Tray_Users.json");
            ListTrayUsers = JsonConvert.DeserializeObject<List<TrayUser>>(json);
        }


        public bool AlreadyInList(string steamId) => ListTrayUsers.Count(r => r.SteamId == steamId) != 0;

        // Never used
        // => public void SaveTrayUsers() => File.WriteAllText("Tray_Users.json", JsonConvert.SerializeObject(ListTrayUsers));
        // => IF AccName == ""  and it's trying to change accounts,
        //    Get the account name from the original file --> This is for accounts that are being logged into that aren't in the quick launch ones.
        //    public string GetAccName(string steamId) => ListTrayUsers.Single(r => r.SteamId == steamId)?.AccName;
        // => public void MoveItemToLast(string steamId)
        //    {
        //        var cur = ListTrayUsers.Single(r => r.SteamId == steamId);
        //        if (ListTrayUsers.IndexOf(cur) == ListTrayUsers.Count - 1) return;
        //        ListTrayUsers.Remove(cur);
        //        ListTrayUsers.Add(cur);
        //    }
    }

}