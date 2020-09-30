using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;

namespace TcNo_Acc_Switcher.Pages.Steam
{
    public class SteamSwitcherBase
    {
        //[JSInvokable]
        //public static Task<int> CopyProfileURL()
        //{
        //    Console.WriteLine("ffffffffffffffffffffff");
        //    return Task.FromResult(0);
        //}
        //[JSInvokable]
        //public static void CopyCommunityUsername(string id)
        //{
        //    Console.WriteLine("YOUR ID IS HERE: " + id);
        //}

        [JSInvokable]
        public static void CopySpecial(string request)
        {
            switch (request)
            {
                case "URL":
                    return;
            }
            var url = "";

            Data.GenericFunctions.CopyToClipboard(url);
        }

        [JSInvokable]
        public static void CopySteamIDType(string request, string SteamId64)
        {
            switch (request)
            {
                case "SteamId":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(SteamId64).Id);
                    break;
                case "SteamId3":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(SteamId64).Id3);
                    break;
                case "SteamId32":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(SteamId64).Id32);
                    break;
                case "SteamId64":
                    Data.GenericFunctions.CopyToClipboard(new Converters.SteamIdConvert(SteamId64).Id64);
                    break;
            }
        }




        //[JSInvokable]
        //public static Task<int> CopyProfileUrl()
        //{
        //    return Task.FromResult(new Random().Next());
        //}
    }
}
