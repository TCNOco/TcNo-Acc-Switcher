using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Data;
using System.Windows.Media;

namespace TcNo_Acc_Switcher_Steam.Converters
{
    public class TimestampToText : IValueConverter
    {
        public object Convert(object value, Type targetType, object parameter, CultureInfo culture)
        {
            if (!(value is string)) throw new InvalidOperationException();

            string localDateTimeOffset;
            try
            {
                localDateTimeOffset = DateTimeOffset.FromUnixTimeSeconds(long.Parse((string)value)).DateTime.ToLocalTime().ToString("dd/MM/yyyy hh:mm:ss");
            }
            catch
            {
                localDateTimeOffset = "-ERR-";
            }
            return localDateTimeOffset;
        }

        public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture)
        {
            throw new NotImplementedException();
        }
    }
}
