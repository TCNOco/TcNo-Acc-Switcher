using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Client.Steam
{
    // // Currently unused code.
    //internal class SteamKeys
    //{
    //    private HttpClient _client;

    //    /// <summary>
    //    /// Input a Steam key to activate, as well as cookies for the session.
    //    /// SteamKeys.ActivateKey(cookie, "XXXXX-XXXXX-XXXXX", out var noMore);
    //    /// </summary>
    //    /// <param name="cookie"></param>
    //    /// <param name="key">Steam key</param>
    //    /// <param name="noMore">(out) If true, do not try any more keys.</param>
    //    /// <returns>(bool) Success</returns>
    //    public Task<bool> ActivateKey(string cookie, string key, out bool noMore)
    //    {
    //        // Cookie provided MUST be in this format for Steam (Order doesn't matter):
    //        // "browserid=XXX; timezoneOffset=XXXX,0; _ga=GA1.XXXXX; _gid=GA1.XXXXX; steamMachineAuth765XXXXXXX=XXXX; steamRememberLogin=XXXX; steamRememberLogin=XXXX; sessionid=XXXX;"

    //        noMore = false;
    //        const string keyUrl = "https://store.steampowered.com/account/ajaxregisterkey/";
    //        var baseAddress = new Uri(keyUrl);
    //        // Note this doesn't seem like it needs headers to work, so I've just left out "Headers =".
    //        // The cookies are manually being set below. They can't be set in "Headers ="
    //        var httpRequestMessage = new HttpRequestMessage
    //        {
    //            Method = HttpMethod.Post,
    //            RequestUri = baseAddress,
    //            Content = new FormUrlEncodedContent(new[]
    //            {
    //                new KeyValuePair<string, string>("product_key", key),
    //                new KeyValuePair<string, string>("sessionid", GetSessionId(cookie)),
    //            })
    //        };

    //        // Assign client if not already
    //        _client ??= new HttpClient { BaseAddress = baseAddress };

    //        _client.DefaultRequestHeaders.Add("Cookie", cookie);
    //        var response = _client.SendAsync(httpRequestMessage).Result;
    //        var result = response.Content.ReadAsStringAsync().Result;
    //        Console.WriteLine(@"-----");
    //        Console.WriteLine(result);
    //        Console.WriteLine(@"-----");

    //        // Decode json result
    //        var json = JObject.Parse(result);
    //        var success = false;
    //        var recieptText = "";
    //        var logText = "";
    //        if (json.ContainsKey("success"))
    //        {
    //            success = json.Value<int>("success") == 1;
    //            recieptText = success ? "Activated key! " : "Failed to activate key! ";
    //            logText = success ? $"{key}: OK: " : $"{key}: FAIL ";
    //        }

    //        // Handle failure
    //        if (!success && json.ContainsKey("purchase_result_details"))
    //        {
    //            var resultCode = json.Value<int>("purchase_result_details");
    //            string codeText;
    //            switch (resultCode)
    //            {
    //                case 9:
    //                    // This Steam account already activated this key.
    //                    codeText = "This Steam account already activated this key.";
    //                    break;
    //                case 13:
    //                    // Not available for purchase in this country.
    //                    codeText = "Not available for purchase in this country.";
    //                    break;
    //                case 14:
    //                    // Invalid product code.
    //                    codeText = "Invalid product code.";
    //                    break;
    //                case 15:
    //                    // Activated by another user.
    //                    codeText = "Activated by another user.";
    //                    break;
    //                case 24:
    //                    // Key requires ownership of another product.
    //                    codeText = "Key requires ownership of another product.";
    //                    break;
    //                case 36:
    //                    // Requires that you first play on a PlayStation 3 system before activating.
    //                    codeText = "Requires that you first play on a PlayStation 3 system before activating.";
    //                    break;
    //                case 53:
    //                    // Too many recent activation attempts.
    //                    codeText = "Too many recent activation attempts.";
    //                    noMore = true;
    //                    break;
    //                default:
    //                    codeText = $"Unknown error code: {resultCode}";
    //                    break;
    //            }

    //            Console.WriteLine(codeText);
    //            recieptText += codeText;
    //            logText += codeText;

    //            recieptText += ' ';
    //            logText += ' ';
    //            if (json.ContainsKey("purchase_result_details"))
    //            {
    //                var reciept = json.Value<JObject>("purchase_receipt_info");
    //                if (reciept == null) return Task.FromResult(false);
    //                ProcessSteamReciept(json, ref recieptText, ref logText);
    //            }

    //        }

    //        // Handle success
    //        if (success)
    //            ProcessSteamReciept(json, ref recieptText, ref logText);

    //        // DO SOMETHING WITH recieptText HERE!
    //        Console.WriteLine(recieptText);
    //        Console.WriteLine(@"----");
    //        Console.WriteLine(logText);
    //        return Task.FromResult(success);
    //    }

    //    /// <summary>
    //    /// Returns sessionId from cookies string
    //    /// </summary>
    //    private string GetSessionId(string cookies)
    //    {
    //        return cookies.Substring(cookies.IndexOf("sessionid=", StringComparison.Ordinal) + 10, 24);
    //    }

    //    /// <summary>
    //    /// Processes the Steam reciept to simple output information.
    //    /// </summary>
    //    private void ProcessSteamReciept(JObject json, ref string recieptText, ref string logText)
    //    {
    //        if (!json.ContainsKey("purchase_receipt_info")) return;

    //        var reciept = json.Value<JObject>("purchase_receipt_info");
    //        if (reciept == null) return;

    //        recieptText += $"Transaction ID: {reciept.Value<string>("transactionid")}" + Environment.NewLine;
    //        recieptText += $"Transaction Time: {reciept.Value<string>("transaction_time")}" + Environment.NewLine;

    //        if (reciept.ContainsKey("line_items"))
    //        {
    //            var lineItems = reciept.Value<JArray>("line_items");
    //            if (lineItems == null) return; // DO SOMETHING WITH recieptText HERE!
    //            for (var i = 0; i < lineItems.Count; i++)
    //            {
    //                var item = lineItems[i];
    //                recieptText += $"(Item {i + 1}): \"{item.Value<string>("line_item_description")}\" [Package ID: {item.Value<string>("packageid")}]" + Environment.NewLine;
    //                if (lineItems.Count == 1)
    //                    logText += $" - \"{item.Value<string>("line_item_description")}\" ({item.Value<string>("packageid")})";
    //                else
    //                    logText += Environment.NewLine + $"  - \"{i + 1}: {item.Value<string>("line_item_description")}\" [Package ID:{item.Value<string>("packageid")}]";
    //            }
    //        }
    //    }
    //}
}
