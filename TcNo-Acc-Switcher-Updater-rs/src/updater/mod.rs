// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

use std::collections::HashMap;
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::process::Command;
use crate::archive;
use crate::patch;
use crate::utils;
use crate::windows;
use serde_json::Value;

const MINIMUM_VC: &str = "14.30.30704";
const API_BASE: &str = "https://tcno.co/Projects/AccSwitcher/api";

pub struct Updater {
    current_version: String,
    latest_version: String,
    main_browser: String,
    updater_directory: PathBuf,
}

impl Updater {
    pub fn new() -> Result<Self, Box<dyn std::error::Error>> {
        let updater_dir = std::env::current_exe()?
            .parent()
            .ok_or("Cannot get exe directory")?
            .to_path_buf();
        
        let current_version = Self::get_version_from_globals()
            .unwrap_or_else(|_| utils::get_version().unwrap_or_default());
        
        let window_settings = utils::get_user_data_folder(&utils::get_app_data_folder()?)?
            .join("WindowSettings.json");
        
        let main_browser = if window_settings.exists() {
            if let Ok(content) = utils::read_all_text(&window_settings) {
                if let Ok(json) = serde_json::from_str::<Value>(&content) {
                    json.get("ActiveBrowser")
                        .and_then(|v| v.as_str())
                        .unwrap_or("WebView")
                        .to_string()
                } else {
                    "WebView".to_string()
                }
            } else {
                "WebView".to_string()
            }
        } else {
            "WebView".to_string()
        };
        
        Ok(Self {
            current_version,
            latest_version: String::new(),
            main_browser,
            updater_directory: updater_dir,
        })
    }
    
    fn get_version_from_globals() -> Result<String, Box<dyn std::error::Error>> {
        let dll_path = std::env::current_dir()?.join("TcNo-Acc-Switcher-Globals.dll");
        
        if !dll_path.exists() {
            return Err("Globals.dll not found".into());
        }
        
        // Note: Reading version from .NET DLL in Rust is complex
        // This would require parsing the DLL or using .NET interop
        // For now, we'll return an error and fall back to WindowSettings.json
        Err("Cannot read version from DLL (requires .NET interop)".into())
    }
    
    pub fn get_updates_list(&mut self) -> Result<HashMap<String, String>, Box<dyn std::error::Error>> {
        let client = reqwest::blocking::Client::new();
        
        #[cfg(debug_assertions)]
        let update_url = format!("{}/update?debug&v={}", API_BASE, self.current_version);
        #[cfg(not(debug_assertions))]
        let update_url = format!("{}/update?v={}", API_BASE, self.current_version);
        
        let versions = client.get(&update_url).send()?.text()?;
        
        #[cfg(debug_assertions)]
        let latest_url = format!("{}?debug&v={}", API_BASE, self.current_version);
        #[cfg(not(debug_assertions))]
        let latest_url = format!("{}?v={}", API_BASE, self.current_version);
        
        self.latest_version = client.get(&latest_url).send()?.text()?;
        
        let json: Value = serde_json::from_str(&versions)?;
        let updates = json.get("updates")
            .and_then(|v| v.as_object())
            .ok_or("Invalid update JSON")?;
        
        let mut updates_map = HashMap::new();
        let mut first_checked = false;
        
        for (version, changes) in updates {
            if Self::check_latest(&self.current_version, version) {
                break;
            }
            
            if !first_checked {
                first_checked = true;
                self.latest_version = version.clone();
                if Self::check_latest(&self.latest_version, &self.current_version) {
                    break;
                }
            }
            
            updates_map.insert(version.clone(), changes.to_string());
        }
        
        Ok(updates_map)
    }
    
    fn check_latest(current: &str, latest: &str) -> bool {
        // Parse dates in format "yyyy-MM-dd_mm"
        if let (Ok(current_date), Ok(latest_date)) = (
            chrono::NaiveDateTime::parse_from_str(&format!("{}:00", current), "%Y-%m-%d_%H:%M:%S"),
            chrono::NaiveDateTime::parse_from_str(&format!("{}:00", latest), "%Y-%m-%d_%H:%M:%S"),
        ) {
            return current_date >= latest_date;
        }
        false
    }
    
