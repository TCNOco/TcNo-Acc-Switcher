#include "progress_bar.hpp"
ProgressBar::ProgressBar() = default;

ProgressBar::ProgressBar(unsigned long n_, const char* description_, std::ostream& out_) {
	n = n_;
	frequency_update = n_;
	description = description_;
	out = &out_;
	unit_bar = "=";
	unit_space = " ";
	desc_width = std::strlen(description);	// character width of description field
}
void ProgressBar::SetFrequencyUpdate(unsigned long frequency_update_) {
	if (frequency_update_ > n) {
		frequency_update = n;	 // prevents crash if freq_updates_ > n_
	}
	else {
		frequency_update = frequency_update_;
	}
}
void ProgressBar::SetStyle(const char* unit_bar_, const char* unit_space_) {
	unit_bar = unit_bar_;
	unit_space = unit_space_;
}
int ProgressBar::GetConsoleWidth() {
	CONSOLE_SCREEN_BUFFER_INFO buffer_info;
	GetConsoleScreenBufferInfo(GetStdHandle(STD_OUTPUT_HANDLE), &buffer_info);
	const int width = buffer_info.srWindow.Right - buffer_info.srWindow.Left;
	return width;
}
int ProgressBar::GetBarLength() const
{
	// get console width and according adjust the length of the progress bar
	const int bar_length = static_cast<int>((static_cast<double>(GetConsoleWidth()) - desc_width - CHARACTER_WIDTH_PERCENTAGE) / 2.);
	return bar_length;
}
void ProgressBar::ClearBarField() const
{
	for (int i = 0; i < GetConsoleWidth(); ++i) {
		*out << " ";
	}
	*out << "\r" << std::flush;
}
void ProgressBar::Progressed(unsigned long idx_)
{
	try {
		if (idx_ > n) throw idx_;
		// determines whether to update the progress bar from frequency_update
		if ((idx_ != n) && (idx_ % (n / frequency_update) != 0)) return;
		// calculate the size of the progress bar
		const int bar_size = GetBarLength();
		// calculate percentage of progress
		const double progress_percent = idx_ * TOTAL_PERCENTAGE / n;
		// calculate the percentage value of a unit bar
		const double percent_per_unit_bar = TOTAL_PERCENTAGE / bar_size;
		// display progress bar
		*out << " " << description << " [";
		for (int bar_length = 0; bar_length <= bar_size - 1; ++bar_length) {
			if (bar_length * percent_per_unit_bar < progress_percent) {
				*out << unit_bar;
			}
			else {
				*out << unit_space;
			}
		}
		*out << "]" << std::setw(CHARACTER_WIDTH_PERCENTAGE + 1) << std::setprecision(1) << std::fixed << progress_percent << "%\r" << std::flush;
	}
	catch (unsigned long e) {
		ClearBarField();
		std::cerr << "PROGRESS_BAR_EXCEPTION: _idx (" << e << ") went out of bounds, greater than n (" << n << ")." << std::endl << std::flush;
	}
}