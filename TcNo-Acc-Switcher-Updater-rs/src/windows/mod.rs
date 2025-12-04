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

#[cfg(windows)]
use std::process::Command;
#[cfg(windows)]
use windows::{
    core::*,
    Win32::Foundation::*,
    Win32::Security::*,
    Win32::System::Registry::*,
    Win32::System::Threading::*,
};

#[cfg(windows)]
pub fn is_admin() -> bool {
    unsafe {
        let mut token_handle = HANDLE::default();
        if OpenProcessToken(
            GetCurrentProcess(),
            TOKEN_QUERY,
            &mut token_handle,
        )
        .is_err()
        {
            return false;
        }
        
        let mut elevation = TOKEN_ELEVATION::default();
        let mut return_length = 0u32;
        
        let result = GetTokenInformation(
            token_handle,
            TokenElevation,
            Some(&mut elevation as *mut _ as *mut _),
            std::mem::size_of::<TOKEN_ELEVATION>() as u32,
            &mut return_length,
        );
        
        CloseHandle(token_handle).ok();
        
        result.is_ok() && elevation.TokenIsElevated != 0
    }
}

#[cfg(windows)]
pub fn has_folder_access<P: AsRef<std::path::Path>>(folder_path: P) -> bool {
    use std::fs;
    use std::io::Write;
    
    let path = folder_path.as_ref();
    let test_file = path.join(format!("test_{}.tmp", uuid::Uuid::new_v4()));
    
    match fs::File::create(&test_file) {
        Ok(mut file) => {
            let result = file.write_all(&[0u8]).is_ok();
            drop(file);
            let _ = fs::remove_file(&test_file);
            result
        }
        Err(_) => false,
    }
}

#[cfg(windows)]
pub fn installed_to_program_files() -> bool {
    let exe_path = match std::env::current_exe() {
        Ok(p) => p,
        Err(_) => return false,
    };
    
    let exe_dir = match exe_path.parent() {
        Some(d) => d,
        None => return false,
    };
    
    let exe_str = exe_dir.to_string_lossy().to_lowercase();
    let prog_files = std::env::var("ProgramFiles")
        .unwrap_or_default()
        .to_lowercase();
    let prog_files_x86 = std::env::var("ProgramFiles(x86)")
        .unwrap_or_default()
        .to_lowercase();
    
    exe_str.contains(&prog_files) || exe_str.contains(&prog_files_x86)
}

#[cfg(windows)]
pub fn restart_as_admin(args: &str) -> std::result::Result<(), Box<dyn std::error::Error>> {
    use std::os::windows::ffi::OsStrExt;
    
    let exe_path = std::env::current_exe()?;
    
    // Use ShellExecute with "runas" verb for elevation
    unsafe {
        use winapi::um::shellapi::ShellExecuteW;
        use winapi::um::winuser::SW_SHOWNORMAL;
        
        let exe_wide: Vec<u16> = exe_path.as_os_str().encode_wide().chain(std::iter::once(0)).collect();
        let args_wide: Vec<u16> = args.encode_utf16().chain(std::iter::once(0)).collect();
        let verb_wide: Vec<u16> = "runas\0".encode_utf16().collect();
        
        let result = ShellExecuteW(
            std::ptr::null_mut(),
            verb_wide.as_ptr(),
            exe_wide.as_ptr(),
            args_wide.as_ptr(),
            std::ptr::null(),
            SW_SHOWNORMAL,
        );
        
        if result as usize > 32 {
            std::process::exit(0);
        } else {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                format!("Failed to elevate process: {}", result as i32),
            ).into());
        }
    }
}

#[cfg(windows)]
pub fn is_vc_runtime_installed(minimum_version: &str) -> bool {
    unsafe {
        let key_path = "SOFTWARE\\Microsoft\\DevDiv\\VC\\Servicing\\14.0\\RuntimeMinimum";
        let key_name = HSTRING::from("Version");
        let mut key = HKEY::default();
        
        let result = RegOpenKeyExW(
            HKEY_LOCAL_MACHINE,
            &HSTRING::from(key_path),
            0,
            KEY_READ,
            &mut key,
        );
        
        if result.is_ok() {
            let mut value_type = REG_VALUE_TYPE::default();
            let mut data = [0u16; 256];
            let mut data_size = (data.len() * 2) as u32;
            
            let query_result = RegQueryValueExW(
                key,
                &key_name,
                None,
                Some(&mut value_type),
                Some(&mut data as *mut _ as *mut u8),
                Some(&mut data_size),
            );
            
            let _ = RegCloseKey(key).ok();
            
            if query_result.is_ok() {
                let version = String::from_utf16_lossy(&data[..data_size as usize / 2])
                    .trim_end_matches('\0')
                    .to_string();
                return crate::utils::compare_versions(minimum_version, &version, ".");
            }
        }
        
        false
    }
}

