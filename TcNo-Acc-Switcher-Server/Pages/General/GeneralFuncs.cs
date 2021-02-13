using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher.Pages.General
{
    public class GeneralFuncs
    {
        public static bool DeletedOutdatedImage(string filename)
        {
            if (DateTime.Now.Subtract(File.GetLastWriteTime(filename)).Days > 7)
            {
                File.Delete(filename);
                return true;
            }
            return false;
        }

        public static bool DeletedInvalidImage(string filename)
        {
            try
            {
                if (!IsValidGdiPlusImage(filename)) // Delete image if is not as valid, working image.
                {
                    File.Delete(filename);
                    return true;
                }
            }
            catch (Exception ex)
            {
                try
                {
                    File.Delete(filename);
                    return true;
                }
                catch (Exception)
                {
                    Console.WriteLine("Empty profile image detected (0 bytes). Can't delete to redownload.\nInfo: \n" + ex);
                    throw;
                }
            }

            return false;
        }
        private static bool IsValidGdiPlusImage(string filename)
        {
            //From https://stackoverflow.com/questions/8846654/read-image-and-determine-if-its-corrupt-c-sharp
            try
            {
                using (var bmp = new System.Drawing.Bitmap(filename))
                    return true;
            }
            catch
            {
                return false;
            }
        }
    }
}
