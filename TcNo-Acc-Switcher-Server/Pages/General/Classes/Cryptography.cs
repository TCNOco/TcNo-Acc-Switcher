using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Server.Pages.General.Classes
{
	public class Cryptography
	{
        // https://stackoverflow.com/a/10177020/2061103
        public static class StringCipher
        {
            // This constant is used to determine the key size of the encryption algorithm in bits.
            // We divide this by 8 within the code below to get the equivalent number of bytes.
            private const int KeySize = 128;

            // This constant determines the number of iterations for the password bytes generation function.
            private const int DerivationIterations = 1000;

            public static string EncryptString(string plainText, string passPhrase) =>
	            Convert.ToBase64String(Encrypt(Encoding.UTF8.GetBytes(plainText), passPhrase));

            public static byte[] Encrypt(byte[] data, string passPhrase)
            {
                // Salt and IV is randomly generated each time, but is prepended to encrypted cipher text
                // so that the same Salt and IV values can be used when decrypting.  
                var saltStringBytes = Generate128BitsOfRandomEntropy();
                var ivStringBytes = Generate128BitsOfRandomEntropy();
                using var password = new Rfc2898DeriveBytes(passPhrase, saltStringBytes, DerivationIterations);
                var keyBytes = password.GetBytes(KeySize / 8);

                using var symmetricKey = new RijndaelManaged
                {
	                BlockSize = 128, Mode = CipherMode.CBC, Padding = PaddingMode.PKCS7
                };

                using var encryptor = symmetricKey.CreateEncryptor(keyBytes, ivStringBytes);
                using var memoryStream = new MemoryStream();
                using var cryptoStream = new CryptoStream(memoryStream, encryptor, CryptoStreamMode.Write);
                cryptoStream.Write(data, 0, data.Length);
                cryptoStream.FlushFinalBlock();
                // Create the final bytes as a concatenation of the random salt bytes, the random iv bytes and the cipher bytes.
                var cipherTextBytes = saltStringBytes;
                cipherTextBytes = cipherTextBytes.Concat(ivStringBytes).ToArray();
                cipherTextBytes = cipherTextBytes.Concat(memoryStream.ToArray()).ToArray();
                memoryStream.Close();
                cryptoStream.Close();
                return cipherTextBytes;
            }

            public static string DecryptString(string cipherText, string passPhrase) => Encoding.UTF8.GetString(Decrypt(Convert.FromBase64String(cipherText), passPhrase));

            public static byte[] Decrypt(byte[] cipherTextBytesWithSaltAndIv, string passPhrase)
            {
                // cipherTextBytesWithSaltAndIv = [32 bytes of Salt] + [16 bytes of IV] + [n bytes of CipherText]
                // Get the salt bytes by extracting the first 16 bytes from the supplied cipherText bytes.
                var saltStringBytes = cipherTextBytesWithSaltAndIv.Take(KeySize / 8).ToArray();
                // Get the IV bytes by extracting the next 16 bytes from the supplied cipherText bytes.
                var ivStringBytes = cipherTextBytesWithSaltAndIv.Skip(KeySize / 8).Take(KeySize / 8).ToArray();
                // Get the actual cipher text bytes by removing the first 64 bytes from the cipherText string.
                var cipherTextBytes = cipherTextBytesWithSaltAndIv.Skip((KeySize / 8) * 2).Take(cipherTextBytesWithSaltAndIv.Length - ((KeySize / 8) * 2)).ToArray();

                using var password = new Rfc2898DeriveBytes(passPhrase, saltStringBytes, DerivationIterations);
                var keyBytes = password.GetBytes(KeySize / 8);

                using var symmetricKey = new RijndaelManaged
                {
	                BlockSize = 128, Mode = CipherMode.CBC, Padding = PaddingMode.PKCS7
                };

                using var decryptor = symmetricKey.CreateDecryptor(keyBytes, ivStringBytes);
                using var memoryStream = new MemoryStream(cipherTextBytes);
                using var cryptoStream = new CryptoStream(memoryStream, decryptor, CryptoStreamMode.Read);

                var outputBytes = new byte[cipherTextBytes.Length];
                var decryptedByteCount = cryptoStream.Read(outputBytes, 0, outputBytes.Length);
                memoryStream.Close();
                cryptoStream.Close();
                return outputBytes;
            }

            private static byte[] Generate128BitsOfRandomEntropy()
            {
                var randomBytes = new byte[16]; // 16 Bytes will give us 128 bits.
                using var rngCsp = new RNGCryptoServiceProvider();
                // Fill the array with cryptographically secure random bytes.
                rngCsp.GetBytes(randomBytes);
                return randomBytes;
            }


            public static bool DecryptFile(string path, string pass)
            {
	            if (!File.Exists(path)) return false;
	            try
	            {
		            File.WriteAllBytes(path, Cryptography.StringCipher.Decrypt(File.ReadAllBytes(path), pass));
		            return true;
	            }
	            catch (Exception)
	            {
		            return false;
	            }
            }
            public static bool EncryptFile(string path, string pass)
            {
	            if (!File.Exists(path)) return false;
	            try
	            {
		            File.WriteAllBytes(path, Cryptography.StringCipher.Encrypt(File.ReadAllBytes(path), pass));
		            return true;
	            }
	            catch (Exception)
	            {
		            return false;
	            }
            }
        }
    }
}
