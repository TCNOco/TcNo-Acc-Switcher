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
using System.Diagnostics;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using ImageMagick;
using Microsoft.AspNetCore.Components;
using SkiaSharp;
using Svg.Skia;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Data.Classes
{
    public class Shortcut : IShortcut
    {
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        [Inject] private ILang Lang { get; }
        [Inject] private IAppSettings AppSettings { get; }
        [Inject] private IAppData AppData { get; }

        public string Exe { get; set; }
        public string WorkingDir { get; set; }
        public string IconDir { get; set; }
        public string ShortcutPath { get; set; }

        public string Desc { get; set; }
        public string Args { get; set; }

        public string Desktop { get; set; } = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        public string StartMenu { get; set; } = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\");

        // Library class used to see if a shortcut exists.
        public string ParentDirectory(string dir) => dir[..dir.LastIndexOf(Path.DirectorySeparatorChar)];
        public bool ShortcutExist() => File.Exists(ShortcutPath);
        public string ShortcutDir() => Path.GetDirectoryName(ShortcutPath) ?? throw new InvalidOperationException();
        public string GetSelfPath() => Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe");


        /// <summary>
        /// Write Shortcut using WShell. Not too sure how else to do this, and it works.
        /// Probably will find another way of doing this at some stage.
        /// </summary>
        private void WriteShortcut()
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.WriteShortcut]");
            _ = Directory.CreateDirectory(ShortcutDir());
            if (File.Exists(ShortcutPath)) return;
            Globals.DeleteFile("CreateShortcut.vbs");

            File.WriteAllLines("CreateShortcut.vbs", new[] {
                "set WshShell = WScript.CreateObject(\"WScript.Shell\")",
                "set oShellLink = WshShell.CreateShortcut(\"" + ShortcutPath + "\")",
                "oShellLink.TargetPath = \"" + Exe + "\"",
                "oShellLink.WindowStyle = 1",
                "oShellLink.IconLocation = \"" + IconDir + "\"",
                "oShellLink.Description = \"" + Desc + "\"",
                "oShellLink.WorkingDirectory = \"" + WorkingDir + "\"",
                "oShellLink.Arguments = \"" + Args + "\"",
                "oShellLink.Save()"
            });

            var vbsProcess = new Process
            {
                StartInfo =
                {
                    FileName = "cscript",
                    Arguments = "//nologo \"" + Path.GetFullPath("CreateShortcut.vbs") + "\"",
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    CreateNoWindow = true
                }
            };

            _ = vbsProcess.Start();
            _ = vbsProcess.StandardOutput.ReadToEnd();
            vbsProcess.Close();

            Globals.DeleteFile("CreateShortcut.vbs");
        }

        /// <summary>
        /// Delete shortcut if it exists
        /// </summary>
        /// <param name="delFolder">(Optional) Whether to delete parent folder if it's empty</param>
        public void DeleteShortcut(bool delFolder)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.DeleteShortcut] delFolder={delFolder}");
            Globals.DeleteFile(ShortcutPath);
            if (!delFolder) return;
            if (Directory.GetFiles(ShortcutDir()).Length == 0)
                Directory.Delete(ShortcutDir());
        }

        /// <summary>
        /// Toggles whether a shortcut exists (Creates/Deletes depending on state)
        /// </summary>
        /// <param name="shouldExist">Whether the shortcut SHOULD exist</param>
        /// <param name="shouldFolderExist">Whether the shortcut ALREADY Exists</param>
        public void ToggleShortcut(bool shouldExist, bool shouldFolderExist = true)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.ToggleShortcut] shouldExist={shouldExist}, shouldFolderExist={shouldFolderExist}");
            switch (shouldExist)
            {
                case true when !ShortcutExist():
                    WriteShortcut();
                    break;
                case false when ShortcutExist():
                    DeleteShortcut(!shouldFolderExist);
                    break;
            }
        }

        /// <summary>
        /// Write shortcut to file if doesn't already exist
        /// </summary>
        public void TryWrite()
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.TryWrite]");
            if (!ShortcutExist()) WriteShortcut();
        }

        #region PROGRAM_SHORTCUTS
        /// <summary>
        /// Fills in necessary info to create a shortcut to the TcNo Account Switcher
        /// </summary>
        /// <param name="location"></param>
        /// <returns></returns>
        public Shortcut Shortcut_Switcher(string location)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Switcher] location={location}");
            // Starts the main picker, with the Steam argument.
            Exe = GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Join(WorkingDir, "wwwroot\\prog_icons\\program.ico"); // To change soon
            ShortcutPath = Path.Join(location, "TcNo Account Switcher.lnk");
            Desc = "TcNo Account Switcher";
            Args = "";
            return this;
        }

        /// <summary>
        /// Sets up a Steam Tray shortcut
        /// </summary>
        /// <param name="location">Place to put shortcut</param>
        /// <returns></returns>
        public Shortcut Shortcut_Tray(string location)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Tray] location={location}");
            Exe = Path.Join(ParentDirectory(GetSelfPath()), "TcNo-Acc-Switcher-Tray.exe");
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Join(WorkingDir, "wwwroot\\prog_icons\\program.ico");
            ShortcutPath = Path.Join(location, "TcNo Account Switcher - Tray.lnk");
            Desc = "TcNo Account Switcher - Tray";
            Args = "";
            return this;
        }

        /// <summary>
        /// Creates an icon file with multiple sizes, and combines the BG and FG images.
        /// </summary>
        /// <param name="bgImg">Background image, platform</param>
        /// <param name="fgImg">Foreground image, user image</param>
        /// <param name="iconName">Filename, unique so stored without being overwritten</param>
        [SupportedOSPlatform("windows")]
        public async Task CreateCombinedIcon(string bgImg, string fgImg, string iconName)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.CreateCombinedIcon] bgImg={bgImg}, fgImg={fgImg.Substring(fgImg.Length - 6, 6)}, iconName=hidden");
            try
            {
                CreateIcon(bgImg, fgImg, ref iconName);
                IconDir = Path.GetFullPath(iconName);
            }
            catch (Exception e)
            {
                Globals.WriteToLog($"Failed to CreateIcon! '{bgImg}', '{fgImg}, '{iconName}'", e);
                await GeneralFuncs.ShowToast("error", Lang["Toast_FailedCreateIcon"]);
            }
        }
        #endregion

        #region SPECIFIC_SHORTCUTS
        // Usage:
        // var s = new Shortcut();
        // s.Shortcut_Platform(Shortcut.Desktop, platformName = "Steam", args = "steam");
        // s.ToggleShortcut(!DesktopShortcut, true);

        /// <summary>
        /// Sets up a platform specific shortcut
        /// </summary>
        /// <param name="location">Place to put shortcut</param>
        /// <param name="platformName">(Optional) Name of the platform this shortcut is for (eg. steam)</param>
        /// <param name="args">(Optional) Arguments to add, default "steam" to open Steam page of switcher</param>
        /// <param name="descAdd">(Optional) Additional description to add to "TcNo Account Switcher - Steam</param>
        /// <param name="platformNameIsFullName">Whether the platformName is the fill name, or just to be appended</param>
        /// <returns></returns>
        public Shortcut Shortcut_Platform(string location, string platformName = "Steam", string args = "steam", string descAdd = "", bool platformNameIsFullName = false)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Platform] location={location}, platformName={platformName}, descAdd={descAdd}, args={args}");
            // Starts the main picker, with the platform argument, eg: "steam", "origin".
            Exe = GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Join(WorkingDir, "wwwroot\\prog_icons\\program.ico"); // TODO: May add platform specific icons here at some point.
            ShortcutPath = Path.Join(location, (platformNameIsFullName ? platformName : $"{platformName} - TcNo Account Switcher") + ".lnk");
            Desc = $"TcNo Account Switcher - {platformName}" + descAdd != "" ? descAdd : "";
            Args = args;
            return this;
        }
        #endregion

        #region SHORTCUTS_INTERFACE
        public bool CheckShortcuts(string platform)
        {
            Globals.DebugWriteLine(@$"[Func:Data\Settings\Shared.CheckShortcuts] platform={platform}");
            AppSettings.CheckShortcuts();
            return File.Exists(Path.Join(Desktop, $"{platform} - TcNo Account Switcher.lnk"));
        }

        public void DesktopShortcut_Toggle(string platform, bool desktopShortcut)
        {
            Globals.DebugWriteLine(@$"[Func:Data\Settings\Shared.DesktopShortcut_Toggle] platform={platform}");
            var s = new Shortcut();
            _ = s.Shortcut_Platform(Desktop, platform, AppSettings.GetPlatform(platform).Identifier);
            s.ToggleShortcut(!desktopShortcut);
        }
        #endregion

        #region START_WITH_WINDOWS
        /// <summary>
        /// Checks if the TcNo Account Switcher Tray application has a Task to start with Windows
        /// </summary>
        public bool StartWithWindows_Enabled() => File.Exists(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.Startup), "TcNo Account Switcher - Tray.lnk"));

        /// <summary>
        /// Toggles whether the TcNo Account Switcher Tray application starts with Windows or not
        /// </summary>
        /// <param name="shouldExist">Whether it should start with Windows, or not</param>
        public void StartWithWindows_Toggle(bool shouldExist)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Task.StartWithWindows_Toggle] shouldExist={shouldExist}");
            var s = new Shortcut();
            _ = s.Shortcut_Tray(Environment.GetFolderPath(Environment.SpecialFolder.Startup));
            s.ToggleShortcut(shouldExist);
        }
        #endregion


        #region FACTORY
        // https://stackoverflow.com/a/32530019/5165437
        public const int MaxIconWidth = 256;
        public const int MaxIconHeight = 256;

        private const ushort HeaderReserved = 0;
        private const ushort HeaderIconType = 1;
        private const byte HeaderLength = 6;

        private const byte EntryReserved = 0;
        private const byte EntryLength = 16;

        private const byte PngColoursInPalette = 0;
        private const ushort PngColorPlanes = 1;

        /// <summary>
        /// Saves the specified <see cref="Bitmap"/> objects as a single
        /// icon into the output stream.
        /// </summary>
        /// <param name="images">The bitmaps to save as an icon.</param>
        /// <param name="stream">The output stream.</param>
        [SupportedOSPlatform("windows")]
        public void SavePngsAsIcon(IEnumerable<Bitmap> images, Stream stream)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.SavePngsAsIcon]");
            if (!OperatingSystem.IsWindows()) return;
            if (images == null)
                throw new ArgumentNullException(nameof(images));
            if (stream == null)
                throw new ArgumentNullException(nameof(stream));
            var enumerable = images as Bitmap[] ?? images.ToArray();
            ThrowForInvalidPng(enumerable);
            var orderedImages = enumerable.OrderBy(i => i.Width)
                                           .ThenBy(i => i.Height)
                                           .ToArray();
            using var writer = new BinaryWriter(stream);
            writer.Write(HeaderReserved);
            writer.Write(HeaderIconType);
            writer.Write((ushort)orderedImages.Length);
            var buffers = new Dictionary<uint, byte[]>();
            uint lengthSum = 0;
            var baseOffset = (uint)(HeaderLength +
                                    EntryLength * orderedImages.Length);
            foreach (var image in orderedImages)
            {
                var buffer = CreateImageBuffer(image);
                var offset = baseOffset + lengthSum;
                writer.Write(GetIconWidth(image));
                writer.Write(GetIconHeight(image));
                writer.Write(PngColoursInPalette);
                writer.Write(EntryReserved);
                writer.Write(PngColorPlanes);
                writer.Write((ushort)Image.GetPixelFormatSize(image.PixelFormat));
                writer.Write((uint)buffer.Length);
                writer.Write(offset);
                lengthSum += (uint)buffer.Length;
                buffers.Add(offset, buffer);
            }
            foreach (var (key, value) in buffers)
            {
                _ = writer.BaseStream.Seek(key, SeekOrigin.Begin);
                writer.Write(value);
            }
        }

        private void ThrowForInvalidPng(IEnumerable<Bitmap> images)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.ThrowForInvalidPng]");
            if (!OperatingSystem.IsWindows()) return;

            foreach (var image in images)
            {
                if (image.PixelFormat != PixelFormat.Format32bppArgb)
                {
                    throw new InvalidOperationException
                        (Lang["PngInvalid_Format", new { x = PixelFormat.Format32bppArgb }]);
                }
                if (image.RawFormat.Guid != ImageFormat.Png.Guid)
                {
                    throw new InvalidOperationException
                        (Lang["PngRequired"]);
                }
                if (image.Width > MaxIconWidth ||
                    image.Height > MaxIconHeight)
                {
                    throw new InvalidOperationException
                        (Lang["PngDimensions", new { MaxIconWidth, MaxIconHeight }]); // VS/JetBrains suggested to remove X=X, Y=Y and 'simplify'. Idk if this broke something before.
                }
            }
        }

        [SupportedOSPlatform("windows")]
        private byte GetIconHeight(Image image)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.GetIconHeight]");
            if (image.Height == MaxIconHeight)
                return 0;
            return (byte)image.Height;
        }

        [SupportedOSPlatform("windows")]
        private byte GetIconWidth(Image image)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.GetIconWidth]");
            if (image.Width == MaxIconWidth)
                return 0;
            return (byte)image.Width;
        }

        [SupportedOSPlatform("windows")]
        private byte[] CreateImageBuffer(Image image)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.CreateImageBuffer]");
            using var stream = new MemoryStream();
            image.Save(stream, image.RawFormat);
            return stream.ToArray();
        }

        #endregion

        /// <summary>
        /// Creates multiple images that are added to a single .ico file.
        /// Images are a combination of the platform's icon and the user accounts icon, for a good shortcut icon.
        /// </summary>
        /// <param name="sBgImg">Background image (platform's image)</param>
        /// <param name="sFgImg">User's profile image, for the foreground</param>
        /// <param name="icoOutput">Output filename</param>
        [SupportedOSPlatform("windows")]
        public void CreateIcon(string sBgImg, string sFgImg, ref string icoOutput)
        {
            var ms = new MemoryStream();
            // If input is SVG, create PNG
            // AND If override PNG does NOT exists:
            if (sBgImg.EndsWith(".svg") && !File.Exists(sBgImg.Replace(".svg", ".png")))
            {
                // If background doesn't exist:
                if (!File.Exists(sBgImg))
                {
                    var fallbackBg = Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png");
                    // If fallback file doesn't exist, or has already been re-run, fail:
                    if (!File.Exists(fallbackBg) || fallbackBg == sBgImg)
                    {
                        Globals.WriteToLog($"Failed to CreateIcon! File does not exist: '{sBgImg}'. Other requested file: '{sFgImg}'");
                        _ = GeneralFuncs.ShowToast("error", Lang["Toast_FailedCreateIcon"]);
                        return;
                    }
                    // Else:
                    sBgImg = fallbackBg;
                    CreateIcon(sBgImg, sFgImg, ref icoOutput);
                }

                // Load SVG into memory.
                var svgContent = File.ReadAllText(sBgImg);
                if (svgContent.Contains("id=\"FG\""))
                {
                    // Set color of foreground content, and insert background image
                    svgContent = svgContent.Replace("<path id=\"FG\"", $"<rect fill=\"{AppSettings.TryGetStyle("platformLogoBackground")}\" width=\"500\" height=\"500\"></rect>\"<path id=\"FG\" fill=\"{AppSettings.TryGetStyle("platformLogoForeground")}\"");
                    // Add the glass effect
                    svgContent = svgContent.Replace("/></svg>", "/><path d=\"M500, 0L0, 0L0, 500L500, 0Z\" fill=\"#FFFFFF\" fill-opacity=\"0.02\"/></svg>");
                }

                using var svg = new SKSvg();
                if (svg.FromSvg(svgContent) is { })
                {
                    //var tempBg =  Path.Join("temp", Path.GetFileNameWithoutExtension(sBgImg) + ".png");
                    //using var svgToPngStream = File.OpenWrite(tempBg);
                    _ = svg.Save(ms, new SKColor(0, 0, 0, 255));
                    //sBgImg = tempBg;
                }
            }
            else
            {
                using var file = new FileStream(sBgImg, FileMode.Open, FileAccess.Read);
                file.CopyTo(ms);
            }


            if (!File.Exists(sFgImg))
                sFgImg = Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png");

            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.CreateIcon]");
            using var ms16 = new MemoryStream();
            using var ms32 = new MemoryStream();
            using var ms48 = new MemoryStream();
            using var ms256 = new MemoryStream();
            CreateImage(ms, sFgImg, ms16, new Size(16, 16));
            CreateImage(ms, sFgImg, ms32, new Size(32, 32));
            CreateImage(ms, sFgImg, ms48, new Size(48, 48));
            CreateImage(ms, sFgImg, ms256, new Size(256, 256));

            _ = Directory.CreateDirectory("IconCache");
            icoOutput = Path.Join("IconCache", icoOutput);
            using var stream = new FileStream(icoOutput, FileMode.Create);
            SavePngsAsIcon(new[] { new Bitmap(ms16), new Bitmap(ms32), new Bitmap(ms48), new Bitmap(ms256) }, stream);
        }

        /// <summary>
        /// Creates a combination of the platform's icon and the user accounts icon, for a good shortcut icon.
        /// </summary>
        /// <param name="sBgImg">Background image (platform's image)</param>
        /// <param name="sFgImg">User's profile image, for the foreground</param>
        /// <param name="output">Output filename</param>
        /// <param name="imgSize">Requested dimensions for the image</param>
        public void CreateImage(MemoryStream sBgImg, string sFgImg, MemoryStream output, Size imgSize)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.CreateImage]");
            _ = sBgImg.Seek(0, SeekOrigin.Begin);
            using MagickImage bgImg = new(sBgImg);
            using MagickImage fgImg = new(sFgImg);
            bgImg.Resize(imgSize.Width, imgSize.Height);
            fgImg.Resize(imgSize.Width / 2, imgSize.Height / 2);

            bgImg.Composite(fgImg, Gravity.Southeast, CompositeOperator.Copy);
            bgImg.Write(output, MagickFormat.Png32);
            _ = output.Seek(0, SeekOrigin.Begin);
        }

        public async Task CreatePlatformShortcut()
        {
            var platform = AppSettings.GetPlatform(AppData.SelectedPlatform);

            Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralFuncs.GiCreatePlatformShortcut] platform={platform}");
            var s = new Shortcut();
            _ = s.Shortcut_Platform(Desktop, platform.Name, platform.Identifier);
            s.ToggleShortcut(true);

            await GeneralFuncs.ShowToast("success", Lang["Toast_ShortcutCreated"], Lang["Success"], "toastarea");
        }
    }
}
