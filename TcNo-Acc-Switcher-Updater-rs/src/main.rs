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

mod archive;
mod gui;
mod logger;
mod patch;
mod updater;
mod utils;
mod windows;

use clap::Parser;
use std::sync::Mutex;
use std::process;
use std::thread;
use std::time::Duration;
use crate::logger::Logger;
use crate::updater::Updater;
use crate::gui::UpdaterApp;

#[derive(Parser, Debug)]
#[command(name = "TcNo-Acc-Switcher-Updater")]
#[command(about = "Updater for the TcNo Account Switcher")]
struct Args {
    #[arg(long)]
    downloadcef: bool,
    
    #[arg(long)]
    verify: bool,
    
    #[arg(long)]
    verifyandclose: bool,
    
    #[arg(long)]
    hashlist: bool,
    
    #[arg(long)]
    createupdate: bool,
}

static SINGLE_INSTANCE: Mutex<()> = Mutex::new(());

#[cfg(windows)]
fn check_vcruntime_available() -> bool {
    use std::ffi::OsStr;
    use std::os::windows::ffi::OsStrExt;
    use winapi::um::libloaderapi::LoadLibraryW;
    
    unsafe {
        // Try to load VCRUNTIME140.dll to check if it's available
        let dll_name: Vec<u16> = OsStr::new("VCRUNTIME140.dll")
            .encode_wide()
            .chain(std::iter::once(0))
            .collect();
        
        let handle = LoadLibraryW(dll_name.as_ptr());
        if !handle.is_null() {
            winapi::um::libloaderapi::FreeLibrary(handle);
            return true;
        }
        
        // Also check if VC++ Runtime is installed via registry
        crate::windows::is_vc_runtime_installed("14.30.30704")
    }
}

#[cfg(not(windows))]
fn check_vcruntime_available() -> bool {
    true // Not applicable on non-Windows
}

