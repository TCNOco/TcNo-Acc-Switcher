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
using System.Runtime.InteropServices;
using System.Runtime.Versioning;

namespace TcNo_Acc_Switcher_Globals
{
    public class IconExtractor
    {
        // SEE https://stackoverflow.com/a/28530403/5165437

        // private const string IidIImageList = "46EB5926-582E-4017-9FDF-E8998DAA0950";
        private const string IidIImageList2 = "192B9D83-50FC-457B-90A0-2B82A8B5DAE1";

        //Working example as of 2017/05/19 on windows 10 x64
        //example found here.
        //http://stackoverflow.com/a/28530403/1572750

        /// <summary>
        /// Get the icon assoicated with a given extension
        /// The size of the Image will be either 16x16,32x32,48x48, or 256x256 depending on which is choosen in the <paramref name="size"/> parameter
        /// </summary>
        /// <param name="ext">Extension of the file to get icon from</param>
        /// <param name="size">The size of the image of the icon you want back</param>
        /// <returns>Image in Png format</returns>
        [SupportedOSPlatform("windows")]
        public static Image GetPngFromExtension(string ext, IconSizes size)
        {
            ext = ext.Replace("*", "").Replace(".", ""); //clean the param up.

            var hIcon = GetIcon(GetIconIndex("*." + ext), size);
            Image result;
            // from native to managed
            try
            {
                using var ico = (Icon)Icon.FromHandle(hIcon).Clone();
                // save to stream to convert to png, then back.
                using var stream = new MemoryStream();
                ico.ToBitmap().Save(stream, ImageFormat.Png);
                result = Image.FromStream(stream);
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to get PNG from extension", e);
                return null;
            }
            finally
            {
                NativeMethods.DestroyIcon(hIcon); // don't forget to cleanup
            }

            return result;
        }

        [SupportedOSPlatform("windows")]
        public static Image GetPngFromExtension(string ext, ShIlIconSizes size)
        {
            ext = ext.Replace("*", "").Replace(".", "");
            var hIcon = GetIcon(GetIconIndex("*." + ext), size);
            Image result;
            // from native to managed
            using (var ico = (Icon)Icon.FromHandle(hIcon).Clone())
            {
                // save to stream to convert to png, then back.
                using (var stream = new MemoryStream())
                {
                    ico.ToBitmap().Save(stream, ImageFormat.Png);
                    result = Image.FromStream(stream);
                }
            }
            NativeMethods.DestroyIcon(hIcon); // don't forget to cleanup
            return result;
        }

        [SupportedOSPlatform("windows")]
        static IntPtr GetIcon(int iImage, IconSizes size)
        {
            IImageList spiml = null;
            var guil = new Guid(IidIImageList2);//or IID_IImageList

            NativeMethods.SHGetImageList((int)size, ref guil, ref spiml);
            var hIcon = IntPtr.Zero;
            spiml.GetIcon(iImage, NativeMethods.IldTransparent | NativeMethods.IldImage, ref hIcon);

            return hIcon;
        }

        static IntPtr GetIcon(int iImage, ShIlIconSizes size)
        {
            IImageList spiml = null;
            var guil = new Guid(IidIImageList2);//or IID_IImageList

            NativeMethods.SHGetImageList((int)size, ref guil, ref spiml);
            var hIcon = IntPtr.Zero;
            spiml.GetIcon(iImage, NativeMethods.IldTransparent | NativeMethods.IldImage, ref hIcon);

            return hIcon;
        }
        static int GetIconIndex(string pszFile)
        {
            var sfi = new SHFILEINFO();
            NativeMethods.SHGetFileInfo(pszFile
                , 0
                , ref sfi
                , (uint)Marshal.SizeOf(sfi)
                , (uint)(SHGFI.SysIconIndex | SHGFI.LargeIcon | SHGFI.UseFileAttributes));
            return sfi.iIcon;
        }


        // FROM https://stackoverflow.com/a/62207568/5165437
        private const uint LoadLibraryAsDatafile = 0x00000002;
        private static readonly IntPtr RtIcon = (IntPtr)3;
        private static readonly IntPtr RtGroupIcon = (IntPtr)14;

