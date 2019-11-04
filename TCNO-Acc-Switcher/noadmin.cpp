#include <windows.h>
#include <shldisp.h>
#include <shlobj.h>
#include <exdisp.h>
#include <atlbase.h>
#include <stdlib.h>
#include <iostream>
#include <system_error>

// Launch unelevated program from elevated program
// https://devblogs.microsoft.com/oldnewthing/20131118-00/?p=2643
// Based off of: https://stackoverflow.com/a/43768571

template< typename T >
void ThrowIfFailed(HRESULT hr, T&& msg)
{
	if (FAILED(hr))
		throw std::system_error{ hr, std::system_category(), std::forward<T>(msg) };
}

template< typename ResultT = std::string >
ResultT to_string(REFIID riid)
{
	LPOLESTR pstr = nullptr;
	if (SUCCEEDED(::StringFromCLSID(riid, &pstr)))
	{
		ResultT result{ pstr, pstr + wcslen(pstr) };
		::CoTaskMemFree(pstr); pstr = nullptr;
		return result;
	}
	return {};
}

struct ComInit
{
	ComInit() { ThrowIfFailed(::CoInitialize(nullptr), "Could not initialize COM"); }
	~ComInit() { ::CoUninitialize(); }
	ComInit(ComInit const&) = delete;
	ComInit& operator=(ComInit const&) = delete;
};
void FindDesktopFolderView(REFIID riid, void** ppv)
{
	CComPtr<IShellWindows> spShellWindows;
	ThrowIfFailed(
		spShellWindows.CoCreateInstance(CLSID_ShellWindows),
		"Could not create instance of IShellWindows");

	CComVariant vtLoc{ CSIDL_DESKTOP };
	CComVariant vtEmpty;
	long lhwnd = 0;
	CComPtr<IDispatch> spdisp;
	ThrowIfFailed(
		spShellWindows->FindWindowSW(
			&vtLoc, &vtEmpty, SWC_DESKTOP, &lhwnd, SWFO_NEEDDISPATCH, &spdisp),
		"Could not find desktop shell window");

	CComQIPtr<IServiceProvider> spProv{ spdisp };
	if (!spProv)
		ThrowIfFailed(E_NOINTERFACE, "Could not query interface IServiceProvider");

	CComPtr<IShellBrowser> spBrowser;
	ThrowIfFailed(
		spProv->QueryService(SID_STopLevelBrowser, IID_PPV_ARGS(&spBrowser)),
		"Could not query service IShellBrowser");

	CComPtr<IShellView> spView;
	ThrowIfFailed(
		spBrowser->QueryActiveShellView(&spView),
		"Could not query active IShellView");

	ThrowIfFailed(
		spView->QueryInterface(riid, ppv),
		"Could not query interface " + to_string(riid) + " from IShellView");
}

void GetDesktopAutomationObject(REFIID riid, void** ppv)
{
	CComPtr<IShellView> spsv;
	FindDesktopFolderView(IID_PPV_ARGS(&spsv));

	CComPtr<IDispatch> spdispView;
	ThrowIfFailed(
		spsv->GetItemObject(SVGIO_BACKGROUND, IID_PPV_ARGS(&spdispView)),
		"Could not get item object SVGIO_BACKGROUND from IShellView");
	ThrowIfFailed(
		spdispView->QueryInterface(riid, ppv),
		"Could not query interface " + to_string(riid) + " from ShellFolderView");
}

void ShellExecuteFromExplorer(
	PCWSTR pszFile,
	PCWSTR pszParameters = nullptr,
	PCWSTR pszDirectory = nullptr,
	PCWSTR pszOperation = nullptr,
	int nShowCmd = SW_SHOWNORMAL)
{
	CComPtr<IShellFolderViewDual> spFolderView;
	GetDesktopAutomationObject(IID_PPV_ARGS(&spFolderView));

	CComPtr<IDispatch> spdispShell;
	ThrowIfFailed(
		spFolderView->get_Application(&spdispShell),
		"Could not get application object from IShellFolderViewDual");

	CComQIPtr<IShellDispatch2> spdispShell2{ spdispShell };
	if (!spdispShell2)
		ThrowIfFailed(E_NOINTERFACE, "Could not query interface IShellDispatch2");

	ThrowIfFailed(
		spdispShell2->ShellExecute(
			CComBSTR{ pszFile },
			CComVariant{ pszParameters ? pszParameters : L"" },
			CComVariant{ pszDirectory ? pszDirectory : L"" },
			CComVariant{ pszOperation ? pszOperation : L"" },
			CComVariant{ nShowCmd }),
		"ShellExecute failed");
}

void startSteamNoAdmin(LPWSTR steam) {
	try
	{
		ComInit init;
		AllowSetForegroundWindow(ASFW_ANY);
		ShellExecuteFromExplorer(steam);
	}
	catch (std::system_error & e)
	{
		std::cout << "ERROR: " << e.what() << "\n"
			<< "Error code: " << e.code() << std::endl;
	}
}