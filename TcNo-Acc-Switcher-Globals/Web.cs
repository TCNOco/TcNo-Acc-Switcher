using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        public static string ReadWebUrl(string requestUri)
        {
            var client = new HttpClient();
            var response = client.Send(new HttpRequestMessage(HttpMethod.Get, requestUri));
            var responseReader = new StreamReader(response.Content.ReadAsStream());
            return responseReader.ReadToEnd();
        }

        /// <summary>
        /// Downloads a dictionary of URL:FILES over x threads
        /// </summary>
        /// <param name="filesWithUrls">URL:FILES</param>
        /// <param name="threads">Number of threads for parallel downloading</param>
        /// <returns></returns>
        public static async Task MultiThreadParallelDownloads(Dictionary<string, string> filesWithUrls, int threads = 3)
        {
            var semaphore = new SemaphoreSlim(Math.Min(threads, filesWithUrls.Count), threads);
            await Task.WhenAll(filesWithUrls.Select(async dl =>
            {
                await semaphore.WaitAsync();
                try
                {
                    DebugWriteLine($"[Func:MultiThreadDownload] Downloading: \"{dl.Key}\" to \"{dl.Value}\" - {semaphore.CurrentCount}");
                    await DownloadFileAsync(dl.Key, dl.Value);
                }
                finally
                {
                    semaphore.Release();
                }
            }));
        }

        /// <summary>
        /// Downloads a dictionary of URL:FILES over x threads
        /// </summary>
        /// <param name="keysAndUrls">key:URL</param>
        /// <param name="threads">Number of threads for parallel downloading</param>
        /// <returns></returns>
        public static async Task MultiThreadParallelReadUrl(Dictionary<string, string> keysAndUrls, int threads = 3)
        {
            var semaphore = new SemaphoreSlim(0, threads);
            await Task.WhenAll(keysAndUrls.Select(async dl =>
            {
                await semaphore.WaitAsync();
                {
                    DebugWriteLine($"[Func:MultiThreadDownload] Downloading: \"{dl.Value}\" to \"{dl.Key}\"");
                    await DownloadFileAsync(dl.Value, dl.Key);
                }
                semaphore.Release();
            }));
        }

    }
}
