// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
using System.Threading.Tasks;
using ImageMagick;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.General.Classes
{
    /// <summary>
    /// Creates icons
    /// </summary>
    public class IconFactory
    {

        #region FACTORY
        // https://stackoverflow.com/a/32530019/5165437
        public const int MaxIconWidth = 256;
        public const int MaxIconHeight = 256;

        private const ushort HeaderReserved = 0;
        private const ushort HeaderIconType = 1;
        private const byte HeaderLength = 6;

        private const byte EntryReserved = 0;
        private const byte EntryLength = 16;

        private const byte PngColorsInPalette = 0;
        private const ushort PngColorPlanes = 1;

        /// <summary>
        /// Saves the specified <see cref="Bitmap"/> objects as a single 
        /// icon into the output stream.
        /// </summary>
        /// <param name="images">The bitmaps to save as an icon.</param>
        /// <param name="stream">The output stream.</param>
        public static void SavePngsAsIcon(IEnumerable<Bitmap> images, Stream stream)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.SavePngsAsIcon]");
            if (images == null)
                throw new ArgumentNullException("images");
            if (stream == null)
                throw new ArgumentNullException("stream");
            IconFactory.ThrowForInvalidPngs(images);
            Bitmap[] orderedImages = images.OrderBy(i => i.Width)
                                           .ThenBy(i => i.Height)
                                           .ToArray();
            using var writer = new BinaryWriter(stream);
            writer.Write(IconFactory.HeaderReserved);
            writer.Write(IconFactory.HeaderIconType);
            writer.Write((ushort)orderedImages.Length);
            Dictionary<uint, byte[]> buffers = new Dictionary<uint, byte[]>();
            uint lengthSum = 0;
            uint baseOffset = (uint)(IconFactory.HeaderLength +
                                     IconFactory.EntryLength * orderedImages.Length);
            foreach (var image in orderedImages)
            {
                byte[] buffer = IconFactory.CreateImageBuffer(image);
                uint offset = (baseOffset + lengthSum);
                writer.Write(IconFactory.GetIconWidth(image));
                writer.Write(IconFactory.GetIconHeight(image));
                writer.Write(IconFactory.PngColorsInPalette);
                writer.Write(IconFactory.EntryReserved);
                writer.Write(IconFactory.PngColorPlanes);
                writer.Write((ushort)Image.GetPixelFormatSize(image.PixelFormat));
                writer.Write((uint)buffer.Length);
                writer.Write(offset);
                lengthSum += (uint)buffer.Length;
                buffers.Add(offset, buffer);
            }
            foreach (var kvp in buffers)
            {
                writer.BaseStream.Seek(kvp.Key, SeekOrigin.Begin);
                writer.Write(kvp.Value);
            }
        }

        private static void ThrowForInvalidPngs(IEnumerable<Bitmap> images)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.ThrowForInvalidPngs]");
            foreach (var image in images)
            {
                if (image.PixelFormat != PixelFormat.Format32bppArgb)
                {
                    throw new InvalidOperationException
                        (string.Format("Required pixel format is PixelFormat.{0}.",
                                       PixelFormat.Format32bppArgb.ToString()));
                }
                if (image.RawFormat.Guid != ImageFormat.Png.Guid)
                {
                    throw new InvalidOperationException
                        ("Required image format is a portable network graphic (png).");
                }
                if (image.Width > IconFactory.MaxIconWidth ||
                    image.Height > IconFactory.MaxIconHeight)
                {
                    throw new InvalidOperationException
                        (string.Format("Dimensions must be less than or equal to {0}x{1}",
                                       IconFactory.MaxIconWidth,
                                       IconFactory.MaxIconHeight));
                }
            }
        }

        private static byte GetIconHeight(Bitmap image)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.GetIconHeight]");
            if (image.Height == IconFactory.MaxIconHeight)
                return 0;
            return (byte)image.Height;
        }

        private static byte GetIconWidth(Bitmap image)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.GetIconWidth]");
            if (image.Width == IconFactory.MaxIconWidth)
                return 0;
            return (byte)image.Width;
        }

        private static byte[] CreateImageBuffer(Bitmap image)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.CreateImageBuffer]");
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
        public static void CreateIcon(string sBgImg, string sFgImg, ref string icoOutput)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.CreateIcon]");
            Directory.CreateDirectory("temp");
            var pngOutput = "temp" + icoOutput.Split(".ico")[0];
            CreateImage(sBgImg, sFgImg, $"{pngOutput}_16.png", new Size(16, 16));
            CreateImage(sBgImg, sFgImg, $"{pngOutput}_32.png", new Size(32, 32));
            CreateImage(sBgImg, sFgImg, $"{pngOutput}_48.png", new Size(48, 48));
            CreateImage(sBgImg, sFgImg, $"{pngOutput}_256.png", new Size(256, 256));

            using var png16 = (Bitmap)Image.FromFile($"{pngOutput}_16.png");
            using var png32 = (Bitmap)Image.FromFile($"{pngOutput}_32.png");
            using var png48 = (Bitmap)Image.FromFile($"{pngOutput}_48.png");
            using var png256 = (Bitmap)Image.FromFile($"{pngOutput}_256.png");

            Directory.CreateDirectory("IconCache");
            icoOutput = Path.Join("IconCache", icoOutput);
            using var stream = new FileStream(icoOutput, FileMode.Create);
            IconFactory.SavePngsAsIcon(new[] { png16, png32, png48, png256 }, stream);

            Directory.Delete("temp", true);
        }

        /// <summary>
        /// Creates a combination of the platform's icon and the user accounts icon, for a good shortcut icon.
        /// </summary>
        /// <param name="sBgImg">Background image (platform's image)</param>
        /// <param name="sFgImg">User's profile image, for the foreground</param>
        /// <param name="output">Output filename</param>
        /// <param name="imgSize">Requested dimensions for the image</param>
        public static void CreateImage(string sBgImg, string sFgImg, string output, Size imgSize)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.CreateImage]");
            using MagickImage bgImg = new MagickImage(sBgImg);
            using MagickImage fgImg = new MagickImage(sFgImg);
            bgImg.Resize(imgSize.Width, imgSize.Height);
            fgImg.Resize(imgSize.Width / 2, imgSize.Height / 2);

            bgImg.Composite(fgImg, Gravity.Southeast, CompositeOperator.Copy);
            bgImg.Write(output);
        }
    }
}
