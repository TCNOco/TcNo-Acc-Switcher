using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Clients
{
    public class SoftwareFuncs
    {

    }

    public class FindClients
    {
        public static SoftwareClient FindSteam()
        {
            SoftwareClient steamClient = new SoftwareClient
            {
                Name = "Steam",
                IconPath = ClientIcons.SteamIconPath
            };

            string[] commonLocations =
                {"C:\\Program Files\\Steam\\Steam.exe", "C:\\Program Files (x86)\\Steam\\Steam.exe"};

            foreach (var commonLocation in commonLocations)
            {
                if (File.Exists(commonLocation))
                {
                    steamClient.Exists = true;
                    steamClient.ExeLocation = commonLocation;
                }
            }

            return steamClient;
        }
    }
}
