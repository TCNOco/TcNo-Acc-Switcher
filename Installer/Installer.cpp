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

// Just note that this requires curl to be installed. I used "vcpkg install curl[openssl]:x64-windows-static" in Microsoft's vcpkg to accomplish that.

#pragma comment(lib, "urlmon.lib")
#pragma comment (lib, "User32.lib")
#define CRT_SECURE_NO_WARNINGS
#include <algorithm>
#include <chrono>
#include <fstream>
#include <iostream>
#include <vector>
#include <Windows.h>
#include <conio.h>

#include <iostream>
#include <string>
#include <tchar.h>
#include <urlmon.h>
#include "progress_bar.hpp"

#include <curl/curl.h>
#include <openssl/ssl.h>
using namespace std;

string required_min_webview = "91.0",
	required_min_desktop_runtime = "5.0.7",
    required_min_aspcore = "5.0.7",
	required_min_vc = "14.30.30704";


std::string getOperatingPath() {
	const HMODULE h_module = GetModuleHandleW(nullptr);
    WCHAR pth[MAX_PATH];
    GetModuleFileNameW(h_module, pth, MAX_PATH);
    wstring ws(pth);
	const string path(ws.begin(), ws.end());
    return path.substr(0, path.find_last_of('\\') + 1);
}

void insert_empty_line()
{
    CONSOLE_SCREEN_BUFFER_INFO buffer_info;
    GetConsoleScreenBufferInfo(GetStdHandle(STD_OUTPUT_HANDLE), &buffer_info);

	std::string s(static_cast<double>(buffer_info.srWindow.Right) - static_cast<double>(buffer_info.srWindow.Left), ' ');
    std::cout << s << '\r';
}

double round_off(const double n) {
    double d = n * 100.0;
    const int i = d + 0.5;
    d = static_cast<float>(i) / 100.0;
    return d;
}

size_t write_data(void* ptr, const size_t size, size_t n_mem_b, FILE* stream) {
	const size_t written = fwrite(ptr, size, n_mem_b, stream);
    return written;
}

string convert_size(size_t size) {
    static const char* sizes[] = { "B", "KB", "MB", "GB" };
    int div = 0;
    size_t rem = 0;

    while (size >= 1024 && div < (sizeof sizes / sizeof * sizes)) {
        rem = (size % 1024);
        div++;
        size /= 1024;
    }

    const double size_d = static_cast<float>(size) + static_cast<float>(rem) / 1024.0;
    string result = to_string(round_off(size_d)) + " " + sizes[div];
    return result;
}

string current_download;
chrono::time_point<chrono::system_clock> last_time = std::chrono::system_clock::now();
int progress_bar(
	void* client_progress_data,
	const double dl_total,
	const double dl_now,
    double ul_total,
    double ul_now)
{
	if (const double n = dl_total; n > 0) {
		const string dls = "Downloading " + current_download + " (" + convert_size(static_cast<size_t>(dl_total)) + ")";
        auto bar1 = new ProgressBar(static_cast<unsigned long>(n), dls.c_str());
        if (const std::chrono::duration<double> elapsed_seconds = std::chrono::system_clock::now() - last_time; elapsed_seconds.count() >= 0.2)
        {
            last_time = std::chrono::system_clock::now();
            bar1->Progressed(static_cast<unsigned long>(dl_now));
        }
    }
    return 0;
}

bool download_file(const char* url, const char* dest) {
	if (CURL * curl = curl_easy_init(); curl) {
	    FILE* fp = fopen(dest, "wb");
        curl_easy_setopt(curl, CURLOPT_URL, url);
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_data);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, fp);
        curl_easy_setopt(curl, CURLOPT_NOPROGRESS, 0L);
        curl_easy_setopt(curl, CURLOPT_PROGRESSFUNCTION, progress_bar);
        curl_easy_setopt(curl, CURLOPT_FRESH_CONNECT, true);
        curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, false);
        curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, true); // Follows 301 redirects, like in the WebView2 link
	    const CURLcode res = curl_easy_perform(curl);
        /* always cleanup */
        curl_easy_cleanup(curl);
        fclose(fp);

        insert_empty_line();

        last_time = std::chrono::system_clock::from_time_t(0);
        cout << " Finished downloading " << current_download << endl;
        return res == CURLE_OK;
    }
    return false;
}

inline bool file_exists(const std::string& name) {
    struct stat buffer;
    return stat(name.c_str(), &buffer) == 0;
}



