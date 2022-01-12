// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.Buffers;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;
using JsonSerializer = System.Text.Json.JsonSerializer;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class Lang
    {
        private static Lang _instance = new();

        private static readonly object LockObj = new();

        public static Lang Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Lang();
                }
            }
            set => _instance = value;
        }

        private Dictionary<string, string> _strings = new();
        public Dictionary<string, string> Strings { get => _instance._strings; set => _instance._strings = value; }

        private string _current = "";

        public string Current { get => _instance._current; set => _instance._current = value; }

        /// <summary>
        /// Get a string
        /// </summary>
        public string this[string key] => Strings.ContainsKey(key) ? Strings[key] : key;

        /// <summary>
        /// Get a string, and replace variables
        /// </summary>
        /// <param name="key">String to look up</param>
        /// <param name="obj">Object of variables to replace</param>
        public string this[string key, object obj]
        {
            get
            {
                try
                {
                    if (!Strings.ContainsKey(key)) return key;
                    var s = Strings[key];
                    if (obj is JsonElement)
                        foreach (var (k, v) in JObject.FromObject(JsonConvert.DeserializeObject(obj.ToString()!)!))
                            s = s.Replace($"{{{k}}}", v.Value<string>());
                    else
                        foreach (var pi in obj.GetType().GetProperties())
                        {
                            dynamic val = pi.GetValue(obj, null);
                            if (val is int) val = val.ToString();
                            s = s.Replace($"{{{pi.Name}}}", (string)val);
                        }
                    return s;
                }
                catch (NullReferenceException e)
                {
                    Globals.WriteToLog(e);
                    return "[Failed to get text: missing parameter] " + key;
                }
            }
        }


        #region FILE_HANDLING
        /// <summary>
        /// Loads the programs default language: English.
        /// </summary>
        public void LoadDefault()
        {
            _ = Load("en-US");
        }

        /// <summary>
        /// Loads the system's language, or the user's saved language
        /// </summary>
        public void LoadLocalised()
        {
            LoadDefault();
            // If setting does not exist in settings file then load the system default
            _ = Instance.Load(AppSettings.Instance.Language == ""
                ? CultureInfo.CurrentCulture.Name
                : AppSettings.Instance.Language);
        }

        /// <summary>
        /// Tries to load a requested language
        /// </summary>
        /// <param name="lang">Formatted language, example: "en-US"</param>
        public bool LoadLang(string lang)
        {
            LoadDefault();
            return Load(lang, true);
        }

        /// <summary>
        /// Get list of files in Resources folder
        /// </summary>
        public List<string> GetAvailableLanguages() => Directory.GetFiles(Path.Join(Globals.AppDataFolder, "Resources")).Select(f => Path.GetFileName(f).Split(".yml")[0]).ToList();
        public Dictionary<string, string> GetAvailableLanguagesDict() => GetAvailableLanguages().ToDictionary(l => new CultureInfo(l).DisplayName);
        public KeyValuePair<string, string> GetCurrentLanguage() => new(new CultureInfo(Current).DisplayName, Current);

        public bool Load(string filename, bool save = false)
        {
            var path = Path.Join(Globals.AppDataFolder, "Resources", filename + ".yml");
            Current = filename;
            if (save && Current == filename)
            {
                AppSettings.Instance.Language = filename;
                AppSettings.Instance.SaveSettings();
            }
            if (!File.Exists(path))
            {
                // Get list of files in Resources folder
                var availableLang = GetAvailableLanguages();

                // Get closest available language
                var langGroup = filename.Split('-')[0];
                var foundClose = false;
                foreach (var l in availableLang.Where(l => l.StartsWith(langGroup)))
                {
                    path = Path.Join(Globals.AppDataFolder, "Resources", l + ".yml");
                    foundClose = true;
                    Current = l;
                    if (save && Current == l)
                    {
                        AppSettings.Instance.Language = l;
                        AppSettings.Instance.SaveSettings();
                    }
                    break;
                }

                // If could not find a language close to requested
                if (!foundClose)
                    return false;
            }

            // Load from en-EN file into Strings
            var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
            var text = Globals.ReadAllText(path);
            var dict = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(text)));
            Debug.Assert(dict != null, nameof(dict) + " != null"); // These files have to exist, or the program will break in many ways
            foreach (var (k, v) in dict)
            {
                Strings[k] = v;
            }

            return true;
        }
        #endregion
    }

    public static partial class JsonExtensions
    {
        public static T ToObject<T>(this JsonElement element, JsonSerializerOptions options = null)
        {
            var bufferWriter = new ArrayBufferWriter<byte>();
            using (var writer = new Utf8JsonWriter(bufferWriter))
            {
                element.WriteTo(writer);
            }

            return JsonSerializer.Deserialize<T>(bufferWriter.WrittenSpan, options);
        }

        public static T ToObject<T>(this JsonDocument document, JsonSerializerOptions options = null)
        {
            if (document == null)
            {
                throw new ArgumentNullException(nameof(document));
            }

            return document.RootElement.ToObject<T>(options);
        }

        public static object ToObject(this JsonElement element, Type returnType, JsonSerializerOptions options = null)
        {
            var bufferWriter = new ArrayBufferWriter<byte>();
            using (var writer = new Utf8JsonWriter(bufferWriter))
            {
                element.WriteTo(writer);
            }

            return JsonSerializer.Deserialize(bufferWriter.WrittenSpan, returnType, options);
        }

        public static object ToObject(this JsonDocument document, Type returnType, JsonSerializerOptions options = null)
        {
            if (document == null)
            {
                throw new ArgumentNullException(nameof(document));
            }

            return document.RootElement.ToObject(returnType, options);
        }
    }
}