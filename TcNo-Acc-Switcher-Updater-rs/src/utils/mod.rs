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

use std::fs;
use std::io::{self, Read};
use std::path::{Path, PathBuf};
#[cfg(windows)]
use std::os::windows::fs::OpenOptionsExt;
use serde_json::Value;

pub fn read_all_text<P: AsRef<Path>>(path: P) -> Result<String, io::Error> {
    #[cfg(windows)]
    {
        let mut file = fs::OpenOptions::new()
            .read(true)
            .share_mode(0x00000001) // FILE_SHARE_READ
            .open(path)?;
        
        let mut contents = String::new();
        file.read_to_string(&mut contents)?;
        Ok(contents)
    }
    #[cfg(not(windows))]
    {
        fs::read_to_string(path)
    }
}

pub fn read_all_lines<P: AsRef<Path>>(path: P) -> Result<Vec<String>, io::Error> {
    let contents = read_all_text(path)?;
    Ok(contents.lines().map(|s| s.to_string()).collect())
}

pub fn get_app_data_folder() -> Result<PathBuf, io::Error> {
    let exe_path = std::env::current_exe()?;
    let exe_dir = exe_path.parent()
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get exe directory"))?;
    
    Ok(exe_dir.parent()
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get parent directory"))?
        .to_path_buf())
}

pub fn get_user_data_folder(app_data: &PathBuf) -> Result<PathBuf, io::Error> {
    let userdata_path_file = app_data.join("userdata_path.txt");
    
    if userdata_path_file.exists() {
        let lines = read_all_lines(&userdata_path_file)?;
        if let Some(first_line) = lines.first() {
            return Ok(PathBuf::from(first_line.trim()));
        }
    }
    
    let app_data_path = dirs::data_dir()
        .ok_or_else(|| io::Error::new(io::ErrorKind::NotFound, "Cannot get AppData directory"))?;
    
    Ok(app_data_path.join("TcNo Account Switcher"))
}

#[allow(dead_code)] // Public utility function for 1-to-1 parity
pub fn get_main_app_data_folder() -> Result<PathBuf, io::Error> {
    get_app_data_folder()
}

pub fn get_version() -> Result<String, io::Error> {
    let window_settings = get_user_data_folder(&get_app_data_folder()?)?
        .join("WindowSettings.json");
    
    if window_settings.exists() {
        let content = read_all_text(&window_settings)?;
        let json: Value = serde_json::from_str(&content)?;
        
        if let Some(version) = json.get("Version").and_then(|v| v.as_str()) {
            return Ok(version.to_string());
        }
    }
    
    Err(io::Error::new(io::ErrorKind::NotFound, "Version not found"))
}

pub fn get_file_md5<P: AsRef<Path>>(file_path: P) -> Result<String, io::Error> {
    let mut file = fs::File::open(file_path)?;
    let metadata = file.metadata()?;
    
    if metadata.len() == 0 {
        return Ok("0".to_string());
    }
    
    let mut hasher = md5::Context::new();
    let mut buffer = vec![0u8; 8192];
    
    loop {
        let bytes_read = file.read(&mut buffer)?;
        if bytes_read == 0 {
            break;
        }
        hasher.consume(&buffer[..bytes_read]);
    }
    
    let hash = hasher.compute();
    Ok(format!("{:x}", hash))
}

pub fn compare_versions(v1: &str, v2: &str, delimiter: &str) -> bool {
    if v1 == v2 {
        return true;
    }
    
    let v1_parts: Vec<i32> = v1.split(delimiter)
        .filter_map(|s| s.parse().ok())
        .collect();
    
    let v2_parts: Vec<i32> = v2.split(delimiter)
        .filter_map(|s| s.parse().ok())
        .collect();
    
    let min_len = v1_parts.len().min(v2_parts.len());
    
    for i in 0..min_len {
        if v2_parts[i] < v1_parts[i] {
            return false;
        }
        if v2_parts[i] > v1_parts[i] {
            return true;
        }
    }
    
    true
}

pub fn copy_files_recursive<P: AsRef<Path>, Q: AsRef<Path>>(
    input_folder: P,
    output_folder: Q,
) -> Result<(), io::Error> {
    let input = input_folder.as_ref();
    let output = output_folder.as_ref();
    
    if !input.exists() {
        return Err(io::Error::new(
            io::ErrorKind::NotFound,
            format!("Input folder does not exist: {:?}", input),
        ));
    }
    
    fs::create_dir_all(output)?;
    
    for entry in walkdir::WalkDir::new(input) {
        let entry = entry?;
        let path = entry.path();
        let relative = path.strip_prefix(input)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidInput, e))?;
        let dest = output.join(relative);
        
        if path.is_dir() {
            fs::create_dir_all(&dest)?;
        } else {
            if let Some(parent) = dest.parent() {
                fs::create_dir_all(parent)?;
            }
            fs::copy(path, &dest)?;
        }
    }
    
    Ok(())
}

#[allow(dead_code)] // Used by updater::apply_update
pub fn move_files_recursive<P: AsRef<Path>, Q: AsRef<Path>>(
    input_folder: P,
    output_folder: Q,
) -> Result<(), io::Error> {
    let input = input_folder.as_ref();
    let output = output_folder.as_ref();
    
    for entry in walkdir::WalkDir::new(input) {
        let entry = entry?;
        let path = entry.path();
        let relative = path.strip_prefix(input)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidInput, e))?;
        let dest = output.join(relative);
        
        if path.is_dir() {
            fs::create_dir_all(&dest)?;
        } else {
            if let Some(parent) = dest.parent() {
                fs::create_dir_all(parent)?;
            }
            if dest.exists() {
                fs::remove_file(&dest)?;
            }
            fs::rename(path, &dest)?;
        }
    }
    
    Ok(())
}

pub fn recursive_delete<P: AsRef<Path>>(base_dir: P, keep_folders: bool) -> Result<(), io::Error> {
    let base = base_dir.as_ref();
    
    if !base.exists() {
        return Ok(());
    }
    
    for entry in walkdir::WalkDir::new(base).contents_first(true) {
        let entry = entry?;
        let path = entry.path();
        
        if path.is_file() {
            let _ = fs::remove_file(path);
        } else if path.is_dir() && !keep_folders {
            let _ = fs::remove_dir(path);
        }
    }
    
    if !keep_folders && base.is_dir() {
        let _ = fs::remove_dir(base);
    }
    
    Ok(())
}

pub fn delete_file<P: AsRef<Path>>(file: P) {
    let path = file.as_ref();
    if !path.exists() {
        return;
    }
    
    #[cfg(windows)]
    {
        if let Ok(metadata) = fs::metadata(path) {
            let mut perms = metadata.permissions();
            perms.set_readonly(false);
            let _ = fs::set_permissions(path, perms);
        }
    }
    
    let _ = fs::remove_file(path);
}

