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
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Net.Http;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State.DataTypes;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.Shared.Modal.Modals
{
    public partial class ModalCrowdin
    {
        [Inject] private IToasts Toasts { get; set; }

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
                var html = Globals.Client.GetStringAsync(
                    "https://api.tcno.co/sw/crowdin/").Result;
                var resp = JsonConvert.DeserializeObject<CrowdinResponse>(html);
                if (resp is null)
                    return new CrowdinResponse();

                resp.Translators.Sort();

                var expandedProofreaders = new SortedDictionary<string, string>();
                foreach (var proofReader in resp.ProofReaders)
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
