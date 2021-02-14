using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows.Data;

namespace TcNo_Acc_Switcher_Client.Converters
{
    public class IntToStringConverter : IValueConverter
    {
        public object Convert(object value, Type targetType, object parameter, CultureInfo culture) => ((int)value).ToString();

        public object ConvertBack(object value, Type targetType, object parameter, CultureInfo culture) => int.Parse((string)value);
    }
}
