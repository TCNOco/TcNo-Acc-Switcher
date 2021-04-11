using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Server.Shared
{
    public partial class ContextMenu : ComponentBase
    {
        private static string _htmlOut = "";
        private static string GetContextMenu()
        {
            _htmlOut = "<ul class=\"contextmenu\">";
            string jText = @"[
              {""Swap to account"": ""SwapTo(-1, event)""},
              {""Login as..."": [
                {""Invisible"": ""SwapTo(7, event)""},
                {""Offline"": ""SwapTo(0, event)""},
                {""Online"": ""SwapTo(1, event)""},
                {""Busy"": ""SwapTo(2, event)""},
                {""Away"": ""SwapTo(3, event)""},
                {""Snooze"": ""SwapTo(4, event)""},
                {""Looking to Trade"": ""SwapTo(5, event)""},
                {""Looking to Play"": ""SwapTo(6, event)""}
              ]},
              {""Copy Profile..."": [
                {""Community URL"": ""copy('URL', event)""},
                {""Community Username"": ""copy('Line2', event)""},
                {""Login username"": ""copy('Username', event)""}
              ]},
              {""Copy SteamID..."": [
                {""SteamID [STEAM0:~]"": ""copy('SteamId', event)""},
                {""SteamID3 [U:1:~]"": ""copy('SteamId3', event)""},
                {""SteamID32"": ""copy('SteamId32', event)""},
                {""SteamID64 7656~"": ""copy('SteamId64', event)""}
              ]},
              {""Copy other..."": [
                {""SteamRep"": ""copy('SteamRep', event)""},
                {""SteamID.uk"": ""copy('SteamID.uk', event)""},
                {""SteamID.io"": ""copy('SteamID.io', event)""},
                {""SteamRep"": ""copy('SteamIDFinder.com', event)""}
              ]},
              {""Create Desktop Shortcut"": ""CreateShortcut()""},
              {""Forget"": ""forget(event)""}
            ]";

            var jO = JArray.Parse(jText);
            foreach (var kvp in jO) // Main list
            {
                // Each item
                foreach (var (key, value) in JObject.FromObject(kvp))
                {
                    ProcessContextItem(key, value);
                }
            }

            _htmlOut += "</ul>";
            return _htmlOut;
        }

        private static void ProcessContextItem(string s, object o)
        {
            var j = JToken.FromObject(o);
            // See if it's a string
            try
            {
                var action = j.Value<string>();
                // Add key and string item
                _htmlOut += $"<li><a onclick=\"{action}\">{s}</a></li>\n";
                return;
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }

            // See if it's an array
            try
            {
                // Add key and string item
                var jArray = j.Value<JArray>();
                _htmlOut += $"<li><a onclick=\"event.preventDefault();\">{s}</a>\n\t<ul class=\"submenu\">";
                // Foreach
                foreach (var jToken in jArray)
                {
                    // Each item
                    foreach (var (key, value) in JObject.FromObject(jToken))
                    {
                        ProcessContextItem(key, value);
                    }
                }
                _htmlOut += "\t</ul>\n</li>";
                return;
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }
        }
    }
}