        [SupportedOSPlatform("windows")]
        public static Icon ExtractIconFromExecutable(string path)
        {
            var hModule = NativeMethods.LoadLibraryEx(path, IntPtr.Zero, LoadLibraryAsDatafile);
            var tmpData = new List<byte[]>();

            bool Callback(IntPtr h, IntPtr t, IntPtr name, IntPtr l)
            {
                var dir = GetDataFromResource(hModule, RtGroupIcon, name);

                // Calculate the size of an entire .icon file.

                int count = BitConverter.ToUInt16(dir, 4); // GRPICONDIR.idCount
                var len = 6 + 16 * count; // sizeof(ICONDIR) + sizeof(ICONDIRENTRY) * count
                for (var i = 0; i < count; ++i) len += BitConverter.ToInt32(dir, 6 + 14 * i + 8); // GRPICONDIRENTRY.dwBytesInRes

                using var dst = new BinaryWriter(new MemoryStream(len));
                // Copy GRPICONDIR to ICONDIR.

                dst.Write(dir, 0, 6);

                var picOffset = 6 + 16 * count; // sizeof(ICONDIR) + sizeof(ICONDIRENTRY) * count

                for (var i = 0; i < count; ++i)
                {
                    // Load the picture.

                    var id = BitConverter.ToUInt16(dir, 6 + 14 * i + 12); // GRPICONDIRENTRY.nID
                    var pic = GetDataFromResource(hModule, RtIcon, (IntPtr) id);

                    // Copy GRPICONDIRENTRY to ICONDIRENTRY.

                    dst.Seek(6 + 16 * i, 0);

                    dst.Write(dir, 6 + 14 * i, 8); // First 8bytes are identical.
                    dst.Write(pic.Length); // ICONDIRENTRY.dwBytesInRes
                    dst.Write(picOffset); // ICONDIRENTRY.dwImageOffset

                    // Copy a picture.

                    dst.Seek(picOffset, 0);
                    dst.Write(pic, 0, pic.Length);

                    picOffset += pic.Length;
                }

                tmpData.Add(((MemoryStream) dst.BaseStream).ToArray());

                return true;
            }

            NativeMethods.EnumResourceNames(hModule, RtGroupIcon, Callback, IntPtr.Zero);
            var iconData = tmpData.ToArray();
            using var ms = new MemoryStream(iconData[0]);
            return new Icon(ms);
        }
        private static byte[] GetDataFromResource(IntPtr hModule, IntPtr type, IntPtr name)
        {
            // Load the binary data from the specified resource.

            var hResInfo = NativeMethods.FindResource(hModule, name, type);

            var hResData = NativeMethods.LoadResource(hModule, hResInfo);

            var pResData = NativeMethods.LockResource(hResData);

            var size = NativeMethods.SizeofResource(hModule, hResInfo);

            var buf = new byte[size];
            Marshal.Copy(pResData, buf, 0, buf.Length);

            return buf;
        }
    }

