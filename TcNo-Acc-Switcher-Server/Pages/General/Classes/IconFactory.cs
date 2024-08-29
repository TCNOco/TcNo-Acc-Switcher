// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Security.Permissions;
using ImageMagick;
using SkiaSharp;
using Svg.Skia;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Pages.General.Classes
{
    /// <summary>
    /// Creates icons
    /// </summary>
    public class IconFactory
    {
        private static readonly Lang Lang = Lang.Instance;

        #region FACTORY
        // https://stackoverflow.com/a/32530019/5165437
        public static readonly int MaxIconWidth = 256;
        public static readonly int MaxIconHeight = 256;

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
        public static void SavePngsAsIcon(IEnumerable<Bitmap> images, Stream stream)
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

        [SupportedOSPlatform("windows")]
        private static void ThrowForInvalidPng(IEnumerable<Bitmap> images)
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
        private static byte GetIconHeight(Image image)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.GetIconHeight]");
            if (image.Height == MaxIconHeight)
                return 0;
            return (byte)image.Height;
        }

        [SupportedOSPlatform("windows")]
        private static byte GetIconWidth(Image image)
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.GetIconWidth]");
            if (image.Width == MaxIconWidth)
                return 0;
            return (byte)image.Width;
        }

        [SupportedOSPlatform("windows")]
        private static byte[] CreateImageBuffer(Image image)
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
        public static void CreateIcon(string sBgImg, string sFgImg, ref string icoOutput)
        {
            var ms = new MemoryStream();
            // If input is SVG, create PNG
            // AND If override PNG does NOT exists:
            if (sBgImg.EndsWith(".svg") && !File.Exists(sBgImg.Replace(".svg",".png")))
            {
                // If background doesn't exist:
                if (!File.Exists(sBgImg))
                {
                    var fallbackBg = Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png");
                    // If fallback file doesn't exist, or has already been re-run, fail:
                    if (!File.Exists(fallbackBg) || fallbackBg == sBgImg)
                    {
                        Globals.WriteToLog($"Failed to CreateIcon! File does not exist: '{sBgImg}'. Other requested file: '{sFgImg}'");
                        _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedCreateIcon"]);
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
        public static void CreateImage(MemoryStream sBgImg, string sFgImg, MemoryStream output, Size imgSize)
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
    }
}
