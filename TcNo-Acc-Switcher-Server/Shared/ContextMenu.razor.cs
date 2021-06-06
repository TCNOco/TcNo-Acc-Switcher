using System;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Shared
{
    public partial class ContextMenu
    {
        private static string _htmlOut = "";
        /// <summary>
        /// Creates a context menu from input json
        /// </summary>
        /// <param name="contextMenuText">JSON string making up context menu</param>
        /// <returns>String of HTML elements, making up the context menu</returns>
        private static string GetContextMenu(string contextMenuText)
        {
            Globals.DebugWriteLine(@"[Func:Shared\ContextMenu.GetContextMenu]");
            _htmlOut = "<ul class=\"contextmenu\">";

            var jO = JArray.Parse(contextMenuText);
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

        /// <summary>
        /// Iterates through input JObject and adds HTML strings to existing _htmlOut, for Context Menu
        /// </summary>
        /// <param name="s">Text to display on current element</param>
        /// <param name="o">Either string with action, or JToken containing more (string, string) or (string, JArray) pairs</param>
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
            catch (InvalidCastException)
            {
                // Left blank
            }
            catch(Exception e)
            {
                Globals.WriteToLog(e.ToString());
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
                        if (key == "")
                            _htmlOut = GeneralFuncs.ReplaceLast(_htmlOut, "event.preventDefault();", value.Value<string>()); // Replace last occurence
                        else ProcessContextItem(key, value);
                    }
                }
                _htmlOut += "\t</ul>\n</li>";
            }
            catch (InvalidCastException)
            {
                // Left blank
            }
            catch (Exception e)
            {
                Globals.WriteToLog(e.ToString());
            }
        }
    }
}
