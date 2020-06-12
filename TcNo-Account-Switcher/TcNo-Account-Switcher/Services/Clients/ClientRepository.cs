using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Clients
{
    class ClientRepository : IClientRepository
    {
        public async Task<List<SoftwareClient>> GetSoftwareClientsAsync()
        {
            // Search common locations for .exe's.
            // Example: C:\Program Files\Steam\Steam.exe
            // Add that to the list, then return list.
            List<SoftwareClient> SoftwareClients = new List<SoftwareClient>();
            await Task.Run(() =>
            {
                string steamLocation = FindClients.FindSteam();
                if (steamLocation != "")
                {
                    SoftwareClients.Add(new SoftwareClient() {ExeLocation = steamLocation });
                }
            });
            return SoftwareClients;
        }

        public async Task<SoftwareClient> AddSoftwareClient(string selectedExe)
        {
            // Check if the .exe selected is a known .exe
            // If it is, then update the entry on the list.
            SoftwareClient sc = new SoftwareClient();
            await Task.Run(() => { });
            return sc;
        }
        public async Task<SoftwareClient> UpdateSoftwareClient(string name, string selectedExe){
            // Update program with the new .exe location
            SoftwareClient sc = new SoftwareClient();
            await Task.Run(() => { });
            return sc;

        }
    }
}