    pub fn download_file_with_progress<F>(
        &self,
        url: &str,
        destination: &Path,
        mut progress_callback: F,
    ) -> Result<(), Box<dyn std::error::Error>>
    where
        F: FnMut(u64, u64),
    {
        let client = reqwest::blocking::Client::new();
        let mut response = client.get(url).send()?;
        
        let total_size = response.content_length().unwrap_or(0);
        let mut file = std::fs::File::create(destination)?;
        let mut downloaded = 0u64;
        let mut buffer = vec![0u8; 8192];
        
        loop {
            let bytes_read = response.read(&mut buffer)?;
            if bytes_read == 0 {
                break;
            }
            file.write_all(&buffer[..bytes_read])?;
            downloaded += bytes_read as u64;
            progress_callback(downloaded, total_size);
        }
        
        Ok(())
    }
    
    pub fn do_update<F1, F2, F3>(
        &mut self,
        cef: bool,
        status_callback: F1,
        log_callback: F2,
        progress_callback: F3,
    ) -> Result<(), Box<dyn std::error::Error>>
    where
        F1: Fn(&str),
        F2: Fn(&str),
        F3: Fn(u64, u64),
    {
        status_callback("Closing TcNo Account Switcher instances (if any).");
        log_callback("Exiting all .exe's in application folder");
        
        let app_data = utils::get_app_data_folder()?;
        self.close_if_running(&app_data)?;
        
        // Move wwwroot to originalwwwroot if exists
        let wwwroot = app_data.join("wwwroot");
        let original_wwwroot = app_data.join("originalwwwroot");
        if wwwroot.exists() {
            if original_wwwroot.exists() {
                utils::recursive_delete(&original_wwwroot, false)?;
            }
            std::fs::rename(&wwwroot, &original_wwwroot)?;
        }
        
        // Cleanup previous updates
        let temp_update = app_data.join("temp_update");
        utils::recursive_delete(&temp_update, false)?;
        
        // Download update
        let download_url = if cef {
            format!(
                "https://github.com/TCNOco/TcNo-Acc-Switcher/releases/download/{}/TcNo-Acc-Switcher_and_CEF_{}.7z",
                self.latest_version, self.latest_version
            )
        } else {
            format!(
                "https://github.com/TCNOco/TcNo-Acc-Switcher/releases/download/{}/TcNo-Acc-Switcher_{}.7z",
                self.latest_version, self.latest_version
            )
        };
        
        log_callback(&format!("Downloading from: {}", download_url));
        let update_file = self.updater_directory.join(format!("{}.7z", self.latest_version));
        
        if update_file.exists() {
            std::fs::remove_file(&update_file)?;
        }
        
        status_callback(&format!("Downloading update: {} (~{}MB)", 
            self.latest_version, 
            if cef { "110" } else { "40" }));
        
        self.download_file_with_progress(&download_url, &update_file, |downloaded, total| {
            progress_callback(downloaded, total);
        })?;
        
        status_callback("Download complete.");
        log_callback("Download complete.");
        
        // Extract
        std::fs::create_dir_all(&temp_update)?;
        status_callback("Extracting patch files");
        log_callback("Extracting patch files");
        archive::extract_archive(&update_file, &temp_update)?;
        
        // Delete downloaded file
        std::fs::remove_file(&update_file)?;
        
        status_callback("Applying update. Window will close...");
        log_callback("Applying update. Window will close...");
        
        // Copy _First_Run_Installer.exe to temp folder
        let temp_folder = std::env::temp_dir().join("TcNo-Acc-Switcher");
        std::fs::create_dir_all(&temp_folder)?;
        let temp_installer = temp_folder.join("_First_Run_Installer.exe");
        let installer_source = app_data.join("updater").join("_First_Run_Installer.exe");
        
        if installer_source.exists() {
            std::fs::copy(&installer_source, &temp_installer)?;
        }
        
        // Run finalize process
        Command::new(&temp_installer)
            .arg("finalizeupdate")
            .arg(&temp_update)
            .arg(&app_data)
            .spawn()?;
        
        Ok(())
    }
    
    pub fn verify_files<F1, F2, F3>(
        &mut self,
        status_callback: F1,
        log_callback: F2,
        progress_callback: F3,
    ) -> Result<(), Box<dyn std::error::Error>>
    where
        F1: Fn(&str),
        F2: Fn(&str),
        F3: Fn(u64, u64),
    {
        let client = reqwest::blocking::Client::new();
        
        #[cfg(debug_assertions)]
        let url = format!("{}?debug&v={}", API_BASE, self.current_version);
        #[cfg(not(debug_assertions))]
        let url = format!("{}?v={}", API_BASE, self.current_version);
        
        self.latest_version = client.get(&url).send()?.text()?;
        
        let using_cef = self.using_cef()?;
        self.do_update(using_cef, status_callback, log_callback, progress_callback)
    }
    