#[cfg(windows)]
pub fn kill_process(proc_name: &str) -> std::result::Result<(), Box<dyn std::error::Error>> {
    Command::new("cmd.exe")
        .args(&["/C", "TASKKILL", "/F", "/T", "/IM", &format!("{}*", proc_name)])
        .output()?;
    Ok(())
}

#[cfg(not(windows))]
pub fn is_admin() -> bool {
    false
}

#[cfg(not(windows))]
pub fn has_folder_access<P: AsRef<std::path::Path>>(_folder_path: P) -> bool {
    true
}

#[cfg(not(windows))]
pub fn installed_to_program_files() -> bool {
    false
}

#[cfg(not(windows))]
pub fn restart_as_admin(_args: &str) -> std::result::Result<(), Box<dyn std::error::Error>> {
    Err("Not supported on this platform".into())
}

#[cfg(not(windows))]
pub fn is_vc_runtime_installed(_minimum_version: &str) -> bool {
    false
}

#[cfg(not(windows))]
pub fn kill_process(_proc_name: &str) -> Result<(), Box<dyn std::error::Error>> {
    Err("Not supported on this platform".into())
}

#[cfg(windows)]
pub fn is_winget_available() -> bool {
    let output = Command::new("cmd.exe")
        .args(&["/C", "winget", "--version"])
        .output();
    
    match output {
        Ok(result) => result.status.success(),
        Err(_) => false,
    }
}

#[cfg(windows)]
pub fn is_choco_available() -> bool {
    let output = Command::new("cmd.exe")
        .args(&["/C", "choco", "--version"])
        .output();
    
    match output {
        Ok(result) => result.status.success(),
        Err(_) => false,
    }
}

#[cfg(windows)]
pub fn install_vcruntime_winget() -> std::result::Result<bool, Box<dyn std::error::Error>> {
    println!("Installing Visual C++ Runtime 2015-2022 via winget...");
    
    let output = Command::new("cmd.exe")
        .args(&["/C", "winget", "install", "Microsoft.VCRedist.2015+.x64", 
                "--accept-source-agreements", "--accept-package-agreements", "--silent"])
        .output()?;
    
    if output.status.success() {
        println!("Successfully installed Visual C++ Runtime via winget.");
        Ok(true)
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        println!("Failed to install via winget: {}", stderr);
        Ok(false)
    }
}

#[cfg(windows)]
pub fn install_vcruntime_choco() -> std::result::Result<bool, Box<dyn std::error::Error>> {
    println!("Installing Visual C++ Runtime 2015-2022 via Chocolatey...");
    
    let output = Command::new("cmd.exe")
        .args(&["/C", "choco", "install", "vcredist-all", "-y"])
        .output()?;
    
    if output.status.success() {
        println!("Successfully installed Visual C++ Runtime via Chocolatey.");
        Ok(true)
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        println!("Failed to install via Chocolatey: {}", stderr);
        Ok(false)
    }
}

#[cfg(windows)]
pub fn download_and_install_vcruntime() -> std::result::Result<bool, Box<dyn std::error::Error>> {
    use std::fs::File;
    
    println!("Downloading Visual C++ Runtime installer...");
    
    let temp_dir = std::env::temp_dir();
    let installer_path = temp_dir.join("vc_redist.x64.exe");
    
    // Download the installer
    let client = reqwest::blocking::Client::new();
    let mut response = client.get("https://aka.ms/vs/17/release/vc_redist.x64.exe").send()
        .map_err(|e| format!("Failed to download installer: {}", e))?;
    
    let mut file = File::create(&installer_path)
        .map_err(|e| format!("Failed to create installer file: {}", e))?;
    std::io::copy(&mut response, &mut file)
        .map_err(|e| format!("Failed to write installer file: {}", e))?;
    drop(file);
    
    println!("Installing Visual C++ Runtime...");
    
    // Run the installer silently
    let output = Command::new(&installer_path)
        .args(&["/install", "/passive", "/norestart"])
        .output()
        .map_err(|e| format!("Failed to run installer: {}", e))?;
    
    // Clean up installer
    let _ = std::fs::remove_file(&installer_path);
    
    if output.status.success() {
        println!("Successfully installed Visual C++ Runtime.");
        Ok(true)
    } else {
        println!("Installation may have failed. Please install manually from:");
        println!("https://aka.ms/vs/17/release/vc_redist.x64.exe");
        Ok(false)
    }
}

#[cfg(not(windows))]
pub fn is_winget_available() -> bool {
    false
}

#[cfg(not(windows))]
pub fn is_choco_available() -> bool {
    false
}

#[cfg(not(windows))]
pub fn install_vcruntime_winget() -> std::result::Result<bool, Box<dyn std::error::Error>> {
    Err("Not supported on this platform".into())
}

#[cfg(not(windows))]
pub fn install_vcruntime_choco() -> std::result::Result<bool, Box<dyn std::error::Error>> {
    Err("Not supported on this platform".into())
}

#[cfg(not(windows))]
pub fn download_and_install_vcruntime() -> std::result::Result<bool, Box<dyn std::error::Error>> {
    Err("Not supported on this platform".into())
}

