using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.BattleNet;
using Formatting = System.Xml.Formatting;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherFuncs
    {
        private static readonly Data.Settings.BattleNet BattleNet = Data.Settings.BattleNet.Instance;
        static string BattleNetRoaming;
        static string BattleNetProgramData;
        private static List<BattleNetSwitcherBase.BattleNetUser> Accounts;
        
        /// <summary>
        /// Main function for Steam Account Switcher. Run on load.
        /// Collects accounts from Steam's loginusers.vdf
        /// Prepares images and VAC/Limited status
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            Accounts = new List<BattleNetSwitcherBase.BattleNetUser>();
            
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.LoadProfiles] Loading BattleNet profiles");
            BattleNetRoaming = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Battle.net");
            BattleNetProgramData = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Battle.net");

            string file = File.ReadAllText(BattleNetRoaming + "\\Battle.net.config");
            foreach (string mail in (Newtonsoft.Json.JsonConvert.DeserializeObject(file) as JObject)?.SelectToken("Client.SavedAccountNames")?.ToString()?.Split(','))
            {
                Accounts.Add(new BattleNetSwitcherBase.BattleNetUser() { Email = mail, BTag = BattleNet.BTags.ContainsKey(mail) ? BattleNet.BTags[mail] : null });
                
            }
            foreach (var acc in Accounts)
            {
                var element =
                    $"<input type=\"radio\" id=\"{acc.Email}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{acc.Email}\" class=\"acc\">\r\n" +
                    $"<img src=\"\\img\\icons\\battleNetDefault.jpg\" draggable=\"false\" />\r\n" +
                    $"<h6>{acc.BTag ?? acc.Email}</h6>\r\n";
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });
            }
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
        }
        

        /// <summary>
        /// Restart BattleNet with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        public static void SwapBattleNetAccounts(string accName = "")
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.SwapBattleNetAccounts] Swapping to: {accName}.");
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting BattleNet");
            CloseBattleNet();
            
            BattleNetSwitcherBase.BattleNetUser account = Accounts.First(x => x.Email == accName);           
            // Load settings into JObject
            string file = File.ReadAllText(BattleNetRoaming + "\\Battle.net.config");
            JObject jObject = Newtonsoft.Json.JsonConvert.DeserializeObject(file) as JObject;

            // Select the JToken with the Account Emails
            JToken jToken = jObject?.SelectToken("Client.SavedAccountNames");
            
            // Set the to be logged in Account to idx 0
            Accounts.Remove(account);
            Accounts.Insert(0, account);
            
            // Build the string with the Emails with the EMail thats should get logged in at first
            string replaceString = "";
            for (int i = 0; i < Accounts.Count; i++)
            {
                replaceString += Accounts[i].Email;
                if (i < Accounts.Count - 1)
                {
                    replaceString += ",";
                }
            }
            
            // Replace and write the new Json
            jToken?.Replace(replaceString);
            File.WriteAllText(BattleNetRoaming + "\\Battle.net.config", jObject?.ToString());

            Process.Start(BattleNet.BattleNetExe());
        }

        /// <summary>
        /// Kills Origin processes when run via cmd.exe
        /// </summary>
        public static void CloseBattleNet()
        {
            Globals.KillProcess("Battle.net");
        }

        public static void SetBattleTag(string accName, string bTag)
        {
            Data.Settings.BattleNet.Instance.BTags.Remove(accName);
            Data.Settings.BattleNet.Instance.BTags.Add(accName,bTag);
            File.WriteAllText(Data.Settings.BattleNet.Instance.SettingsFile, Data.Settings.BattleNet.Instance.GetJObject().ToString());
            AppData.ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        }

        public static void DeleteBattleTag(string accName)
        {
            Data.Settings.BattleNet.Instance.BTags.Remove(accName);
            File.WriteAllText(Data.Settings.BattleNet.Instance.SettingsFile, Data.Settings.BattleNet.Instance.GetJObject().ToString());
            AppData.ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        }
    }
}
