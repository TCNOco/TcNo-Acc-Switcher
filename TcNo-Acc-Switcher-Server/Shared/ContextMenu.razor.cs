using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Shared
{
    public partial class ContextMenu : ComponentBase
    {
        private static string _htmlOut = "";
        /// <summary>
        /// Creates a context menu from input json
        /// </summary>
        /// <param name="contextMenuText">JSON string making up context menu</param>
        /// <returns>String of HTML elements, making up the context menu</returns>
        private static string GetContextMenu(string contextMenuText)
        {
            Globals.DebugWriteLine($@"[Func:Shared\ContextMenu.GetContextMenu]");
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
            catch (InvalidCastException e)
            {
                // Left blank
            }
            catch(Exception e)
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
            catch (InvalidCastException e)
            {
                // Left blank
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }
        }
    }
}