bool min_vc_met = false,
	min_webview_met = false,
	min_desktop_runtime_met = false,
	min_aspcore_met = false;
bool compare_versions(string v1, string v2, const string& delimiter);
void find_installed_net_runtimes(bool x32);
void find_installed_c_runtimes();
wstring s2_ws(const string& s)
{
	const int s_length = static_cast<int>(s.length()) + 1;
	const int len = MultiByteToWideChar(CP_ACP, 0, s.c_str(), s_length, nullptr, 0);
	const auto buf = new wchar_t[len];
    MultiByteToWideChar(CP_ACP, 0, s.c_str(), s_length, buf, len);
    wstring r(buf);
    delete[] buf;
    return r;
}

void install_runtime(const string& path, const string& name, const bool& passive)
{
    cout << "Installing: " << name << endl;
    STARTUPINFO si = { sizeof(STARTUPINFO) };
    PROCESS_INFORMATION pi;

    wstring args;
	if (passive) args = L"/install /passive";

    CreateProcessW(s2_ws(path).c_str(), &args[0], nullptr,
        nullptr, 0, 0, nullptr, nullptr, &si, &pi);
    WaitForSingleObject(pi.hProcess, INFINITE);
}

void wait_for_input()
{
    _getch();
}

const bool test_mode = false;
const bool test_downloads = false;
const bool test_installs = false;
int main()
{
	// Goal of this application:
	// - Check for the existence of required runtimes, and install them if missing. [First run ever on a computer]
	// --- [Unlikely] Maybe in the future: Verify application folders, and handle zipping and sending error logs? Possibly just the first or none of these.
	const string operating_path = getOperatingPath();
    SetConsoleTitle(_T("TcNo Account Switcher - Runtime installer"));
    cout << "Welcome to the TcNo Account Switcher - Runtime installer" << endl <<
        "------------------------------------------------------------------------" << endl << endl <<
        "Currently installed runtimes:" << endl;

	/* Find installed runtimes */
    find_installed_net_runtimes(false);
    find_installed_c_runtimes();

    cout << "------------------------------------------------------------------------" << endl << endl;

    /* Warn about runtime downloads */
    if (!min_webview_met || !min_aspcore_met || !min_desktop_runtime_met || !min_vc_met || test_mode)
    {
        cout << "One or more runtimes were not installed:" << endl;
        int total = 0;
        if (!min_webview_met) {
            cout << " + Microsoft WebView2 Runtime [~1,70 MB + ~120 MB while installing]" << endl;
            total += 122;
        }
        if (!min_aspcore_met){
			cout << " + Microsoft .NET 5 Desktop Runtime [~52,3 MB]" << endl;
        	total += 52;
		}
        if (!min_desktop_runtime_met){
			cout << " + Microsoft Microsoft ASP.NET Core 5.0 Runtime [~7,90 MB]" << endl;
        	total += 8;
		}

        if (!min_vc_met) {
            cout << " + C++ Redistributable 2015-2022 [~24 MB]" << endl;
            total += 24;
        }

        cout << " = Total download size: ~" << total << " MB" << endl << endl <<
            "Press any key to start download..." << endl;

        wait_for_input();

        /* Download runtimes */
        bool w_runtime_install = false,
            d_runtime_install = false,
            a_runtime_install = false,
    		c_runtime_install = false;
    	const string runtime_folder = operating_path + "runtime_installers\\";
        const string w_runtime = "https://go.microsoft.com/fwlink/p/?LinkId=2124703",
            w_runtime_local = runtime_folder + "MicrosoftEdgeWebview2Setup.exe",
            w_runtime_name = "Microsoft WebView2 Runtime",
            d_runtime = "https://dotnetcli.azureedge.net/dotnet/WindowsDesktop/5.0.10/windowsdesktop-runtime-5.0.10-win-x64.exe",
            d_runtime_local = runtime_folder + "windowsdesktop-runtime-5.0.10-win-x64.exe",
            d_runtime_name = "Microsoft .NET 5 Desktop Runtime",
            a_runtime = "https://dotnetcli.azureedge.net/dotnet/aspnetcore/Runtime/5.0.10/aspnetcore-runtime-5.0.10-win-x64.exe",
            a_runtime_local = runtime_folder + "aspnetcore-runtime-5.0.10-win-x64.exe",
            a_runtime_name = "Downloading Microsoft ASP.NET Core 5.0 Runtime",
    		c_runtime = "https://aka.ms/vs/17/release/vc_redist.x64.exe",
            c_runtime_local = runtime_folder + "VC_redist.x64.exe",
            c_runtime_name = "Downloading C++ Redistributable 2015-2022";

        if (!(CreateDirectoryA(runtime_folder.c_str(), NULL) || ERROR_ALREADY_EXISTS == GetLastError()))
        {
            cout << "Failed to create folder: " << runtime_folder << endl;
            system("pause");
            return 1;
        }

        if (!min_webview_met || test_downloads)
        {
            current_download = w_runtime_name;
            if (!download_file(w_runtime.c_str(), w_runtime_local.c_str()))
            {
                cout << "Failed to download and install Microsoft WebView2 Runtime. To download: 1. Click the link below:" << endl <<
                    "https://dotnet.microsoft.com/download/dotnet/5.0/runtime" << endl <<
                    "2. Click 'Download Hosting Bundle' under Run server apps." << endl << endl;
            }
            else w_runtime_install = true;
        }

        if (!min_desktop_runtime_met || test_downloads)
        {
            current_download = d_runtime_name;
            if (!download_file(d_runtime.c_str(), d_runtime_local.c_str()))
            {
                cout << "Failed to download and install .NET 5 Desktop Runtime. Please download it here:" << endl <<
                    "https://go.microsoft.com/fwlink/p/?LinkId=2124703" << endl << endl;
            }
            else d_runtime_install = true;
        }

        if (!min_aspcore_met || test_downloads)
        {
            current_download = a_runtime_name;
            if (!download_file(a_runtime.c_str(), a_runtime_local.c_str()))
            {
                cout << "Failed to download and install ASP.NET Core 5.0 Runtime. To download: 1. Click the link below:" << endl <<
                    "https://dotnet.microsoft.com/download/dotnet/5.0/runtime" << endl <<
                    "2. Click 'Download x64' under Run Desktop apps." << endl << endl;
            }
            else a_runtime_install = true;
        }

        if (!min_vc_met || test_downloads)
        {
            current_download = c_runtime_name;
            if (!download_file(c_runtime.c_str(), c_runtime_local.c_str()))
            {
                cout << "Failed to download and install C++ Redistributable 2015-2022. To download: 1. Click the link below:" << endl <<
                    "https://docs.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist?view=msvc-170" << endl <<
                    "2. Click the link next to 'X64', under the \"Visual Studio 2015, 2017, 2019, and 2022\" heading." << endl << endl;
            }
            else c_runtime_install = true;
        }

        cout << endl;

    	if (w_runtime_install || d_runtime_install || a_runtime_install || c_runtime_install || test_installs)
    	{
            cout << "------------------------------------------------------------------------" << endl << endl <<
                "One or more are ready for install. If and when prompted if you would like to install, click 'Yes'." << endl << endl <<
                "Press any key to start install..." << endl;
            wait_for_input();

            if (w_runtime_install || test_installs)
                install_runtime(w_runtime_local, w_runtime_name, false); // Does not support "/passive"

            if (d_runtime_install || test_installs)
                install_runtime(d_runtime_local, d_runtime_name, true);

            if (a_runtime_install || test_installs)
                install_runtime(a_runtime_local, a_runtime_name, true);

            if (c_runtime_install || test_installs)
                install_runtime(c_runtime_local, c_runtime_name, true);
    	}

        system("cls");
        cout << "Currently installed runtimes:" << endl;
        find_installed_net_runtimes(false);
        find_installed_c_runtimes();
        cout << "------------------------------------------------------------------------" << endl << endl <<
            "Verify you meet the minimum recommended requirements:" << endl;
    }
	else
    {
    cout << "It looks like everything is installed. Verify you meet the minimum recommended requirements:" << endl;
    }

    cout << " - Windows Desktop Runtime 5.0.7+" << endl <<
        " - ASP.NET Core 5.0.7+." << endl <<
        " - Edge WebView2 Runtime 91.0+" << endl <<
        " - C++ Redistributable 2015-2022 14.30.30704+" << endl <<
        "------------------------------------------------------------------------" << endl << endl <<
        "That should be everything. The main program, TcNo-Acc-Switcher.exe, will auto-run when you press a key to continue." << endl << endl <<
        "If it doesn't work, please refer to install instructions, here:" << endl <<
        "https://github.com/TcNobo/TcNo-Acc-Switcher#required-runtimes-download-and-install" << endl << endl;

    system("pause");

	// Launch main program:
	string main_path = operating_path + "TcNo-Acc-Switcher.exe";
	STARTUPINFO si = { sizeof(STARTUPINFO) };
	PROCESS_INFORMATION pi;
    CreateProcess(s2_ws(main_path).c_str(), nullptr, nullptr,
        nullptr, 0, 0, nullptr, nullptr, &si, &pi);
}

