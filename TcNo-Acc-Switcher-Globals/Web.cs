using System;
using System.Collections.Generic;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using HtmlAgilityPack;

namespace TcNo_Acc_Switcher_Globals;

public partial class Globals
{
    // Set "user-agent"= "TcNo Account Switcher"
    public static readonly HttpClient Client = GetNewClient();

    private static HttpClient GetNewClient()
    {
        var client = new HttpClient();
        client.DefaultRequestHeaders.Add("user-agent", "TcNo Account Switcher");
        client.DefaultRequestHeaders.Add("dnt", "1");
        client.DefaultRequestHeaders.Add("cache-control", "max-age=0");
        client.DefaultRequestHeaders.Add("accept-language", "en-US,en;q=0.9");

        return client;
    }
    
    /// <summary>
    /// Uploads CrashLogs and log.txt if crashed.
    /// </summary>
    public static void UploadLogs()
    {
        if (!Directory.Exists("CrashLogs")) return;
        if (!Directory.Exists("CrashLogs\\Submitted")) _ = Directory.CreateDirectory("CrashLogs\\Submitted");

        // Collect all logs into one string to compress
        var postData = new Dictionary<string, string>();
        var combinedCrashLogs = "";
        foreach (var file in Directory.EnumerateFiles("CrashLogs", "*.txt"))
        {
            try
            {
                combinedCrashLogs += Globals.ReadAllText(file);
                File.Move(file, $"CrashLogs\\Submitted\\{Path.GetFileName(file)}");
            }
            catch (Exception e)
            {
                Globals.WriteToLog(@"[Caught - UploadLogs()]" + e);
            }
        }

        // If no logs collected, return.
        if (combinedCrashLogs == "") return;

        // Else: send log file as well.
        if (File.Exists("log.txt"))
        {
            try
            {
                postData.Add("logs", Compress(Globals.ReadAllText("log.txt")));
            }
            catch (Exception e)
            {
                Globals.WriteToLog(@"[Caught - UploadLogs()]" + e);
            }
        }

        // Send report to server
        postData.Add("crashLogs", Compress(combinedCrashLogs));
        if (postData.Count == 0) return;

        try
        {
            HttpContent content = new FormUrlEncodedContent(postData);
            _ = Client.PostAsync("https://api.tcno.co/sw/crash/", content);
        }
        catch (Exception e)
        {
            File.WriteAllText($"CrashLogs\\CrashLogUploadErr-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt", Globals.GetEnglishError(e));
        }
    }

    private static string Compress(string text)
    {
        //https://www.neowin.net/forum/topic/994146-c-help-php-compatible-string-gzip/
        var buffer = Encoding.UTF8.GetBytes(text);

        var memoryStream = new MemoryStream();
        using (var gZipStream = new GZipStream(memoryStream, CompressionMode.Compress, true))
        {
            gZipStream.Write(buffer, 0, buffer.Length);
        }

        memoryStream.Position = 0;

        var compressedData = new byte[memoryStream.Length];
        _ = memoryStream.Read(compressedData, 0, compressedData.Length);

        return Convert.ToBase64String(compressedData);
    }
    
    
    /// <summary>
    /// Reads website as out webText. Returns true if successful.
    /// </summary>
    public static bool ReadWebUrl(string requestUri, out string webText, string cookies = "")
    {
        var request = new HttpRequestMessage(HttpMethod.Get, requestUri);
        if (!string.IsNullOrEmpty(cookies))
            request.Headers.Add("cookie", cookies);
        
        var response = Client.Send(request);
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
    /// <param name="wwwroot">Use Globals.WwwRoot</param>
    public static void DownloadProfileImage(string platformName, string uniqueId, string urlOrPath, string wwwroot)
    {
        var destination = $"wwwroot\\img\\profiles\\{GetCleanFilePath(platformName)}\\{uniqueId}.jpg";
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
        catch (Exception e)
        {
            CopyFile(Path.Join(wwwroot, "\\img\\BasicDefault.png"), destination);
        }
    }
}