    #region Definitions
    [ComImport]
    [Guid("46EB5926-582E-4017-9FDF-E8998DAA0950")]
    [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    internal interface IImageList
    {
        [PreserveSig]
        int Add(
        IntPtr hbmImage,
        IntPtr hbmMask,
        ref int pi);

        [PreserveSig]
        int Replace(
        int i,
        IntPtr hbmImage,
        IntPtr hbmMask);

        [PreserveSig]
        int Remove(int i);

        [PreserveSig]
        int GetIcon(
        int i,
        int flags,
        ref IntPtr picon);

        [PreserveSig]
        int Copy(
        int iDst,
        IImageList punkSrc,
        int iSrc,
        int uFlags);

        [PreserveSig]
        int Clone(
        ref Guid riid,
        ref IntPtr ppv);
    }


    [StructLayout(LayoutKind.Sequential)]
    internal struct IMAGEINFO
    {
        public IntPtr hbmImage;
        public IntPtr hbmMask;
        public int Unused1;
        public int Unused2;
        public Rect rcImage;
    }

    [StructLayout(LayoutKind.Sequential)]
    internal struct IMAGELISTDRAWPARAMS
    {
        public int cbSize;
        public IntPtr himl;
        public int i;
        public IntPtr hdcDst;
        public int x;
        public int y;
        public int cx;
        public int cy;
        public int xBitmap;    // x offest from the upperleft of bitmap
        public int yBitmap;    // y offset from the upperleft of bitmap
        public int rgbBk;
        public int rgbFg;
        public int fStyle;
        public int dwRop;
        public int fState;
        public int Frame;
        public int crEffect;
    }


    [StructLayout(LayoutKind.Sequential)]
    internal struct Point
    {
        private readonly int x;
        private readonly int y;
    }


    [StructLayout(LayoutKind.Sequential)]
    internal struct Rect
    {
        public int left, top, right, bottom;
    }


    public enum ShIlIconSizes
    {
        ShIlLarge = 0x0,
        ShIlSmall = 0x1,
        /*
        ShilExtraLarge = 0x2,
        ShIlSysSmall = 0x3,
        ShIlJumbo = 0x4,
        */
        ShIlLast = 0x4,
    }


    /// <summary>
    /// More user friend version of <see cref="ShIlIconSizes"/>, note this does not contain the "last" option, which would point to Jumbo.
    /// </summary>
    public enum IconSizes
    {
        /// <summary>
        /// SHIL_LARGE, 32x32
        /// </summary>
        Large = 0x0,
        /// <summary>
        /// SHIL_SMALL, 16x16
        /// </summary>
        Small = 0x1,
        /// <summary>
        /// SHIL_EXTRALARGE, 48x48
        /// </summary>
        ExtraLarge = 0x2,
        /// <summary>
        /// SHIL_SYSSMALL meaning system small, same size as small
        /// </summary>
        SystemSmall = 0x3,
        /// <summary>
        /// ShIlJumbo, 256x256
        /// </summary>
        Jumbo = 0x4,
    }


    [StructLayout(LayoutKind.Sequential)]
    internal struct SHFILEINFO
    {
        public const int NAMESIZE = 80;
        public IntPtr hIcon;
        public int iIcon;
        public uint dwAttributes;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 260)]
        public string szDisplayName;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 80)]
        public string szTypeName;
    }

    [Flags]
    internal enum SHGFI : uint
    {
        /// <summary>get icon</summary>
        Icon = 0x000000100,
        /// <summary>get display name</summary>
        DisplayName = 0x000000200,
        /// <summary>get type name</summary>
        TypeName = 0x000000400,
        /// <summary>get attributes</summary>
        Attributes = 0x000000800,
        /// <summary>get icon location</summary>
        IconLocation = 0x000001000,
        /// <summary>return exe type</summary>
        ExeType = 0x000002000,
        /// <summary>get system icon index</summary>
        SysIconIndex = 0x000004000,
        /// <summary>put a link overlay on icon</summary>
        LinkOverlay = 0x000008000,
        /// <summary>show icon in selected state</summary>
        Selected = 0x000010000,
        /// <summary>get only specified attributes</summary>
        AttrSpecified = 0x000020000,
        /// <summary>get large icon</summary>
        LargeIcon = 0x000000000,
        /// <summary>get small icon</summary>
        SmallIcon = 0x000000001,
        /// <summary>get open icon</summary>
        OpenIcon = 0x000000002,
        /// <summary>get shell size icon</summary>
        ShellIconSize = 0x000000004,
        /// <summary>pszPath is a pidl</summary>
        PIDL = 0x000000008,
        /// <summary>use passed dwFileAttribute</summary>
        UseFileAttributes = 0x000000010,
        /// <summary>apply the appropriate overlays</summary>
        AddOverlays = 0x000000020,
        /// <summary>Get the index of the overlay in the upper 8 bits of the iIcon</summary>
        OverlayIndex = 0x000000040,
    }
    #endregion
}
