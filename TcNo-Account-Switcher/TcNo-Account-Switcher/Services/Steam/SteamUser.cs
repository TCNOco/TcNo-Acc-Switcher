using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Account_Switcher.Services.Steam
{
    public class SteamUser
    {
        public SteamUser()
        {

        }

        public string Name { get; set; }
        public string SteamId { get; set; }
        public string ImgUrl { get; set; }
        public string LastLogin { get; set; }
        public string AccName { get; set; }
        public System.Windows.Media.Brush VacStatus { get; set; }
    }
}
