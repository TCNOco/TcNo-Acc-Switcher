// TCNO-Acc-Switcher.cpp : This file contains the 'main' function. Program execution begins and ends there.
//
#include <iostream>
#include <windows.h>   // WinApi header
#include <stdlib.h>
#include <string>
#include <vector>
#include <cctype>
#include <sstream>
#include <fstream>
#include <algorithm>    // For std::remove()
#include <conio.h>
#include <chrono>
#include <thread>
#include <atlstr.h>

// For unelevated program launch
#include "noadmin.h"

#define KEY_UP 72       //Up arrow character
#define KEY_DOWN 80     //Down arrow character
#define KEY_ENTER '\r'  //Enter key charatcer

using namespace std;    // std::cout, std::cin
bool firstprint = true;


vector<vector<string>> userAccounts; // Steam64ID, Username, Remember password <0,1>
HANDLE  hConsole;
string SteamFolder = "C:\\Program Files (x86)\\Steam\\",
	LoginUsersVDF = SteamFolder+"config\\loginusers.vdf",
	SteamEXE = SteamFolder+"Steam.exe";
std::vector<std::string> fLoginUsersLines;

bool getSteamAccounts() 
{
	std::ifstream fLoginUsers(LoginUsersVDF); // SOMEHOW READ AS UNICODE
	string curline, lineNoQuot;
	string username, steamID, rememberAccount, personaName;

	while (std::getline(fLoginUsers, curline)) {
		fLoginUsersLines.push_back(curline);

		// Remove tabs
		curline.erase(std::remove(curline.begin(), curline.end(), '\t'), curline.end());

		lineNoQuot = curline;
		lineNoQuot.erase(std::remove(lineNoQuot.begin(), lineNoQuot.end(), '"'), lineNoQuot.end()); // Remove inverted commas, to check if SteamID

		if (std::all_of(lineNoQuot.begin(), lineNoQuot.end(), ::isdigit)) // Check if line is JUST digits -> SteamID
		{
			steamID = lineNoQuot;
		}
		else if (curline.find("AccountName") != std::string::npos) // Line contains username
		{
			username = lineNoQuot.substr(11, lineNoQuot.length());
		}
		else if (curline.find("RememberPassword") != std::string::npos) // Line contains if password is remembered
		{
			rememberAccount = lineNoQuot.substr(lineNoQuot.length() - 1);
		}
		else if (curline.find("PersonaName") != std::string::npos) // Line contains if password is remembered
		{
			personaName = lineNoQuot.substr(11, lineNoQuot.length());
		}


		if (!username.empty() && !steamID.empty() && !rememberAccount.empty()) // If both username and steam ID are set
		{
			userAccounts.push_back({ username, steamID, rememberAccount, personaName });
			//cout << "---> Added: " << steamID << " - " << username << "  -- " << rememberAccount <<  endl; // DEBUG - Account found and added
			username = "";
			steamID = "";
			rememberAccount = "";
			personaName = "";
		}
		//std::cout << curline << std::endl; // DEBUG - Every line
	}
	userAccounts.push_back({ "Sign in with a new account.", "", "" });
	return true;
}
void printSteamAccs(int& selectedLine) {
	if (!firstprint) {
		system("CLS");
		cout << flush;
	}

	cout << "Welcome to TCNO Steam Account Switcher [https://tcno.co/]." << endl;
	cout << "How to use:" << endl <<
		"1. DO NOT change users or log out via Steam - You can exit as normal." << endl <<
		" - You can start steam as normal, just use this to change accounts." << endl <<
		"2. Check \"Remember password\" if asked for one." << endl <<
		"3. To sign in with a new account, select the last option." << endl << 
		"4. As soon as you hit enter, steam will force kill and reopen immediately! Be aware of your data." << endl <<
		"-- This program never accesses or asks for your actual password! --" << endl;

	cout << "Options [Arrow keys, Enter]:" << endl << endl;

	int curLinePrint = 0, accounts = userAccounts.size();
	for (vector<string> userAccount : userAccounts)
	{
		// Colours:
		// 7   -  White on Black
		// 240 - Black on White
		int k = 7;
		if (curLinePrint == selectedLine)
			k = 240;

		cout << "   - ";
		SetConsoleTextAttribute(hConsole, k); // If selected, change to color
		cout << userAccount[0];
		if (curLinePrint != accounts - 1) {
			cout << " (\"" << userAccount[3] << "\" " << userAccount[1] << ")";
			if (userAccount[2] == "1")
				cout << " - Saved";
		}
		SetConsoleTextAttribute(hConsole, 7); // Set to Black on White
		cout << endl;
		curLinePrint++;
	}
}

