// TCNO-Acc-Switcher.cpp : This file contains the 'main' function. Program execution begins and ends there.
//
#include <iostream>
#include <windows.h>   // WinApi header
#include <string>
#include <vector>
#include <cctype>
#include <sstream>
#include <fstream>
#include <algorithm>    // For std::remove()
#include <conio.h>
//#include <atlstr.h"> // Install ATL Libs for this


#define KEY_UP 72       //Up arrow character
#define KEY_DOWN 80     //Down arrow character
#define KEY_ENTER '\r'  //Enter key charatcer

using namespace std;    // std::cout, std::cin


vector<vector<string>> userAccounts; // Steam64ID, Username, Remember password <0,1>
HANDLE  hConsole;
string LoginUsersVDF = "C:\\Program Files (x86)\\Steam\\config\\loginusers.vdf";

bool getSteamAccounts() 
{
	std::ifstream fLoginUsers(LoginUsersVDF); // SOMEHOW READ AS UNICODE
	string curline, lineNoQuot;
	string username, steamID, rememberAccount, personaName;

	while (std::getline(fLoginUsers, curline)) {
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
	system("CLS");
	cout << flush;

	cout << "Welcome to TCNO Steam Account Switcher." << endl;
	cout << "How to use:" << endl <<
		"1. DO NOT change users or log out via Steam." << endl <<
		"2. Check \"Remember password\" if asked for one." << endl <<
		"3. To sign in with a new account, select the last option." << endl << 
		"4. You can start steam as normal, just use this to change accounts." << endl <<
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
	std::ifstream fLoginUsers(LoginUsersVDF); // Change to input output stream -- No internet.
	string curline, lineNoQuot;
	bool userIDMatch = false;

	while (std::getline(fLoginUsers, curline)) {


		break; // JUST FOR DEBUGGING


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
			cout << curline << endl; // DEBUG ONLY
		}
		else if (curline.find("mostrecent") != std::string::npos)
		{
			// Set ever=y mostrecent to 0, unless it's the one you want to switch to.
			//REPLACE LINE WITH:
			string toreplace;
			if (userIDMatch) {
				toreplace = "\t\t\"mostrecent\"\t\t\"1\"";
			}else{
				toreplace = "\t\t\"mostrecent\"\t\t\"0\"";
			}
			cout << toreplace << endl; // DEBUG ONLY
		}
		else { cout << curline << endl; } // DEBUG ONLY
	}

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

	/*DWORD keyAutoLoginUserType = REG_SZ;
	DWORD keyRememberPasswordType = REG_DWORD;*/

	//RegOpenKeyEx(HKEY_CURRENT_USER, HKCUValve, 0, KEY_SET_VALUE, &hKey);
	//TCHAR
	//RegSetValueEx(hKey, TEXT("AutoLoginUser"), 0, REG_SZ, (LPBYTE), sizeof()));

	//// READ VARIABLE
	//char buf[255] = { 0 };
	//DWORD bufSize = sizeof(buf);
	//RegOpenKeyEx(HKEY_CURRENT_USER, HKCUValve, 0, KEY_QUERY_VALUE, &hKey);
	//auto ret = RegQueryValueEx(hKey, TEXT("AutoLoginUser"), 0, &keyAutoLoginUserType, (LPBYTE)buf, &bufSize);
	//RegCloseKey(hKey);
	//string out= "";
	//for (int i = 0; i < bufSize; i++)
	//{
	//	out += buf[i];
	//}
	//cout << out;

}

int main()
{
	hConsole = GetStdHandle(STD_OUTPUT_HANDLE);

	static const string aoptions[] = { "1. Add currently logged in Steam account" };
	vector<string> options(aoptions, aoptions + sizeof(aoptions) / sizeof(aoptions[0]));

	// Push back other accounts here
	getSteamAccounts();
	cout << "Collected user accounts" << endl;
	// "Print accoutns on each line*"
	int selectedLine = 0;
	printSteamAccs(selectedLine);

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
	mostRecentUpdate(account);


	cin.get();
	return 0;
}


void showColourList() {
	HANDLE  hConsole;
	int k;

	hConsole = GetStdHandle(STD_OUTPUT_HANDLE);
	for (k = 1; k < 255; k++)
	{
		SetConsoleTextAttribute(hConsole, k);
		cout << k << " I want to be nice today!" << endl;
	}
	cin.get();
}
// Run program: Ctrl + F5 or Debug > Start Without Debugging menu
// Debug program: F5 or Debug > Start Debugging menu

// Tips for Getting Started: 
//   1. Use the Solution Explorer window to add/manage files
//   2. Use the Team Explorer window to connect to source control
//   3. Use the Output window to see build output and other messages
//   4. Use the Error List window to view errors
//   5. Go to Project > Add New Item to create new code files, or Project > Add Existing Item to add existing code files to the project
//   6. In the future, to open this project again, go to File > Open > Project and select the .sln file
