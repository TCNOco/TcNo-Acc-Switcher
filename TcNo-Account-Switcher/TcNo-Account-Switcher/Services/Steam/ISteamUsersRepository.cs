using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Steam
{
    public interface ISteamUsersRepository
    {
        Task<List<SteamUser>> GetSteamUsersAsync();
        Task DeleteSteamUserAsync(string steamId);
        Task<SteamUser> GetImageAsync(string steamId);
    }
}
