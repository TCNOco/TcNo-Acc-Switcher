// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
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

use std::fs::{File, OpenOptions};
use std::io::{self, Write};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use once_cell::sync::Lazy;

static LOG_WRITER: Lazy<Arc<Mutex<Option<File>>>> = Lazy::new(|| {
    Arc::new(Mutex::new(None))
});

pub struct Logger;

impl Logger {
    pub fn init() -> Result<(), io::Error> {
        let log_path = Self::get_log_path()?;
        
        if let Some(parent) = log_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        // Clear existing log file
        if log_path.exists() {
            std::fs::write(&log_path, "")?;
        }
        
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_path)?;
        
        *LOG_WRITER.lock().unwrap() = Some(file);
        Ok(())
    }
    
    fn get_log_path() -> Result<PathBuf, io::Error> {
        // Get app data folder
        let exe_path = std::env::current_exe()?;
        let exe_dir = exe_path.parent()
            .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get exe directory"))?;
        let app_data_folder = exe_dir.parent()
            .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get parent directory"))?;
        
        // Get user data folder
        let userdata_path_file = app_data_folder.join("userdata_path.txt");
        let user_data_folder = if userdata_path_file.exists() {
            if let Ok(lines) = std::fs::read_to_string(&userdata_path_file) {
                if let Some(first_line) = lines.lines().next() {
                    PathBuf::from(first_line.trim())
                } else {
                    dirs::data_dir()
                        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get AppData directory"))?
                        .join("TcNo Account Switcher")
                }
            } else {
                dirs::data_dir()
                    .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get AppData directory"))?
                    .join("TcNo Account Switcher")
            }
        } else {
            dirs::data_dir()
                .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get AppData directory"))?
                .join("TcNo Account Switcher")
        };
        
        Ok(user_data_folder.join("UpdaterLog.txt"))
    }
    
    pub fn write_line(line: &str) {
        println!("{}", line);
        
        if let Ok(mut writer_guard) = LOG_WRITER.lock() {
            if let Some(ref mut file) = *writer_guard {
                let _ = writeln!(file, "{}", line);
                let _ = file.flush();
            }
        }
    }
    
    #[allow(dead_code)] // Public API method for error logging
    pub fn log_to_error_file(log: &str) -> Result<(), io::Error> {
        let log_path = Self::get_log_path()?;
        let user_data_folder = log_path.parent()
            .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get log directory"))?;
        let error_logs_dir = user_data_folder.join("UpdaterErrorLogs");
        
        std::fs::create_dir_all(&error_logs_dir)?;
        
        let timestamp = chrono::Local::now().format("%d-%m-%y_%H-%M-%S%.3f");
        let file_path = error_logs_dir.join(format!("AccSwitcher-Updater-{}.txt", timestamp));
        
        let version = crate::utils::get_version().unwrap_or_else(|_| "unknown".to_string());
        let log_entry = format!(
            "{}({})\n{}\n",
            chrono::Local::now().format("%Y-%m-%d %H:%M:%S%.3f"),
            version,
            log
        );
        
        let log_entry_clone = log_entry.clone();
        std::fs::write(&file_path, log_entry)?;
        Self::write_line(&format!("\nUpdater encountered an error!\n: {}", log_entry_clone));
        
        Ok(())
    }
}