void mostRecentUpdate(vector<string> accountName) {
	// Name: 0, SteamID: 1
	// -----------------------------------
	// ----- Manage "loginusers.vdf" -----
	// -----------------------------------
	std::ofstream fLoginUsersOUT(LoginUsersVDF, ios::trunc);
	string curline, lineNoQuot;
	bool userIDMatch = false;
	string outline = "";
	for (string curline : fLoginUsersLines) {
		outline = curline;
		//break; // JUST FOR DEBUGGING


		// Do we even need this?
		lineNoQuot = curline;
		lineNoQuot.erase(std::remove(lineNoQuot.begin(), lineNoQuot.end(), '\t'), lineNoQuot.end()); // Remove tab spacing
		lineNoQuot.erase(std::remove(lineNoQuot.begin(), lineNoQuot.end(), '"'), lineNoQuot.end());  // Remove inverted commas, to check if SteamID

		if (std::all_of(lineNoQuot.begin(), lineNoQuot.end(), ::isdigit)) // Check if line is JUST digits -> SteamID
		{
			userIDMatch = false;
			if (lineNoQuot == accountName[1]) { // Most recent ID matches! Set this account to active.
				userIDMatch = true;
			}
		}
		else if (curline.find("mostrecent") != std::string::npos)
		{
			// Set ever=y mostrecent to 0, unless it's the one you want to switch to.
			//REPLACE LINE WITH:
			if (userIDMatch) {
				outline = "\t\t\"mostrecent\"\t\t\"1\"";
			}else{
				outline = "\t\t\"mostrecent\"\t\t\"0\"";
			}
		}
		fLoginUsersOUT << outline << endl;
	}
	fLoginUsersOUT.close();

	// -----------------------------------
	// --------- Manage registry ---------
	// -----------------------------------
	/*
	------------ Structure ------------
	HKEY_CURRENT_USER\Software\Valve\Steam\
		--> AutoLoginUser = username
		--> RememberPassword = 1
	*/
	HKEY hKey;
	LPCWSTR HKCUValve = L"Software\\\Valve\\Steam";

	RegOpenKeyEx(HKEY_CURRENT_USER, HKCUValve, 0, KEY_SET_VALUE, &hKey);

	// String to TCHAR
	TCHAR accname[64];
	_tcscpy_s(accname, CA2T(accountName[0].c_str()));

	RegSetValueEx(hKey, TEXT("AutoLoginUser"), 0, REG_SZ, (LPBYTE)accname, (accountName[0].size() + 1) * sizeof(wchar_t));
	RegCloseKey(hKey);

	RegOpenKeyEx(HKEY_CURRENT_USER, HKCUValve, 0, KEY_SET_VALUE, &hKey);
	DWORD val = 1;
	RegSetValueEx(hKey, TEXT("RememberPassword"), 0, REG_DWORD, (const BYTE*)&val, sizeof(val));
	RegCloseKey(hKey);
}
VOID startSteamAdmin(LPCTSTR lpApplicationName)
{
	SHELLEXECUTEINFO shExecInfo;

	shExecInfo.cbSize = sizeof(SHELLEXECUTEINFO);

	shExecInfo.fMask = NULL;
	shExecInfo.hwnd = NULL;
	shExecInfo.lpVerb = L"runas";
	shExecInfo.lpFile = lpApplicationName;
	shExecInfo.lpParameters = NULL;
	shExecInfo.lpDirectory = NULL;
	shExecInfo.nShow = SW_MAXIMIZE;
	shExecInfo.hInstApp = NULL;

	ShellExecuteEx(&shExecInfo);
}

inline bool checkFileExist(const std::string& name) {
	ifstream f(name.c_str());
	return f.good();
}

void addTrailingBackslash(string &str){
	if (str[str.size()] != '\\') { // Add final backslash if user missed it.
		str += '\\';
	}
}

