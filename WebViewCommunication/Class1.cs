using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;

namespace WebViewCommunication
{
    [ComVisible(true)]
    public sealed class Bridge
    {
        public Bridge()
        {
        }

        public void RetrieveScore(int score)
        {
            Debug.WriteLine("Score: " + score);
        }
    }
}
