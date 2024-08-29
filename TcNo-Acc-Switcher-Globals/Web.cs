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
        /// <summary>
        /// Reads website as out webText. Returns true if successful.
        /// </summary>
        public static bool ReadWebUrl(string requestUri, out string webText, string cookies = "")
        {
            using var client = new HttpClient();

            client.DefaultRequestHeaders.Add("user-agent", "TcNo Account Switcher");
            if (!string.IsNullOrEmpty(cookies))
            {
                client.DefaultRequestHeaders.Add("cookie", cookies);
            }
            client.DefaultRequestHeaders.Add("dnt", "1");
            client.DefaultRequestHeaders.Add("cache-control", "max-age=0");
            client.DefaultRequestHeaders.Add("accept-language", "en-US,en;q=0.9");

            var response = client.Send(new HttpRequestMessage(HttpMethod.Get, requestUri));
            var responseReader = new StreamReader(response.Content.ReadAsStream());
            webText = responseReader.ReadToEnd();

            if (response.StatusCode == HttpStatusCode.OK) return true;

            WriteToLog($"ERROR LOADING URL: {requestUri}.\n - Error: {response.StatusCode}");
            return false;
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


        public static bool GetWebHtmlDocument(ref HtmlDocument htmlDocument, string url, out string responseText, string cookies = "")
        {
            var readSuccess = ReadWebUrl(url, out responseText, cookies);

            try
            {
                htmlDocument.LoadHtml(responseText);
            }
            catch (Exception)
            {
                return false;
            }

            return readSuccess;
        }

        /// <summary>
        /// Downloads or copies an image to the requested account. Copies default if fails.
        /// </summary>
        /// <param name="platformName">Platform name</param>
        /// <param name="uniqueId">Unique ID of account</param>
        /// <param name="urlOrPath">URL or filepath to download/copy image from</param>
        /// <param name="wwwroot">Use GeneralFuncs.WwwRoot()</param>
        public static void DownloadProfileImage(string platformName, string uniqueId, string urlOrPath, string wwwroot, bool offlineMode = false)
        {
            var destination = $"wwwroot\\img\\profiles\\{GetCleanFilePath(platformName)}\\{uniqueId}.jpg";

            if (offlineMode)
            {
                CopyFile(Path.Join(wwwroot, "\\img\\BasicDefault.png"), destination);
                return;
            }

            try
            {
                // Is url -> Download
                if (!DownloadFile(urlOrPath, destination))
                {
                    // Is not url -> Copy file
                    if (!CopyFile(ExpandEnvironmentVariables(urlOrPath), destination))
                    {
                        // Is neither! Copy in the default.
                        throw new Exception("Could not download or copy file!");
                    }
                }
            }
            catch (Exception)
            {
                CopyFile(Path.Join(wwwroot, "\\img\\BasicDefault.png"), destination);
            }
        }
    }
}