fn restart_with_args() {
    use std::env;
    
    let exe_path = match env::current_exe() {
        Ok(path) => path,
        Err(_) => {
            eprintln!("Failed to get executable path. Please restart manually.");
            process::exit(1);
        }
    };
    
    // Get original command-line arguments
    let args: Vec<String> = env::args().skip(1).collect();
    
    eprintln!("\nRestarting application with same arguments...");
    
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        let mut cmd = process::Command::new(&exe_path);
        cmd.args(&args);
        // CREATE_NO_WINDOW flag to hide console window if spawned
        cmd.creation_flags(0x08000000); // CREATE_NO_WINDOW
        if let Err(e) = cmd.spawn() {
            eprintln!("Failed to restart: {}. Please restart manually.", e);
            process::exit(1);
        }
    }
    
    #[cfg(not(windows))]
    {
        if let Err(e) = process::Command::new(&exe_path).args(&args).spawn() {
            eprintln!("Failed to restart: {}. Please restart manually.", e);
            process::exit(1);
        }
    }
    
    process::exit(0);
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Set up panic handler for better error reporting
    std::panic::set_hook(Box::new(|panic_info| {
        eprintln!("PANIC: {:?}", panic_info);
        if let Some(s) = panic_info.payload().downcast_ref::<&str>() {
            eprintln!("Message: {}", s);
        }
        if let Some(s) = panic_info.payload().downcast_ref::<String>() {
            eprintln!("Message: {}", s);
        }
        if let Some(location) = panic_info.location() {
            eprintln!("Location: {}:{}:{}", location.file(), location.line(), location.column());
        }
    }));
    
    // Check for Visual C++ Runtime before proceeding
    #[cfg(windows)]
    {
        if !check_vcruntime_available() {
            eprintln!("\nERROR: Visual C++ Runtime 2015-2022 is required but not found.");
            eprintln!("Attempting to install automatically...\n");
            
            let mut installed = false;
            
            // Try winget first
            if crate::windows::is_winget_available() {
                eprintln!("Detected winget. Installing via winget...");
                match crate::windows::install_vcruntime_winget() {
                    Ok(true) => {
                        installed = true;
                        eprintln!("\nInstallation successful! Restarting application...");
                        // Wait a moment for installation to complete
                        thread::sleep(Duration::from_secs(2));
                        restart_with_args();
                    }
                    Ok(false) => {
                        eprintln!("winget installation failed. Trying alternatives...\n");
                    }
                    Err(e) => {
                        eprintln!("Error using winget: {}. Trying alternatives...\n", e);
                    }
                }
            }
            
            // Try choco if winget failed or isn't available
            if !installed && crate::windows::is_choco_available() {
                eprintln!("Detected Chocolatey. Installing via Chocolatey...");
                match crate::windows::install_vcruntime_choco() {
                    Ok(true) => {
                        installed = true;
                        eprintln!("\nInstallation successful! Restarting application...");
                        // Wait a moment for installation to complete
                        thread::sleep(Duration::from_secs(2));
                        restart_with_args();
                    }
                    Ok(false) => {
                        eprintln!("Chocolatey installation failed. Offering direct download...\n");
                    }
                    Err(e) => {
                        eprintln!("Error using Chocolatey: {}. Offering direct download...\n", e);
                    }
                }
            }
            
            // If both failed or aren't available, offer direct download
            if !installed {
                eprintln!("Neither winget nor Chocolatey is available, or installation failed.");
                eprintln!("Would you like to download and install directly? (Y to install, any other key to exit): ");
                
                let mut input = String::new();
                if let Ok(_) = std::io::stdin().read_line(&mut input) {
                    let trimmed = input.trim().to_lowercase();
                    if trimmed == "y" || trimmed == "yes" {
                        eprintln!("\nDownloading and installing Visual C++ Runtime...");
                        match crate::windows::download_and_install_vcruntime() {
                            Ok(true) => {
                                eprintln!("\nInstallation successful! Restarting application...");
                                // Wait a moment for installation to complete
                                thread::sleep(Duration::from_secs(2));
                                restart_with_args();
                            }
                            Ok(false) | Err(_) => {
                                eprintln!("\nAutomatic installation failed.");
                                eprintln!("Please install manually from:");
                                eprintln!("https://aka.ms/vs/17/release/vc_redist.x64.exe");
                                eprintln!("\nPress Enter to exit...");
                                let mut _buffer = String::new();
                                let _ = std::io::stdin().read_line(&mut _buffer);
                                process::exit(1);
                            }
                        }
                    } else {
                        eprintln!("\nInstallation cancelled. Exiting...");
                        process::exit(1);
                    }
                } else {
                    eprintln!("\nFailed to read input. Exiting...");
                    process::exit(1);
                }
            }
        }
    }
    
    // Initialize logger
    Logger::init()?;
    
    // Single instance check
    let _guard = SINGLE_INSTANCE.lock().unwrap_or_else(|_| {
        eprintln!("Another instance is already running");
        process::exit(1056);
    });
    
    // Parse arguments
    let args = Args::parse();
    
    Logger::write_line(&format!("Updater started with args: {:?}", args));
    
    // Check admin privileges if needed
    let app_data = match utils::get_app_data_folder() {
        Ok(path) => path,
        Err(e) => {
            eprintln!("Failed to get app data folder: {}", e);
            eprintln!("Please ensure the updater is in the correct location.");
            process::exit(1);
        }
    };
    
    if crate::windows::installed_to_program_files() && !crate::windows::is_admin() {
        if let Err(e) = crate::windows::restart_as_admin("") {
            eprintln!("Failed to restart as admin: {}", e);
            process::exit(1);
        }
        return Ok(());
    }
    
    if !crate::windows::has_folder_access(&app_data) {
        if let Err(e) = crate::windows::restart_as_admin("") {
            eprintln!("Failed to restart as admin: {}", e);
            process::exit(1);
        }
        return Ok(());
    }
    
    // Handle command-line modes
    if args.createupdate {
        let updater = Updater::new()?;
        updater.create_update()?;
        Logger::write_line("CreateUpdate complete: Exiting");
        process::exit(0);
    }
    
    if args.hashlist {
        let updater = Updater::new()?;
        updater.generate_hashes()?;
        Logger::write_line("GenerateHashes complete: Exiting");
        process::exit(0);
    }
    
    if args.downloadcef {
        // Run in headless mode for CEF download
        let mut updater = Updater::new()?;
        updater.download_cef_now(
            |s| Logger::write_line(&format!("Status: {}", s)),
            |s| Logger::write_line(s),
            |_, _| {},
        )?;
        Logger::write_line("DownloadCefNow complete: Exiting");
        process::exit(0);
    }
    
    if args.verifyandclose {
        // Run verify and close mode (only works if up to date)
        let mut updater = Updater::new()?;
        updater.verify_and_exit(
            |s| Logger::write_line(&format!("Status: {}", s)),
            |s| Logger::write_line(s),
            |_, _| {},
        )?;
        Logger::write_line("VerifyAndClose complete: Exiting");
        process::exit(0);
    }
    
    if args.verify {
        // Run in headless mode for verification
        let mut updater = Updater::new()?;
        updater.verify_files(
            |s| Logger::write_line(&format!("Status: {}", s)),
            |s| Logger::write_line(s),
            |_, _| {},
        )?;
        Logger::write_line("Verify complete: Exiting");
        process::exit(0);
    }
    
    // Run GUI mode using Slint
    Logger::write_line("Starting GUI...");
    eprintln!("Starting GUI window...");
    
    // Force software rendering to avoid OpenGL requirement
    std::env::set_var("SLINT_BACKEND", "software");
    eprintln!("Set SLINT_BACKEND=software for software rendering");
    
    let gui_app = match UpdaterApp::new() {
        Ok(app) => app,
        Err(e) => {
            eprintln!("Failed to create GUI app: {}", e);
            Logger::write_line(&format!("Failed to create GUI app: {}", e));
            return Err(e);
        }
    };
    
    eprintln!("GUI app created, showing window...");
    gui_app.show();
    
    Logger::write_line("GUI window shown, entering event loop...");
    eprintln!("Entering event loop...");
    
    // Run the Slint event loop (blocks until window is closed)
    gui_app.run();
    
    Logger::write_line("GUI closed normally");
    eprintln!("GUI closed normally");
    
    // Keep console open to see any errors
    eprintln!("Press Enter to exit...");
    use std::io;
    let mut input = String::new();
    let _ = io::stdin().read_line(&mut input);
    
    Ok(())
}

