// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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

// Goal of this project:
// To allow users to start the program regardless of .NET version.
// If .NET is missing > Open installer, then after install relaunch.

#pragma once
#ifndef _PROGRESS_BAR_
#define _PROGRESS_BAR_
#include <Windows.h>
#include <iostream>
#include <iomanip>
#define TOTAL_PERCENTAGE 100.0
#define CHARACTER_WIDTH_PERCENTAGE 4
class ProgressBar {
public:
	ProgressBar();
	ProgressBar(unsigned long n_, const char* description_ = "", std::ostream& out_ = std::cerr);
	void SetFrequencyUpdate(unsigned long frequency_update_);
	void SetStyle(const char* unit_bar_, const char* unit_space_);
	void Progressed(unsigned long idx_);
private:
	unsigned long n{};
	unsigned int desc_width{};
	unsigned long frequency_update{};
	std::ostream* out{};
	const char* description{};
	const char* unit_bar{};
	const char* unit_space{};
	void ClearBarField() const;
	static int GetConsoleWidth();
	int GetBarLength() const;
};
#endif