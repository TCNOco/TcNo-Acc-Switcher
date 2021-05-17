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

#include <iostream>
#include <vector>
#include <Windows.h>
using namespace std;

struct software
{
    WCHAR* s_display_name;
    WCHAR* s_version;
};


vector<software> v_webview, v_aspcore, v_desktop_runtime;
void find_installed_runtimes(bool x32);

int main()
{
	// Goal of this application:
	// - Check for the existence of required runtimes, and install them if missing. [First run ever on a computer]
	// - Possibly verify application folders, and handle zipping and sending error logs? Possibly just the first or none of these.
	cout << "Checking for installed runtimes!\n";
    find_installed_runtimes(false);

    cout << "Checking versions of installed runtimes (if any)";
	// Loop through each of the vectors, and see if installed at all or up-to-date.
	// Download and install any that are missing.

	// Maybe verifies proram files, and downloads broken files?
	system("pause");
}


void find_installed_runtimes(const bool x32)
{
    // Find installed runtimes, and add them to the list
    const auto s_root1 = L"SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall";
    const auto s_root2 = L"SOFTWARE\\Wow6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall";
	
    HKEY h_uninst_key = nullptr;
    HKEY h_app_key = nullptr;
    long l_result = ERROR_SUCCESS;
    DWORD dw_type = KEY_ALL_ACCESS;
    DWORD dw_buffer_size = 0;
    DWORD dw_v_buffer_size = 0;

    //Open the "Uninstall" key.
	if (x32 && RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_root1, 0, KEY_READ, &h_uninst_key) != ERROR_SUCCESS ||
		RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_root2, 0, KEY_READ, &h_uninst_key) != ERROR_SUCCESS)
		return;

    for (DWORD dw_index = 0; l_result == ERROR_SUCCESS; dw_index++)
    {
	    WCHAR s_app_key_name[1024];
	    //Enumerate all sub keys...
        dw_buffer_size = sizeof(s_app_key_name);
        if ((l_result = RegEnumKeyEx(h_uninst_key, dw_index, s_app_key_name, &dw_buffer_size, nullptr, nullptr, nullptr,
                                     nullptr)) == ERROR_SUCCESS)
        {
	        WCHAR s_version[1024];
	        WCHAR s_display_name[1024];
	        WCHAR s_sub_key[1024];
	        //Open the sub key.
            if (x32) wsprintf(s_sub_key, L"%s\\%s", s_root1, s_app_key_name);
            else wsprintf(s_sub_key, L"%s\\%s", s_root2, s_app_key_name);
            if (RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_sub_key, 0, KEY_READ, &h_app_key) != ERROR_SUCCESS)
            {
                RegCloseKey(h_app_key);
                RegCloseKey(h_uninst_key);
                continue;
            }

            //Get the display name value from the application's sub key.
            dw_buffer_size = sizeof(s_display_name);
            dw_v_buffer_size = sizeof(s_version);
            if (RegQueryValueEx(h_app_key, L"DisplayName", nullptr, &dw_type, reinterpret_cast<unsigned char*>(s_display_name), &dw_buffer_size) == ERROR_SUCCESS &&
                RegQueryValueEx(h_app_key, L"DisplayVersion", nullptr, &dw_type, reinterpret_cast<unsigned char*>(s_version), &dw_v_buffer_size) == ERROR_SUCCESS)
            {
                string name(s_display_name, s_display_name + dw_buffer_size);
                string version(s_display_name, s_version + dw_v_buffer_size);
                            	
                if (name.find("WebView2") != std::string::npos)
                {
                    v_webview.push_back({ s_display_name, s_version });
                    wprintf(L"%s\n", s_display_name);
                }
            	
                if (name.find("Desktop Runtime") != std::string::npos && name.find("x64") != std::string::npos)
                {
                    v_desktop_runtime.push_back({ s_display_name, s_version });
                    wprintf(L"%s\n", s_display_name);
                }
            	
                if (name.find("ASP.NET Core") != std::string::npos)
                {
                    v_aspcore.push_back({ s_display_name, s_version });
                    wprintf(L"%s\n", s_display_name);
                }
            }
            else {
                //Display name value doe not exist, this application was probably uninstalled.
            }

            RegCloseKey(h_app_key);
        }
    }

    RegCloseKey(h_uninst_key);
}
