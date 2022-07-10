using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using HtmlAgilityPack;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        public static string ReadWebUrl(string requestUri)
        {
            var client = new HttpClient();
            var response = client.Send(new HttpRequestMessage(HttpMethod.Get, requestUri));
            if (response.StatusCode != HttpStatusCode.OK)
            {
                WriteToLog($"ERROR LOADING URL: {requestUri}.\n - Error: {response.StatusCode}");
            }

            var responseReader = new StreamReader(response.Content.ReadAsStream());
            return responseReader.ReadToEnd();
        }

        private static SemaphoreSlim _semaphoreSlim;
        /// <summary>
        /// Downloads a dictionary of URL:FILES over x threads
        /// </summary>
        /// <param name="filesWithUrls">URL:FILES</param>
        /// <param name="threads">Number of threads for parallel downloading</param>
        /// <returns></returns>
        public static async Task MultiThreadParallelDownloads(Dictionary<string, string> filesWithUrls, int threads = 10)
        {
            _semaphoreSlim = new SemaphoreSlim(threads, threads);
            var tasks = filesWithUrls.Select(fileWithUrl => Task.Run(() => DownloadWorker(fileWithUrl))).ToList();

            await Task.WhenAll(tasks);
        }

        //private static Random rnd = new Random();
        private static async Task DownloadWorker(KeyValuePair<string, string> filesWithUrl)
        {
            //var threadId = rnd.Next(0, 10000);
            //Console.WriteLine($@"THREAD CREATED ({threadId}) - {DateTime.Now:hh:mm:ss.fff}");

            await _semaphoreSlim.WaitAsync();
            try
            {
                DebugWriteLine($"[Func:MultiThreadParallelDownloads - DownloadWorker] Downloading: \"{filesWithUrl.Key}\" to \"{filesWithUrl.Value}\"");
                await DownloadFileAsync(filesWithUrl.Key, filesWithUrl.Value);
            }
            finally
            {
                //Console.WriteLine($@"THREAD FINISHED ({threadId}) - {DateTime.Now:hh:mm:ss.fff}");
                _semaphoreSlim.Release();
            }
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


        public static bool GetWebHtmlDocument(ref HtmlDocument htmlDocument, string url, out string responseText)
        {
            responseText = ReadWebUrl(url);
            try
            {
                htmlDocument.LoadHtml(responseText);
            }
            catch (Exception)
            {
                return false;
            }

            return true;
        }
    }
}