    fn using_cef(&self) -> Result<bool, Box<dyn std::error::Error>> {
        let app_data = utils::get_app_data_folder()?;
        let user_data = utils::get_user_data_folder(&app_data)?;
        let window_settings = user_data.join("WindowSettings.json");
        
        if window_settings.exists() {
            let content = utils::read_all_text(&window_settings)?;
            let json: Value = serde_json::from_str(&content)?;
            
            if let Some(browser) = json.get("ActiveBrowser").and_then(|v| v.as_str()) {
                return Ok(browser.to_lowercase() != "webview");
            }
        }
        
        Ok(true) // Default to CEF
    }
    
    pub fn close_if_running(&self, folder: &Path) -> Result<(), Box<dyn std::error::Error>> {
        let exe_name = std::env::current_exe()?
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("")
            .to_string();
        
        for entry in walkdir::WalkDir::new(folder) {
            let entry = entry?;
            let path = entry.path();
            
            if path.extension().and_then(|e| e.to_str()) == Some("exe") {
                if let Some(file_name) = path.file_name().and_then(|n| n.to_str()) {
                    if file_name == exe_name 
                        || file_name == "TcNo-Acc-Switcher-Updater.exe"
                        || file_name == "TcNo-Acc-Switcher-Updater_main.exe" {
                        continue;
                    }
                    
                    // Check if process is running
                    let process_name = file_name.trim_end_matches(".exe");
                    if let Err(e) = windows::kill_process(process_name) {
                        log::warn!("Failed to kill process {}: {}", process_name, e);
                    }
                }
            }
        }
        
        Ok(())
    }
    
    pub fn generate_hashes(&self) -> Result<(), Box<dyn std::error::Error>> {
        let new_folder = std::path::Path::new("TcNo-Acc-Switcher");
        let _new_dict: HashMap<String, String> = std::collections::HashMap::new();
        
        // Use patch module's dir_search_with_hash logic
        patch::create_folder_patches(new_folder, new_folder, std::path::Path::new("."), true)?;
        
        // The hashes.json is already created by create_folder_patches
        Ok(())
    }
    
    pub fn download_cef_now<F1, F2, F3>(
        &mut self,
        status_callback: F1,
        log_callback: F2,
        progress_callback: F3,
    ) -> Result<(), Box<dyn std::error::Error>>
    where
        F1: Fn(&str),
        F2: Fn(&str),
        F3: Fn(u64, u64),
    {
        // Check VC++ Runtime
        if !windows::is_vc_runtime_installed(MINIMUM_VC) {
            status_callback("Downloading and installing Visual C++ Runtime 2015-2022");
            log_callback("Downloading and installing Visual C++ Runtime 2015-2022");
            
            let installer = utils::get_app_data_folder()?.join("_First_Run_Installer.exe");
            if installer.exists() {
                Command::new(&installer).arg("vc").spawn()?.wait()?;
            }
        }
        
        status_callback("Preparing to install Chrome Embedded Framework");
        status_callback("Checking latest version number");
        
        let client = reqwest::blocking::Client::new();
        #[cfg(debug_assertions)]
        let url = format!("{}?debug&v={}", API_BASE, self.current_version);
        #[cfg(not(debug_assertions))]
        let url = format!("{}?v={}", API_BASE, self.current_version);
        
        self.latest_version = client.get(&url).send()?.text()?;
        self.do_update(true, status_callback, log_callback, progress_callback)
    }
    
    pub fn create_update(&self) -> Result<(), Box<dyn std::error::Error>> {
        let old_folder = std::path::Path::new("OldVersion");
        let new_folder = std::path::Path::new("TcNo-Acc-Switcher");
        let output_folder = std::path::Path::new("UpdateOutput");
        
        std::fs::create_dir_all(output_folder)?;
        
        let delete_file_list = output_folder.join("filesToDelete.txt");
        utils::recursive_delete(output_folder, false)?;
        utils::delete_file(&delete_file_list);
        
        let files_to_delete = patch::create_folder_patches(
            old_folder,
            new_folder,
            output_folder,
            false,
        )?;
        
        std::fs::write(&delete_file_list, files_to_delete.join("\n"))?;
        
        Ok(())
    }
    
    pub fn current_version(&self) -> &str {
        &self.current_version
    }
    
    pub fn latest_version(&self) -> &str {
        &self.latest_version
    }
}


