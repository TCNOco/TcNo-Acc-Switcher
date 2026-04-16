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
        private static readonly Dictionary<string, List<string>> ReturnObject = new()
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
                ro["Folders"] = Directory.GetLogicalDrives().ToList();
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

        [JSInvokable]
        public static object GetFolders(string path)
        {
            var ro = ReturnObject;
            try
            {
                ro["Folders"] = Directory.GetDirectories(path).ToList();
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Could not get list of files and folders.", e);
            }
            return ro;
        }
    }
}
