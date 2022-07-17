using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Linq;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Globals
{
    public static class Extensions
    {
        public static void Sort<T>(this ObservableCollection<T> collection) where T : IComparable
        {
            var sorted = collection.OrderBy(x => x).ToList();
            for (var i = 0; i < sorted.Count; i++)
                collection.Move(collection.IndexOf(sorted[i]), i);
        }
    }

    public static class JsonExtensions
    {
        // From: https://stackoverflow.com/a/35804255 and https://dotnetfiddle.net/f3q04u
        // Enables easy replacement of specific vars in JObject: var newJsonAuthorString = JsonExtensions.ReplacePath(jsonString, @"$.store.book[*].author", "NewAuthorSpecifiedByUser");
        /// <summary>
        /// Replaces a token's child or children with a new value.
        /// </summary>
        /// <typeparam name="T"></typeparam>
        /// <param name="root">JToken item to be edited and returned</param>
        /// <param name="path">Selector path of key to be edited</param>
        /// <param name="newValue">New value for key/s</param>
        /// <returns>Modified JToken</returns>
        /// <exception cref="ArgumentNullException"></exception>
        public static JToken ReplacePath<T>(this JToken root, string path, T newValue)
        {
            if (root == null || path == null)
                throw new ArgumentNullException();

            foreach (var value in root.SelectTokens(path).ToList())
            {
                if (value == root)
                    root = JToken.FromObject(newValue);
                else
                    value.Replace(JToken.FromObject(newValue));
            }

            return root;
        }

        /// <summary>
        /// Replaces a token's child or children with a new value. This version takes a string as input, and outputs as one.
        /// </summary>
        /// <typeparam name="T"></typeparam>
        /// <param name="jsonString">JSON string to edit</param>
        /// <param name="path">Selector path of key to be edited</param>
        /// <param name="newValue">New value for key/s</param>
        /// <returns>Modified JSON string</returns>
        public static string ReplacePath<T>(string jsonString, string path, T newValue)
        {
            return JToken.Parse(jsonString).ReplacePath(path, newValue).ToString();
        }
    }
}
