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
use crate::logger::Logger;
use crate::updater::Updater;

#[derive(Parser, Debug)]
#[command(name = "TcNo-Acc-Switcher-Updater")]
#[command(about = "Updater for the TcNo Account Switcher")]
struct Args {
    #[arg(long)]
    downloadcef: bool,
    
    #[arg(long)]
    verify: bool,
    
    #[arg(long)]
    hashlist: bool,
    
    #[arg(long)]
    createupdate: bool,
}

static SINGLE_INSTANCE: Mutex<()> = Mutex::new(());

fn main() -> Result<(), Box<dyn std::error::Error>> {
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
    let app_data = utils::get_app_data_folder()?;
    if crate::windows::installed_to_program_files() && !crate::windows::is_admin() {
        crate::windows::restart_as_admin("")?;
        return Ok(());
    }
    
    if !crate::windows::has_folder_access(&app_data) {
        crate::windows::restart_as_admin("")?;
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
    
    // Run GUI mode
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_title("TcNo Account Switcher Updater")
            .with_inner_size([523.0, 397.0])
            .with_decorations(false)
            .with_transparent(true),
        ..Default::default()
    };
    
    eframe::run_native(
        "TcNo Account Switcher Updater",
        options,
        Box::new(|_cc| Box::new(gui::UpdaterApp::new())),
    )?;
    
    Ok(())
}

