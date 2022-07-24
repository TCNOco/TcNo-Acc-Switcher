using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Net.Http;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.Shared.Modal.Modals
{
    public partial class ModalCrowdin
    {
        [Inject] private Toasts Toasts { get; set; }

        public class CrowdinResponse
        {
            [JsonProperty("ProofReaders")]
            public SortedDictionary<string, string> ProofReaders { get; set; }

            [JsonProperty("Translators")]
            public List<string> Translators { get; set; }
        }


        /// <summary>
        /// Returns an object with a list of all translators, and proofreaders with their languages.
        /// </summary>
        public CrowdinResponse CrowdinList()
        {
            try
            {
                var html = new HttpClient().GetStringAsync(
                    "https://tcno.co/Projects/AccSwitcher/api/crowdinNew/").Result;
                var resp = JsonConvert.DeserializeObject<CrowdinResponse>(html);
                if (resp is null)
                    return new CrowdinResponse();

                resp.Translators.Sort();

                var expandedProofreaders = new SortedDictionary<string, string>();
                foreach (var proofReader in resp?.ProofReaders)
                {
                    expandedProofreaders.Add(proofReader.Key,
                        string.Join(", ",
                            proofReader.Value.Split(',').Select(lang => new CultureInfo(lang).DisplayName)
                                .ToList()));
                }

                resp.ProofReaders = expandedProofreaders;
                return resp;
            }
            catch (Exception e)
            {
                // Handle website not loading or JObject not loading properly
                Globals.WriteToLog("Failed to load Crowdin users", e);
                Toasts.ShowToastLang(ToastType.Error, "Crowdin_Fail");
                return new CrowdinResponse();
            }
        }
    }
}