void setSteamFolder() { // Set to x64 or x32 Steam install
	/*
		#include <direct.h>
		#define GetCurrentDir _getcwd
		// Get current working directory
		char cCurrentPath[FILENAME_MAX];
		GetCurrentDir(cCurrentPath, sizeof(cCurrentPath));
		cCurrentPath[sizeof(cCurrentPath) - 1] = '\0';
		printf("The current working directory is %s", cCurrentPath);
	*/
	SteamFolder = "C:\\Program Files (x86)\\Steam\\";
	//string SteamFolder = "C:\\Program Files (x86)\\Steam\\",
	//	LoginUsersVDF = SteamFolder + "config\\loginusers.vdf",
	//	SteamEXE = SteamFolder + "Steam.exe";
			//std::getline(fLoginUsers, curline)
	if (checkFileExist("SteamLocation.txt")) {
		std::ifstream steamloc("SteamLocation.txt");
		getline(steamloc, SteamFolder);
		addTrailingBackslash(SteamFolder);

		LoginUsersVDF = SteamFolder + "config\\loginusers.vdf";
		SteamEXE = SteamFolder + "Steam.exe";
		cout << "Steam location set to: " << SteamFolder << endl;
		if (!checkFileExist(SteamFolder + "Steam.exe")) {
			cout << "Steam.exe was not found!" << endl << endl <<
				"Press any key to exit..." << endl;
			cin.get();
			exit(EXIT_FAILURE);
		}
	}
	else {
		if (!checkFileExist(SteamFolder + "Steam.exe")) { // x64 does not exist.
			if (checkFileExist("C:\\Program Files\\Steam\\Steam.exe")) { //x32 does exist.
				SteamFolder = "C:\\Program Files\\Steam\\";
				LoginUsersVDF = SteamFolder + "config\\loginusers.vdf";
				SteamEXE = SteamFolder + "Steam.exe";
			}
			else {
				cout << "Steam was not found!" << endl;
				cout << "Enter Steam's location as such: \"C:\\Program Files\\Steam\\\"" << endl;

				bool SteamFound = false;
				while (!SteamFound)
				{
					cout << "Location: ";
					cin >> SteamFolder;
					cout << endl;
					addTrailingBackslash(SteamFolder);

					LoginUsersVDF = SteamFolder + "config\\loginusers.vdf";
					SteamEXE = SteamFolder + "Steam.exe";

					if (checkFileExist(SteamEXE)) {
						SteamFound = true;
						std::ofstream steamloc("SteamLocation.txt", ios::trunc);
						steamloc << SteamFolder;
						steamloc.close();

					}
					else {
						cout << "Steam.exe was not found at: " << SteamEXE << endl;
					}
				}
			}
		}
	}
}

inline bool getRunAsSetting() {
	return (checkFileExist("admin") || checkFileExist("admin.txt")); // Returns true if either file exists.
}

int main()
{
	SetConsoleTitleW(L"TcNo Steam Account Switcher");
	hConsole = GetStdHandle(STD_OUTPUT_HANDLE);
	// Set .exe directory as running directory, so saves and looks for files in the right place.
	HMODULE hModule = GetModuleHandleW(NULL);
	WCHAR path[MAX_PATH];
	GetModuleFileNameW(hModule, path, MAX_PATH);
	PathRemoveFileSpec(path);
	SetCurrentDirectory(path);

	setSteamFolder();
	bool runasAdmin = getRunAsSetting();

	// Push back other accounts here
	getSteamAccounts();
	cout << "Collected user accounts" << endl;
	// "Print accoutns on each line*"
	int selectedLine = 0;
	printSteamAccs(selectedLine);
	firstprint = false;

	// Adapted from: https://stackoverflow.com/questions/51410048/c-multiple-choice-with-arrow-keys
	bool selecting = true;
	bool updated = false;
	char c;
	while (selecting) {
		switch ((c = _getch())) {
		case KEY_UP:
			if (selectedLine > 0) {
				--selectedLine;
			}
			else // Wrap around
			{
				selectedLine = userAccounts.size() - 1;
			}
			updated = true;
			break;
		case KEY_DOWN:
			if (selectedLine < userAccounts.size() - 1) {
				++selectedLine;
			}
			else // Wrap around
			{
				selectedLine = 0;
			}
			updated = true;
			break;
		case KEY_ENTER:
			selecting = false;
			break;
		default: break;
		}
		if (updated) {
			printSteamAccs(selectedLine);
			updated = false;
		}
	}
	vector<string> account;
	if (selectedLine == userAccounts.size() - 1) // User selected to make a new account. 
	{
		account = { "", "", "", "" };
		cout << endl << "You will be asked for new login when steam starts." << endl;
	}
	else {
		account = userAccounts[selectedLine];
		cout << endl << "Changing user accounts to: " << account[0] << endl;
	}

	system("TASKKILL /F /T /IM steam*"); // Admin required to also kill Steam Service.
	// Work on starting process without admin for the future.

	mostRecentUpdate(account);

	LPWSTR steam = new wchar_t[SteamEXE.size() + 1];
	copy(SteamEXE.begin(), SteamEXE.end(), steam);
	steam[SteamEXE.size()] = 0;

	cout << endl << "Starting Steam..." << endl;

	if (runasAdmin) 
		startSteamAdmin(steam);
	else 
		startSteamNoAdmin(steam);


	// Closing in 3 seconds
	cout << "Done!" << endl << "Closing in 3 seconds" << endl;
	std::this_thread::sleep_for((chrono::seconds)3);

	return 0;
}


//void showColourList() {
//	HANDLE  hConsole;
//	int k;
//
//	hConsole = GetStdHandle(STD_OUTPUT_HANDLE);
//	for (k = 1; k < 255; k++)
//	{
//		SetConsoleTextAttribute(hConsole, k);
//		cout << k << " I want to be nice today!" << endl;
//	}
//	cin.get();
//}
// Run program: Ctrl + F5 or Debug > Start Without Debugging menu
// Debug program: F5 or Debug > Start Debugging menu