/// <summary>
/// Returns whether v2 is newer than, or equal to v1.
/// Works with unequal string sizes.
/// </summary>
bool compare_versions(string v1, string v2, const string& delimiter)
{
    if (v1 == v2) return true;
    vector<int> v1_arr;
    vector<int> v2_arr;

    size_t pos = 0;
    std::string token;
    while ((pos = v1.find(delimiter)) != std::string::npos) {
        token = v1.substr(0, pos);
        v1_arr.push_back(stoi(token));
        if (v1.find(delimiter) != std::string::npos) v1.erase(0, pos + delimiter.length());
    }
    v1_arr.push_back(stoi(v1));

    while ((pos = v2.find(delimiter)) != std::string::npos) {
        token = v2.substr(0, pos);
        v2_arr.push_back(stoi(token));
        if (v2.find(delimiter) != std::string::npos) v2.erase(0, pos + delimiter.length());
    }
    v2_arr.push_back(stoi(v2));

    for (int i = 0; i < min(v1_arr.size(), v2_arr.size()); ++i)
    {
        if (v2_arr[i] < v1_arr[i]) return false;
        if (v2_arr[i] > v1_arr[i]) return true;
    }

    return true;
}

void find_installed_c_runtimes()
{
    // Get C+++ Runtime info
    HKEY key = nullptr;
    WCHAR s_key[1024];
    DWORD dw_type = KEY_ALL_ACCESS;
    WCHAR version[1024];
    DWORD dw_v_buffer_size = sizeof(version);
    const auto subkey = L"SOFTWARE\\Microsoft\\DevDiv\\VC\\Servicing\\14.0\\RuntimeMinimum";
    if (RegOpenKeyEx(HKEY_LOCAL_MACHINE, subkey, 0, KEY_READ, &key) != ERROR_SUCCESS)
    {
        RegCloseKey(key);
        return;
    };

    if (RegQueryValueEx(key, L"Version", nullptr, &dw_type, reinterpret_cast<unsigned char*>(version), &dw_v_buffer_size) == ERROR_SUCCESS)
	{
        wprintf(L" - C++ Redistributable 2015-2022 [%s]\n", version);
        const string s_version(std::begin(version), std::end(version));
        min_vc_met = compare_versions(required_min_vc, string(s_version), ".");
    }
    RegCloseKey(key);
}
void find_installed_net_runtimes(const bool x32)
{
    // Get .NET info
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
	if ((x32 && RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_root1, 0, KEY_READ, &h_uninst_key) != ERROR_SUCCESS) ||
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
	        WCHAR version[1024];
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
            dw_v_buffer_size = sizeof(version);
            if (RegQueryValueEx(h_app_key, L"DisplayName", nullptr, &dw_type, reinterpret_cast<unsigned char*>(s_display_name), &dw_buffer_size) == ERROR_SUCCESS &&
                RegQueryValueEx(h_app_key, L"DisplayVersion", nullptr, &dw_type, reinterpret_cast<unsigned char*>(version), &dw_v_buffer_size) == ERROR_SUCCESS)
            {
                const string s_version(std::begin(version), std::end(version));

                if (wcsstr(s_display_name, L"WebView2") != nullptr)
                {
                    min_webview_met = min_webview_met || compare_versions(required_min_webview, string(s_version), ".");
                    wprintf(L" - %s ", s_display_name);
                    printf("[%s]\n", s_version.c_str());
                }

                if (wcsstr(s_display_name, L"Desktop Runtime") != nullptr && wcsstr(s_display_name, L"x64") != nullptr)
                {
                    min_desktop_runtime_met = min_desktop_runtime_met || compare_versions(required_min_desktop_runtime, string(s_version), ".");
                    wprintf(L" - %s\n", s_display_name);
                }

                if (wcsstr(s_display_name, L"ASP.NET Core 5") != nullptr)
                {
                    min_aspcore_met = min_aspcore_met || compare_versions(required_min_aspcore, string(s_version), ".");
                    wprintf(L" - %s\n", s_display_name);
                }
            }
            RegCloseKey(h_app_key);
        }
    }
    RegCloseKey(h_uninst_key);
}