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

slint::include_modules!();

use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;
use std::env;
use crate::updater::Updater;
use crate::logger::Logger;

pub struct UpdaterApp {
    ui: MainWindow,
    status: Arc<Mutex<String>>,
    log_lines: Arc<Mutex<Vec<String>>>,
    progress: Arc<Mutex<f32>>,
    button_text: Arc<Mutex<String>>,
    button_enabled: Arc<Mutex<bool>>,

    verify_mode: Arc<Mutex<bool>>,
    current_version: Arc<Mutex<String>>,
}

impl UpdaterApp {
    pub fn new() -> Result<Self, Box<dyn std::error::Error>> {
        // Force software rendering to avoid OpenGL requirement
        std::env::set_var("SLINT_BACKEND", "software");
        
        let ui = MainWindow::new().map_err(|e| format!("Failed to create MainWindow: {}", e))?;
        
        // Shared state
        let status = Arc::new(Mutex::new("Initializing...".to_string()));
        let log_lines = Arc::new(Mutex::new(Vec::new()));
        let progress = Arc::new(Mutex::new(0.0));
        let button_text = Arc::new(Mutex::new("...".to_string()));
        let button_enabled = Arc::new(Mutex::new(false));
        let verify_mode = Arc::new(Mutex::new(false));
        let current_version = Arc::new(Mutex::new(String::new()));
        
        // Clone for thread
        let status_clone = status.clone();
        let button_text_clone = button_text.clone();
        let button_enabled_clone = button_enabled.clone();
        
        // Setup callbacks
        ui.on_start_button_clicked({
            let button_text_btn = button_text.clone();
            let verify_mode_btn = verify_mode.clone();
            move || {
                let btn_text = button_text_btn.lock().unwrap().clone();
                if btn_text == "Start update" {
                    if let Ok(mut vm) = verify_mode_btn.lock() {
                        *vm = false;
                    }
                    // TODO: Implement update logic
                } else if btn_text == "Verify files" {
                    if let Ok(mut vm) = verify_mode_btn.lock() {
                        *vm = true;
                    }
                    // TODO: Implement verify logic
                } else if btn_text == "Exit" {
                    std::process::exit(0);
                }
            }
        });
        
        ui.on_minimize_window({
            let ui_weak = ui.as_weak();
            move || {
                if let Some(ui) = ui_weak.upgrade() {
                    eprintln!("Minimize button clicked");
                    ui.window().set_minimized(true);
                }
            }
        });
        
        ui.on_close_window({
            move || {
                eprintln!("Close button clicked - exiting");
                std::process::exit(0);
            }
        });
        
        ui.on_drag_window({
            // Slint with no-frame windows should handle dragging automatically
            // when TouchArea has mouse-cursor: move and pointer events are handled
            // This callback is triggered when drag is initiated from the UI
            move || {
                // The actual dragging is handled by Slint's window system
                // when the TouchArea detects mouse movement with mouse-cursor: move
            }
        });
        
        // Initialize updater in background thread
        thread::spawn(move || {
            if let Err(e) = std::panic::catch_unwind(|| {
                Logger::write_line("Background thread: Starting updater initialization...");
                
                match Updater::new() {
                    Ok(mut updater) => {
                        Logger::write_line("Updater initialized successfully");
                        
                        // Check for updates
                        Logger::write_line("Checking for updates");
                        match updater.get_updates_list() {
                            Ok(updates) => {
                                if updates.is_empty() {
                                    if let Ok(mut status) = status_clone.lock() {
                                        *status = "No updates available".to_string();
                                    }
                                    if let Ok(mut btn_text) = button_text_clone.lock() {
                                        *btn_text = "Exit".to_string();
                                    }
                                    if let Ok(mut btn_enabled) = button_enabled_clone.lock() {
                                        *btn_enabled = true;
                                    }
                                } else {
                                    if let Ok(mut status) = status_clone.lock() {
                                        *status = format!("{} update(s) available", updates.len());
                                    }
                                    if let Ok(mut btn_text) = button_text_clone.lock() {
                                        *btn_text = "Start update".to_string();
                                    }
                                    if let Ok(mut btn_enabled) = button_enabled_clone.lock() {
                                        *btn_enabled = true;
                                    }
                                }
                            }
                            Err(e) => {
                                Logger::write_line(&format!("Error checking for updates: {}", e));
                                if let Ok(mut status) = status_clone.lock() {
                                    *status = format!("Error: {}", e);
                                }
                            }
                        }
                    }
                    Err(e) => {
                        Logger::write_line(&format!("Failed to initialize updater: {}", e));
                        if let Ok(mut status) = status_clone.lock() {
                            *status = format!("Error: {}", e);
                        }
                    }
                }
            }) {
                eprintln!("Background thread panicked: {:?}", e);
                Logger::write_line("Background thread panicked");
            }
        });
        
        // Update UI from state
        let ui_weak = ui.as_weak();
        let status_ui = status.clone();
        let log_lines_ui = log_lines.clone();
        let progress_ui = progress.clone();
        let button_text_ui = button_text.clone();
        let button_enabled_ui = button_enabled.clone();
        thread::spawn(move || {
            loop {
                thread::sleep(Duration::from_millis(100));
                
                if let Some(ui) = ui_weak.upgrade() {
                    let status_val = status_ui.lock().unwrap().clone();
                    let log_lines_val = log_lines_ui.lock().unwrap().clone();
                    let progress_val = *progress_ui.lock().unwrap();
                    let button_text_val = button_text_ui.lock().unwrap().clone();
                    let button_enabled_val = *button_enabled_ui.lock().unwrap();
                    
                    let log_text = log_lines_val.join("\n");
                    
                    ui.set_app_state(AppState {
                        status_text: status_val.into(),
                        log_text: log_text.into(),
                        progress: progress_val,
                        button_text: button_text_val.into(),
                        button_enabled: button_enabled_val,
                    });
                } else {
                    break;
                }
            }
        });
        
        Ok(Self {
            ui,
            status,
            log_lines,
            progress,
            button_text,
            button_enabled,
            verify_mode,
            current_version,
        })
    }
    
    pub fn show(&self) {
        if let Err(e) = self.ui.window().show() {
            eprintln!("Failed to show window: {}", e);
            std::process::exit(1);
        }
    }
    
    pub fn run(&self) {
        eprintln!("Starting Slint event loop...");
        match self.ui.run() {
            Ok(_) => {
                eprintln!("Slint event loop exited normally");
            }
            Err(e) => {
                eprintln!("Failed to run UI event loop: {}", e);
                eprintln!("Error details: {:?}", e);
                std::process::exit(1);
            }
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_status(&self, status: &str) {
        if let Ok(mut s) = self.status.lock() {
            *s = status.to_string();
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn write_line(&self, line: &str) {
        if let Ok(mut lines) = self.log_lines.lock() {
            lines.push(line.to_string());
            // Keep only last 100 lines
            if lines.len() > 100 {
                lines.remove(0);
            }
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_progress(&self, progress: f32) {
        if let Ok(mut p) = self.progress.lock() {
            *p = progress;
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_button_text(&self, text: &str) {
        if let Ok(mut bt) = self.button_text.lock() {
            *bt = text.to_string();
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_button_enabled(&self, enabled: bool) {
        if let Ok(mut be) = self.button_enabled.lock() {
            *be = enabled;
        }
    }
}
