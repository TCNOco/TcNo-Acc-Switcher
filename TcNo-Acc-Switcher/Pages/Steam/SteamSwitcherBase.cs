using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;

namespace TcNo_Acc_Switcher.Pages.Steam
{
    public class SteamSwitcherBase
    {
        [JSInvokable]
        public static Task<int> CopyProfileURL()
        {
            Console.WriteLine("ffffffffffffffffffffff");
            return Task.FromResult(0);
        }

        //[JSInvokable]
        //public static Task<int> CopyProfileUrl()
        //{
        //    return Task.FromResult(new Random().Next());
        //}
    }
}
