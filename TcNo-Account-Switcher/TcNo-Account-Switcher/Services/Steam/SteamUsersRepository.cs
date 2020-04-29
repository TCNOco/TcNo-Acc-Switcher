using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Steam
{
    public class SteamUsersRepository : ISteamUsersRepository
    {
        public async Task<List<SteamUser>> GetSteamUsersAsync()
        {
            // Get steam users from loginusers.vdf
            await Task.Run(() => { });
            List<SteamUser> Users = new List<SteamUser>();
            return Users;
        }

        public async Task DeleteSteamUserAsync(string SteamId)
        {
            // Delete
            // Save changes async
            await Task.Run(() => { });
        }

        public async Task<SteamUser> GetImageAsync(string SteamId)
        {
            await Task.Run(() => {});
            return new SteamUser();
        }
    }
}
