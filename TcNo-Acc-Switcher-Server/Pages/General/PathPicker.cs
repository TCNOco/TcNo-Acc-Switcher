using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.General
{

    public class PathPicker
    {
        private static Dictionary<string, List<string>> ReturnObject = new Dictionary<string, List<string>>()
        {
            {"Folders", new List<string>()},
            {"Files", new List<string>()}
        };

        [JSInvokable]
        public static object GetLogicalDrives()
        {
            var ro = ReturnObject;
            try
            {
                ro["Folders"] = System.IO.Directory.GetLogicalDrives().ToList();
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Could not list Logical Drives.", e);
            }
            return ro;
        }

        [JSInvokable]
        public static object GetFoldersAndFiles(string path)
        {
            var ro = ReturnObject;
            try
            {
                ro["Folders"] = Directory.GetDirectories(path).ToList();
                ro["Files"] = Directory.GetFiles(path).ToList();
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Could not get list of files and folders.", e);
            }
            return ro;
        }
    }
}
