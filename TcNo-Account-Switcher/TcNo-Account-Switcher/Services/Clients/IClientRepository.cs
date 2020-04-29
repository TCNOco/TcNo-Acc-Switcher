using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Clients
{
    public interface IClientRepository
    {
        Task<List<SoftwareClient>> GetSoftwareClientsAsync(); // Load found software, or load from file. Show the rest grayed out.
        Task<SoftwareClient> AddSoftwareClient(string selectedExe); // Find if .exe supported, then save that software location.
        Task<SoftwareClient> UpdateSoftwareClient(string name, string selectedExe); // Relocate program to another location.
    }
}
